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
	writeMu      sync.Mutex
	state        State
	paused       bool
	conn         *websocket.Conn
	messageQueue [][]byte
	controlQueue [][]byte
	done         chan struct{}
	closeOnce    sync.Once
	tlsCache     tls.ClientSessionCache
}

// NewClient creates a new client.
func NewClient(options ClientOptions) *Client {
	options.applyDefaults()
	return &Client{
		options:  options,
		state:    StateInit,
		tlsCache: tls.NewLRUClientSessionCache(32),
	}
}

func (c *Client) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) Paused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paused
}

func (c *Client) setState(newState State) {
	c.mu.Lock()
	oldState := c.state
	if oldState == newState || oldState.IsTerminal() {
		c.mu.Unlock()
		return
	}
	c.state = newState
	c.mu.Unlock()

	if c.sessionOptions != nil && c.sessionOptions.OnStateChange != nil {
		c.sessionOptions.OnStateChange(oldState, newState)
	} else if c.options.OnStateChange != nil {
		c.options.OnStateChange(oldState, newState)
	}
}

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

func (c *Client) getOnToken() func(Token) {
	if c.sessionOptions != nil && c.sessionOptions.OnToken != nil {
		return c.sessionOptions.OnToken
	}
	return c.options.OnToken
}

func (c *Client) getOnEndpoint() func() {
	if c.sessionOptions != nil && c.sessionOptions.OnEndpoint != nil {
		return c.sessionOptions.OnEndpoint
	}
	return c.options.OnEndpoint
}

func (c *Client) getOnFinalized() func() {
	if c.sessionOptions != nil && c.sessionOptions.OnFinalized != nil {
		return c.sessionOptions.OnFinalized
	}
	return c.options.OnFinalized
}

func (c *Client) getOnDisconnected() func(string) {
	if c.sessionOptions != nil && c.sessionOptions.OnDisconnected != nil {
		return c.sessionOptions.OnDisconnected
	}
	return c.options.OnDisconnected
}

// Start begins a transcription session.
func (c *Client) Start(ctx context.Context, sessionOpts SessionOptions) error {
	c.mu.Lock()
	if c.state.IsActive() {
		c.mu.Unlock()
		return ErrClientAlreadyActive
	}
	c.sessionOptions = &sessionOpts
	c.sessionOptions.applyDefaults()
	c.messageQueue = make([][]byte, 0, c.options.BufferQueueSize)
	c.controlQueue = nil
	c.done = make(chan struct{})
	c.closeOnce = sync.Once{}
	c.paused = false
	c.mu.Unlock()

	c.setState(StateConnecting)

	apiKey, err := c.getAPIKey()
	if err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusAPIKeyFetchFailed, "failed to get API key", err))
		return err
	}

	connCtx, cancel := context.WithTimeout(ctx, c.options.ConnectTimeout)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: c.options.ConnectTimeout,
		NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{}
			conn, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
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
	for _, msg := range c.controlQueue {
		conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			c.mu.Unlock()
			c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to send queued control message", err))
			c.closeConnection()
			return err
		}
	}
	c.controlQueue = nil
	c.mu.Unlock()

	c.setState(StateRunning)

	if cb := c.getOnStarted(); cb != nil {
		cb()
	}

	go c.readLoop()
	go c.keepAliveLoop()

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

func (c *Client) getAPIKey() (string, error) {
	if c.options.APIKeyFunc != nil {
		return c.options.APIKeyFunc()
	}
	return c.options.APIKey, nil
}

// SendAudio sends audio data to the transcription service.
func (c *Client) SendAudio(data []byte) error {
	c.mu.RLock()
	state := c.state
	paused := c.paused
	c.mu.RUnlock()

	if paused {
		return nil
	}

	switch state {
	case StateConnecting:
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

// SendStream reads from r and sends audio chunks until EOF.
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

// Pause pauses audio transmission.
func (c *Client) Pause() {
	c.mu.Lock()
	c.paused = true
	c.mu.Unlock()
}

// Resume resumes audio transmission.
func (c *Client) Resume() {
	c.mu.Lock()
	c.paused = false
	c.mu.Unlock()
}

// Finalize triggers manual finalization of non-final tokens.
func (c *Client) Finalize(opts ...FinalizeOptions) error {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	switch state {
	case StateConnecting:
		data, err := json.Marshal(NewFinalizeMessage(opts...))
		if err != nil {
			return err
		}
		c.mu.Lock()
		if len(c.messageQueue) >= c.options.BufferQueueSize {
			c.mu.Unlock()
			return NewError(ErrorStatusQueueLimitExceeded, "message queue limit exceeded")
		}
		c.controlQueue = append(c.controlQueue, data)
		c.mu.Unlock()
		return nil

	case StateRunning, StateFinishing:
		return c.sendControl(NewFinalizeMessage(opts...))

	default:
		return nil
	}
}

// Stop gracefully stops the transcription session.
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
		c.mu.Lock()
		c.paused = false
		c.mu.Unlock()

		c.setState(StateFinishing)

		return c.writeRaw(websocket.TextMessage, []byte{})
	}

	return nil
}

// Cancel immediately terminates the transcription session.
func (c *Client) Cancel() {
	c.mu.RLock()
	state := c.state
	c.mu.RUnlock()

	if !state.IsInactive() {
		c.closeResources()
		c.setState(StateCanceled)
	}
}

// writeRaw writes a message directly to the WebSocket.
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

// sendControl sends a JSON control message.
func (c *Client) sendControl(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeRaw(websocket.TextMessage, data)
}

// readLoop reads messages from the WebSocket.
func (c *Client) readLoop() {
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

			if state.IsTerminal() {
				return
			}

			if state == StateFinishing {
				connErr := NewErrorWithCause(ErrorStatusConnectionClosed, "WebSocket closed before finished response", err)
				c.handleError(connErr)
				return
			}

			if state == StateRunning || state == StateConnecting {
				if cb := c.getOnDisconnected(); cb != nil {
					cb("")
				}
				c.closeResources()
				c.setState(StateClosed)
				return
			}

			return
		}

		var response Response
		if err := json.Unmarshal(message, &response); err != nil {
			c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to parse response", err))
			return
		}

		if response.ErrorCode != nil || response.ErrorMessage != "" {
			code := 0
			if response.ErrorCode != nil {
				code = *response.ErrorCode
			}
			c.handleError(MapAPIError(response.ErrorMessage, code))
			return
		}

		hasEndpoint := hasSpecialToken(response.Tokens, "<end>")
		hasFinalized := hasSpecialToken(response.Tokens, "<fin>")
		response.Tokens = filterSpecialTokens(response.Tokens)

		if cb := c.getOnToken(); cb != nil {
			for _, tok := range response.Tokens {
				cb(tok)
			}
		}

		if cb := c.getOnResult(); cb != nil {
			cb(&response)
		}

		if hasEndpoint {
			if cb := c.getOnEndpoint(); cb != nil {
				cb()
			}
		}

		if hasFinalized {
			if cb := c.getOnFinalized(); cb != nil {
				cb()
			}
		}

		if response.Finished {
			c.handleFinished()
			return
		}
	}
}

// keepAliveLoop sends keep-alive messages at regular intervals.
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

func (c *Client) handleError(err *Error) {
	c.closeResources()
	c.setState(StateError)

	if cb := c.getOnError(); cb != nil {
		cb(err)
	}
}

func (c *Client) handleFinished() {
	c.closeResources()
	c.setState(StateFinished)

	if cb := c.getOnFinished(); cb != nil {
		cb()
	}
}

func (c *Client) closeConnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) closeResources() {
	c.closeOnce.Do(func() {
		if c.done != nil {
			close(c.done)
		}
		c.closeConnection()
		c.mu.Lock()
		c.messageQueue = nil
		c.controlQueue = nil
		c.mu.Unlock()
	})
}

// Close releases all resources.
func (c *Client) Close() error {
	c.Cancel()
	return nil
}

func hasSpecialToken(tokens []Token, text string) bool {
	for i := range tokens {
		if tokens[i].Text == text {
			return true
		}
	}
	return false
}

// filterSpecialTokens removes <end> and <fin> control tokens in-place.
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
