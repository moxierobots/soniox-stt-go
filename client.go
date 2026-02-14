package soniox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client is a Soniox Speech-to-Text WebSocket client.
type Client struct {
	options        ClientOptions
	sessionOptions *SessionOptions

	mu           sync.RWMutex
	writeMu      sync.Mutex // serializes all WebSocket writes — replaces channel for zero-hop direct writes
	state        State
	paused       bool
	conn         *websocket.Conn
	messageQueue [][]byte
	done         chan struct{}
	closeOnce    sync.Once
	tlsCache     tls.ClientSessionCache // reused across sessions for faster TLS resumption
}

// NewClient creates a new Soniox client with the given options.
func NewClient(options ClientOptions) *Client {
	options.applyDefaults()
	return &Client{
		options:  options,
		state:    StateInit,
		tlsCache: tls.NewLRUClientSessionCache(32),
	}
}

// State returns the current state of the client.
func (c *Client) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Paused returns whether the client is currently paused.
func (c *Client) Paused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paused
}

// setState updates the client state and triggers the callback.
// Transitions from terminal states (Finished, Error, Canceled) are silently ignored
// to prevent race conditions between concurrent goroutines.
func (c *Client) setState(newState State) {
	c.mu.Lock()
	oldState := c.state
	if oldState == newState || oldState.IsTerminal() {
		c.mu.Unlock()
		return
	}
	c.state = newState
	c.mu.Unlock()

	// Call session-level callback first, then client-level
	if c.sessionOptions != nil && c.sessionOptions.OnStateChange != nil {
		c.sessionOptions.OnStateChange(oldState, newState)
	} else if c.options.OnStateChange != nil {
		c.options.OnStateChange(oldState, newState)
	}
}

// getCallback returns the appropriate callback (session-level takes precedence).
func (c *Client) getOnStarted() func() {
	if c.sessionOptions != nil && c.sessionOptions.OnStarted != nil {
		return c.sessionOptions.OnStarted
	}
	return c.options.OnStarted
}

func (c *Client) getOnResult() func(*Response) {
	if c.sessionOptions != nil && c.sessionOptions.OnResult != nil {
		return c.sessionOptions.OnResult
	}
	return c.options.OnResult
}

func (c *Client) getOnFinished() func() {
	if c.sessionOptions != nil && c.sessionOptions.OnFinished != nil {
		return c.sessionOptions.OnFinished
	}
	return c.options.OnFinished
}

func (c *Client) getOnError() func(*Error) {
	if c.sessionOptions != nil && c.sessionOptions.OnError != nil {
		return c.sessionOptions.OnError
	}
	return c.options.OnError
}

// Start begins a transcription session with the given options.
// The provided context controls the session lifetime: cancelling it will
// close the WebSocket connection and set the state to Canceled.
func (c *Client) Start(ctx context.Context, sessionOpts SessionOptions) error {
	c.mu.Lock()
	if c.state.IsActive() {
		c.mu.Unlock()
		return ErrClientAlreadyActive
	}
	c.sessionOptions = &sessionOpts
	c.sessionOptions.applyDefaults()
	c.messageQueue = make([][]byte, 0, c.options.BufferQueueSize)
	c.done = make(chan struct{})
	c.closeOnce = sync.Once{}
	c.paused = false
	c.mu.Unlock()

	c.setState(StateConnecting)

	// Get API key
	apiKey, err := c.getAPIKey()
	if err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusAPIKeyFetchFailed, "failed to get API key", err))
		return err
	}

	// Create connection context with timeout
	connCtx, cancel := context.WithTimeout(ctx, c.options.ConnectTimeout)
	defer cancel()

	// Connect to WebSocket with TCP_NODELAY and TLS session cache
	dialer := websocket.Dialer{
		HandshakeTimeout: c.options.ConnectTimeout,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{}
			conn, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			// TCP_NODELAY disables Nagle's algorithm — each write goes to the
			// wire immediately instead of being buffered up to 40ms.
			if tc, ok := conn.(*net.TCPConn); ok {
				tc.SetNoDelay(true)
			}
			return conn, nil
		},
		TLSClientConfig: &tls.Config{
			ClientSessionCache: c.tlsCache,
		},
	}

	conn, _, err := dialer.DialContext(connCtx, c.options.WebSocketURL, http.Header{})
	if err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to connect", err))
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// Send initial configuration — direct write, no goroutines running yet
	request := c.sessionOptions.toRequest(apiKey)
	configData, err := json.Marshal(request)
	if err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to marshal configuration", err))
		c.closeConnection()
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, configData); err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to send configuration", err))
		c.closeConnection()
		return err
	}

	// Send any queued messages — direct write, still single-threaded
	c.mu.Lock()
	for _, msg := range c.messageQueue {
		conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
		if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			c.mu.Unlock()
			c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to send queued message", err))
			c.closeConnection()
			return err
		}
	}
	c.messageQueue = nil
	c.mu.Unlock()

	c.setState(StateRunning)

	// Call onStarted callback
	if cb := c.getOnStarted(); cb != nil {
		cb()
	}

	// Start background goroutines — only readLoop and keepAliveLoop.
	// No writeLoop needed: all writes go through writeMu directly.
	go c.readLoop()
	go c.keepAliveLoop()

	// Monitor context for cancellation
	go func() {
		select {
		case <-ctx.Done():
			c.closeResources()
			c.setState(StateCanceled)
		case <-c.done:
		}
	}()

	return nil
}

// getAPIKey retrieves the API key from the configured source.
func (c *Client) getAPIKey() (string, error) {
	if c.options.APIKeyFunc != nil {
		return c.options.APIKeyFunc()
	}
	return c.options.APIKey, nil
}

// SendAudio sends audio data to the transcription service.
// If the client is paused, audio is silently dropped.
// Writes go directly to the WebSocket (no channel hop) for minimum latency.
func (c *Client) SendAudio(data []byte) error {
	c.mu.RLock()
	state := c.state
	paused := c.paused
	c.mu.RUnlock()

	if paused {
		return nil // drop silently when paused
	}

	switch state {
	case StateConnecting:
		// Queue the message for sending after connection is established
		c.mu.Lock()
		if len(c.messageQueue) >= c.options.BufferQueueSize {
			c.mu.Unlock()
			return NewError(ErrorStatusQueueLimitExceeded, "message queue limit exceeded")
		}
		c.messageQueue = append(c.messageQueue, data)
		c.mu.Unlock()
		return nil

	case StateRunning:
		return c.writeRaw(websocket.BinaryMessage, data)

	default:
		return NewError(ErrorStatusInvalidState, "cannot send audio in state: "+string(state))
	}
}

// SendStream reads audio data from r and sends it to the transcription service.
// It blocks until r returns io.EOF, an error occurs, or the session ends.
// If opts.Finish is true, Stop() is called after the entire stream is consumed.
func (c *Client) SendStream(r io.Reader, opts ...SendStreamOptions) error {
	var opt SendStreamOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.ChunkSize <= 0 {
		opt.ChunkSize = DefaultStreamChunkSize
	}

	buf := make([]byte, opt.ChunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			// Copy the data since buf is reused
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if sendErr := c.SendAudio(chunk); sendErr != nil {
				return sendErr
			}
			if opt.PaceInterval > 0 {
				time.Sleep(opt.PaceInterval)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if opt.Finish {
		return c.Stop()
	}
	return nil
}

// Pause pauses audio transmission. Audio sent via SendAudio will be silently dropped.
// Keep-alive messages continue to be sent to maintain the connection.
func (c *Client) Pause() {
	c.mu.Lock()
	c.paused = true
	c.mu.Unlock()
}

// Resume resumes audio transmission after a Pause.
func (c *Client) Resume() {
	c.mu.Lock()
	c.paused = false
	c.mu.Unlock()
}

// Finalize triggers manual finalization of non-final tokens.
// Optionally pass FinalizeOptions to specify trailing silence milliseconds.
// If the client is not in Running or Finishing state, the call is silently ignored.
func (c *Client) Finalize(opts ...FinalizeOptions) error {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if state != StateRunning && state != StateFinishing {
		return nil
	}

	return c.sendControl(NewFinalizeMessage(opts...))
}

// Stop gracefully stops the transcription session, waiting for final results.
func (c *Client) Stop() error {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if state == StateConnecting {
		c.closeResources()
		c.handleFinished()
		return nil
	}

	if state == StateRunning {
		// Unpause if paused
		c.mu.Lock()
		c.paused = false
		c.mu.Unlock()

		c.setState(StateFinishing)

		// Send empty text message directly to signal end of audio stream
		return c.writeRaw(websocket.TextMessage, []byte{})
	}

	return nil
}

// Cancel immediately terminates the transcription session without waiting for results.
func (c *Client) Cancel() {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if !state.IsInactive() {
		c.closeResources()
		c.setState(StateCanceled)
	}
}

// writeRaw writes a message directly to the WebSocket, serialized by writeMu.
// This is the hot path — no channels, no goroutine hops, just mutex + syscall.
func (c *Client) writeRaw(msgType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return ErrClientNotConnected
	}

	conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
	if err := conn.WriteMessage(msgType, data); err != nil {
		return NewErrorWithCause(ErrorStatusWebSocketError, "write error", err)
	}
	return nil
}

// sendControl sends a JSON control message directly to the WebSocket.
func (c *Client) sendControl(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeRaw(websocket.TextMessage, data)
}

// readLoop reads messages from the WebSocket.
func (c *Client) readLoop() {
	defer c.closeResources()

	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			c.mu.RLock()
			state := c.state
			c.mu.RUnlock()

			if state == StateRunning || state == StateConnecting {
				c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "read error", err))
			}
			return
		}

		var response Response
		if err := json.Unmarshal(message, &response); err != nil {
			c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to parse response", err))
			return
		}

		// Check for API error
		if response.ErrorCode != nil || response.ErrorMessage != "" {
			code := 0
			if response.ErrorCode != nil {
				code = *response.ErrorCode
			}
			c.handleError(NewErrorWithCode(ErrorStatusAPIError, response.ErrorMessage, code))
			return
		}

		// Filter special control tokens before delivering to user
		response.Tokens = filterSpecialTokens(response.Tokens)

		// Call result callback
		if cb := c.getOnResult(); cb != nil {
			cb(&response)
		}

		// Check if finished
		if response.Finished {
			c.handleFinished()
			return
		}
	}
}

// keepAliveLoop sends keep-alive messages at regular intervals.
// Messages are sent when KeepAlive is enabled or when the client is paused.
func (c *Client) keepAliveLoop() {
	ticker := time.NewTicker(c.options.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.RLock()
			state := c.state
			paused := c.paused
			keepAlive := c.options.KeepAlive
			c.mu.RUnlock()

			shouldSend := (state == StateRunning || state == StateFinishing) && (paused || keepAlive)
			if !shouldSend {
				continue
			}

			// Best-effort: ignore errors on keepalive
			c.sendControl(NewKeepAliveMessage())
		}
	}
}

// handleError handles an error by closing resources, setting state, and calling the callback.
func (c *Client) handleError(err *Error) {
	c.closeResources()
	c.setState(StateError)

	if cb := c.getOnError(); cb != nil {
		cb(err)
	}
}

// handleFinished handles session completion.
func (c *Client) handleFinished() {
	c.closeResources()
	c.setState(StateFinished)

	if cb := c.getOnFinished(); cb != nil {
		cb()
	}
}

// closeConnection closes the WebSocket connection.
func (c *Client) closeConnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// closeResources closes all resources associated with the session.
// It is safe to call from multiple goroutines; only the first call takes effect.
func (c *Client) closeResources() {
	c.closeOnce.Do(func() {
		// Signal all goroutines (keepAliveLoop, context monitor) to stop
		if c.done != nil {
			close(c.done)
		}

		// Close the WebSocket connection (causes readLoop to get an error and return)
		c.closeConnection()

		// Clear message queue
		c.mu.Lock()
		c.messageQueue = nil
		c.mu.Unlock()
	})
}

// Close closes the client and releases all resources.
func (c *Client) Close() error {
	c.Cancel()
	return nil
}

// filterSpecialTokens removes control tokens (<end>, <fin>) from the token list.
// These tokens are internal markers from the API and should not be exposed to users.
func filterSpecialTokens(tokens []Token) []Token {
	n := 0
	for _, t := range tokens {
		if t.Text != "<end>" && t.Text != "<fin>" {
			tokens[n] = t
			n++
		}
	}
	return tokens[:n]
}
