package soniox

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client is a Soniox Speech-to-Text WebSocket client.
type Client struct {
	options        ClientOptions
	sessionOptions *SessionOptions

	mu            sync.RWMutex
	state         State
	conn          *websocket.Conn
	messageQueue  [][]byte
	done          chan struct{}
	keepAliveDone chan struct{}
	writeQueue    chan []byte
	closeOnce     sync.Once
}

// NewClient creates a new Soniox client with the given options.
func NewClient(options ClientOptions) *Client {
	options.applyDefaults()
	return &Client{
		options: options,
		state:   StateInit,
	}
}

// State returns the current state of the client.
func (c *Client) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// setState updates the client state and triggers the callback.
func (c *Client) setState(newState State) {
	c.mu.Lock()
	oldState := c.state
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
func (c *Client) Start(ctx context.Context, sessionOpts SessionOptions) error {
	c.mu.Lock()
	if c.state.IsActive() {
		c.mu.Unlock()
		return ErrClientAlreadyActive
	}
	c.state = StateConnecting
	c.sessionOptions = &sessionOpts
	c.sessionOptions.applyDefaults()
	c.messageQueue = make([][]byte, 0, c.options.BufferQueueSize)
	c.done = make(chan struct{})
	c.writeQueue = make(chan []byte, c.options.BufferQueueSize)
	c.closeOnce = sync.Once{}
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

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: c.options.ConnectTimeout,
	}

	conn, _, err := dialer.DialContext(connCtx, c.options.WebSocketURL, http.Header{})
	if err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to connect", err))
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// Send initial configuration
	request := c.sessionOptions.toRequest(apiKey)
	if err := c.sendJSON(request); err != nil {
		c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "failed to send configuration", err))
		c.closeConnection()
		return err
	}

	// Send any queued messages
	c.mu.Lock()
	for _, msg := range c.messageQueue {
		if err := c.conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
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

	// Start keep-alive goroutine if enabled
	if c.options.KeepAlive {
		c.keepAliveDone = make(chan struct{})
		go c.keepAliveLoop()
	}

	// Start read and write goroutines
	go c.readLoop()
	go c.writeLoop()

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
// The audio should be in the format specified in SessionOptions.
func (c *Client) SendAudio(data []byte) error {
	c.mu.RLock()
	state := c.state
	conn := c.conn
	c.mu.RUnlock()

	switch state {
	case StateConnecting:
		// Queue the message
		c.mu.Lock()
		if len(c.messageQueue) >= c.options.BufferQueueSize {
			c.mu.Unlock()
			return NewError(ErrorStatusQueueLimitExceeded, "message queue limit exceeded")
		}
		c.messageQueue = append(c.messageQueue, data)
		c.mu.Unlock()
		return nil

	case StateRunning:
		if conn == nil {
			return ErrClientNotConnected
		}
		select {
		case c.writeQueue <- data:
			return nil
		default:
			return NewError(ErrorStatusQueueLimitExceeded, "write queue full")
		}

	default:
		return NewError(ErrorStatusInvalidState, "cannot send audio in state: "+string(state))
	}
}

// Finalize triggers manual finalization of non-final tokens.
func (c *Client) Finalize() error {
	return c.sendJSON(NewFinalizeMessage())
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
		c.setState(StateFinishing)

		// Send empty message to signal end of audio
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn != nil {
			// Send empty text message to signal end of audio stream
			if err := conn.WriteMessage(websocket.TextMessage, []byte{}); err != nil {
				return NewErrorWithCause(ErrorStatusWebSocketError, "failed to send stop signal", err)
			}
		}
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

// sendJSON sends a JSON message to the WebSocket.
func (c *Client) sendJSON(v interface{}) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return ErrClientNotConnected
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
		return c.conn.WriteMessage(websocket.TextMessage, data)
	}
	return ErrClientNotConnected
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

// writeLoop writes queued messages to the WebSocket.
func (c *Client) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case data, ok := <-c.writeQueue:
			if !ok {
				return
			}

			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			c.mu.Lock()
			conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
			err := conn.WriteMessage(websocket.BinaryMessage, data)
			c.mu.Unlock()

			if err != nil {
				c.handleError(NewErrorWithCause(ErrorStatusWebSocketError, "write error", err))
				return
			}
		}
	}
}

// keepAliveLoop sends keep-alive messages at regular intervals.
func (c *Client) keepAliveLoop() {
	ticker := time.NewTicker(c.options.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.keepAliveDone:
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.RLock()
			state := c.state
			c.mu.RUnlock()

			if state == StateRunning || state == StateFinishing {
				if err := c.sendJSON(NewKeepAliveMessage()); err != nil {
					// Log error but don't fail the session
					continue
				}
			}
		}
	}
}

// handleError handles an error by setting state and calling callback.
func (c *Client) handleError(err *Error) {
	c.setState(StateError)
	c.closeResources()

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
func (c *Client) closeResources() {
	c.closeOnce.Do(func() {
		// Signal goroutines to stop
		if c.done != nil {
			close(c.done)
		}

		// Stop keep-alive
		if c.keepAliveDone != nil {
			close(c.keepAliveDone)
		}

		// Close write queue
		c.mu.Lock()
		if c.writeQueue != nil {
			// Drain the queue first
			for len(c.writeQueue) > 0 {
				<-c.writeQueue
			}
		}
		c.mu.Unlock()

		// Close connection
		c.closeConnection()

		// Clear session options
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
