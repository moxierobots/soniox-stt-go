package soniox

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- Unit tests for filterSpecialTokens ---

func TestFilterSpecialTokens(t *testing.T) {
	tests := []struct {
		name   string
		input  []Token
		expect []string
	}{
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
		{
			name:   "no special tokens",
			input:  []Token{{Text: "hello "}, {Text: "world"}},
			expect: []string{"hello ", "world"},
		},
		{
			name:   "filter <end>",
			input:  []Token{{Text: "hello "}, {Text: "<end>"}, {Text: "world"}},
			expect: []string{"hello ", "world"},
		},
		{
			name:   "filter <fin>",
			input:  []Token{{Text: "hello "}, {Text: "<fin>"}},
			expect: []string{"hello "},
		},
		{
			name:   "filter both",
			input:  []Token{{Text: "<end>"}, {Text: "test"}, {Text: "<fin>"}, {Text: "ok"}},
			expect: []string{"test", "ok"},
		},
		{
			name:   "all special",
			input:  []Token{{Text: "<end>"}, {Text: "<fin>"}},
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterSpecialTokens(tt.input)
			if len(result) == 0 && len(tt.expect) == 0 {
				return // both empty, OK
			}
			if len(result) != len(tt.expect) {
				t.Fatalf("expected %d tokens, got %d", len(tt.expect), len(result))
			}
			for i, tok := range result {
				if tok.Text != tt.expect[i] {
					t.Errorf("token[%d]: expected %q, got %q", i, tt.expect[i], tok.Text)
				}
			}
		})
	}
}

// --- Unit tests for State helpers ---

func TestStateIsActive(t *testing.T) {
	active := []State{StateConnecting, StateRunning, StateFinishing}
	inactive := []State{StateInit, StateFinished, StateError, StateCanceled, StateClosed}

	for _, s := range active {
		if !s.IsActive() {
			t.Errorf("expected %s to be active", s)
		}
	}
	for _, s := range inactive {
		if s.IsActive() {
			t.Errorf("expected %s to be inactive", s)
		}
	}
}

func TestStateIsTerminal(t *testing.T) {
	terminal := []State{StateFinished, StateError, StateCanceled, StateClosed}
	nonTerminal := []State{StateInit, StateConnecting, StateRunning, StateFinishing}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

// --- Unit tests for Error types ---

func TestNewError(t *testing.T) {
	err := NewError(ErrorStatusAPIError, "test error")
	if err.Status != ErrorStatusAPIError {
		t.Errorf("expected status %s, got %s", ErrorStatusAPIError, err.Status)
	}
	if err.Message != "test error" {
		t.Errorf("expected message %q, got %q", "test error", err.Message)
	}
	if err.Code != nil {
		t.Error("expected nil code")
	}
	if err.Cause != nil {
		t.Error("expected nil cause")
	}
}

func TestNewErrorWithCode(t *testing.T) {
	err := NewErrorWithCode(ErrorStatusAPIError, "api err", 42)
	if err.Code == nil || *err.Code != 42 {
		t.Errorf("expected code 42, got %v", err.Code)
	}
	if !strings.Contains(err.Error(), "code=42") {
		t.Errorf("Error() should contain code: %s", err.Error())
	}
}

func TestNewErrorWithCause(t *testing.T) {
	cause := NewError(ErrorStatusWebSocketError, "ws fail")
	err := NewErrorWithCause(ErrorStatusConnectionClosed, "closed", cause)
	if err.Cause != cause {
		t.Error("expected cause to be set")
	}
	if err.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}
}

func TestIsErrorStatus(t *testing.T) {
	err := NewError(ErrorStatusAPIError, "test")
	if !IsErrorStatus(err, ErrorStatusAPIError) {
		t.Error("expected IsErrorStatus to return true")
	}
	if IsErrorStatus(err, ErrorStatusWebSocketError) {
		t.Error("expected IsErrorStatus to return false for different status")
	}
}

// --- Unit tests for Options ---

func TestClientOptionsDefaults(t *testing.T) {
	opts := ClientOptions{}
	opts.applyDefaults()

	if opts.WebSocketURL != DefaultWebSocketURL {
		t.Errorf("expected default WebSocketURL, got %q", opts.WebSocketURL)
	}
	if opts.BufferQueueSize != DefaultBufferQueueSize {
		t.Errorf("expected default BufferQueueSize %d, got %d", DefaultBufferQueueSize, opts.BufferQueueSize)
	}
	if opts.ConnectTimeout != DefaultConnectTimeout {
		t.Errorf("expected default ConnectTimeout, got %v", opts.ConnectTimeout)
	}
	if opts.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("expected default WriteTimeout, got %v", opts.WriteTimeout)
	}
	if opts.KeepAliveInterval != DefaultKeepAliveInterval {
		t.Errorf("expected default KeepAliveInterval, got %v", opts.KeepAliveInterval)
	}
}

func TestSessionOptionsDefaults(t *testing.T) {
	opts := SessionOptions{}
	opts.applyDefaults()

	if opts.Model != DefaultModel {
		t.Errorf("expected default model %q, got %q", DefaultModel, opts.Model)
	}
	if opts.AudioFormat != DefaultAudioFormat {
		t.Errorf("expected default audio format %q, got %q", DefaultAudioFormat, opts.AudioFormat)
	}
}

func TestSessionOptionsToRequest(t *testing.T) {
	opts := SessionOptions{
		Model:               "stt-rt-v4",
		AudioFormat:         "pcm_s16le",
		SampleRate:          16000,
		NumChannels:         1,
		LanguageHints:       []string{"en", "es"},
		LanguageHintsStrict: true,
		ClientReferenceID:   "ref-123",
	}
	req := opts.toRequest("test-key")

	if req.APIKey != "test-key" {
		t.Errorf("expected api key %q, got %q", "test-key", req.APIKey)
	}
	if req.LanguageHintsStrict != true {
		t.Error("expected LanguageHintsStrict to be true")
	}
	if req.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", req.SampleRate)
	}

	// Verify JSON serialization includes language_hints_strict
	data, _ := json.Marshal(req)
	if !strings.Contains(string(data), `"language_hints_strict":true`) {
		t.Errorf("JSON should contain language_hints_strict: %s", string(data))
	}
}

// --- Unit tests for Client ---

func TestNewClient(t *testing.T) {
	client := NewClient(ClientOptions{APIKey: "test"})
	if client.State() != StateInit {
		t.Errorf("expected initial state Init, got %s", client.State())
	}
	if client.Paused() {
		t.Error("expected not paused initially")
	}
}

func TestSetStateIgnoresTerminal(t *testing.T) {
	client := NewClient(ClientOptions{})

	// Manually set to a terminal state
	client.mu.Lock()
	client.state = StateError
	client.mu.Unlock()

	// Try to transition from terminal - should be ignored
	client.setState(StateRunning)
	if client.State() != StateError {
		t.Errorf("expected state to remain Error, got %s", client.State())
	}
}

func TestSetStateIgnoresSameState(t *testing.T) {
	callCount := 0
	client := NewClient(ClientOptions{
		OnStateChange: func(old, new State) {
			callCount++
		},
	})

	client.setState(StateConnecting)
	if callCount != 1 {
		t.Errorf("expected 1 callback, got %d", callCount)
	}

	// Same state transition - should be ignored
	client.setState(StateConnecting)
	if callCount != 1 {
		t.Errorf("expected still 1 callback, got %d", callCount)
	}
}

func TestSendAudioWhenIdle(t *testing.T) {
	client := NewClient(ClientOptions{})
	err := client.SendAudio([]byte("audio"))
	if err == nil {
		t.Fatal("expected error when sending audio in Init state")
	}
	if !IsErrorStatus(err, ErrorStatusInvalidState) {
		t.Errorf("expected InvalidState error, got %v", err)
	}
}

func TestPauseResume(t *testing.T) {
	client := NewClient(ClientOptions{})
	if client.Paused() {
		t.Error("should not be paused initially")
	}
	client.Pause()
	if !client.Paused() {
		t.Error("should be paused after Pause()")
	}
	client.Resume()
	if client.Paused() {
		t.Error("should not be paused after Resume()")
	}
}

// --- Mock WebSocket server for integration tests ---

// mockWSHandler creates a basic mock WebSocket server that:
// 1. Reads the config message
// 2. For each binary message, sends a result with a token
// 3. When it receives an empty text message, sends a finished response
func mockWSHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read config message
		_, _, err = conn.ReadMessage()
		if err != nil {
			t.Logf("read config error: %v", err)
			return
		}

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Empty text message = stop signal
			if msgType == websocket.TextMessage && len(msg) == 0 {
				resp := Response{
					Finished: true,
					Tokens:   []Token{},
				}
				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// Text message = control (finalize, keepalive)
			if msgType == websocket.TextMessage {
				continue
			}

			// Binary message = audio, send back a result
			resp := Response{
				Tokens: []Token{
					{Text: "hello ", IsFinal: true},
					{Text: "<end>"},
				},
				FinalAudioProcMs: 100,
				TotalAudioProcMs: 100,
			}
			data, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func startMockServer(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(mockWSHandler(t))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	t.Cleanup(server.Close)
	return wsURL
}

// --- Integration tests with mock server ---

func TestConnectAndStop(t *testing.T) {
	wsURL := startMockServer(t)

	var states []State
	var mu sync.Mutex

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
		OnStateChange: func(old, new State) {
			mu.Lock()
			states = append(states, new)
			mu.Unlock()
		},
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if client.State() != StateRunning {
		t.Errorf("expected Running state, got %s", client.State())
	}

	err = client.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Wait for finished
	deadline := time.After(5 * time.Second)
	for {
		if client.State() == StateFinished {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for Finished state, current: %s", client.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	// States should be: Connecting, Running, Finishing, Finished
	expected := []State{StateConnecting, StateRunning, StateFinishing, StateFinished}
	if len(states) != len(expected) {
		t.Errorf("expected %d state transitions, got %d: %v", len(expected), len(states), states)
	} else {
		for i, s := range expected {
			if states[i] != s {
				t.Errorf("state[%d]: expected %s, got %s", i, s, states[i])
			}
		}
	}
}

func TestSendAudioAndReceive(t *testing.T) {
	wsURL := startMockServer(t)

	var receivedTokens []Token
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{
		OnResult: func(resp *Response) {
			mu.Lock()
			receivedTokens = append(receivedTokens, resp.Tokens...)
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send some audio
	err = client.SendAudio([]byte("fake audio data"))
	if err != nil {
		t.Fatalf("SendAudio failed: %v", err)
	}

	// Give the server a moment to respond
	time.Sleep(100 * time.Millisecond)

	err = client.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
	}

	mu.Lock()
	defer mu.Unlock()

	// Should have received "hello " but NOT "<end>" (filtered)
	foundHello := false
	for _, tok := range receivedTokens {
		if tok.Text == "<end>" {
			t.Error("special token <end> should have been filtered")
		}
		if tok.Text == "hello " {
			foundHello = true
		}
	}
	if !foundHello {
		t.Error("expected to receive 'hello ' token")
	}
}

func TestSendStream(t *testing.T) {
	wsURL := startMockServer(t)

	done := make(chan struct{})
	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send via SendStream with Finish=true
	audioData := bytes.NewReader([]byte("fake audio data that is long enough"))
	err = client.SendStream(audioData, SendStreamOptions{
		ChunkSize: 8,
		Finish:    true,
	})
	if err != nil {
		t.Fatalf("SendStream failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
	}

	if client.State() != StateFinished {
		t.Errorf("expected Finished state, got %s", client.State())
	}
}

func TestContextCancellation(t *testing.T) {
	wsURL := startMockServer(t)

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	err := client.Start(ctx, SessionOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel the context
	cancel()

	// Wait for state to become Canceled
	deadline := time.After(5 * time.Second)
	for {
		state := client.State()
		if state == StateCanceled {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for Canceled state, current: %s", client.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestPausedAudioDropped(t *testing.T) {
	wsURL := startMockServer(t)

	var resultCount int
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{
		OnResult: func(resp *Response) {
			mu.Lock()
			resultCount++
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Pause and send audio - should be dropped
	client.Pause()
	err = client.SendAudio([]byte("should be dropped"))
	if err != nil {
		t.Fatalf("SendAudio while paused should not error: %v", err)
	}

	// Wait a bit to ensure nothing comes back
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := resultCount
	mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 results while paused, got %d", count)
	}

	// Resume and stop
	client.Resume()
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
	}
}

func TestFinalizeMessage(t *testing.T) {
	wsURL := startMockServer(t)

	done := make(chan struct{})
	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Finalize should not error
	err = client.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Stop
	err = client.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
	}
}

func TestFinalizeWithTrailingSilence(t *testing.T) {
	// Verify JSON serialization includes trailing_silence_ms
	msg := NewFinalizeMessage(FinalizeOptions{TrailingSilenceMs: 500})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"trailing_silence_ms":500`) {
		t.Errorf("expected trailing_silence_ms in JSON, got: %s", s)
	}

	// Without options, trailing_silence_ms should be omitted
	msg2 := NewFinalizeMessage()
	data2, _ := json.Marshal(msg2)
	if strings.Contains(string(data2), "trailing_silence_ms") {
		t.Errorf("trailing_silence_ms should be omitted when not set, got: %s", string(data2))
	}
}

func TestFinalizeIgnoredWhenNotActive(t *testing.T) {
	client := NewClient(ClientOptions{})

	// Client is in Init state — Finalize should be silently ignored
	err := client.Finalize()
	if err != nil {
		t.Fatalf("Finalize in Init state should not error, got: %v", err)
	}

	err = client.Finalize(FinalizeOptions{TrailingSilenceMs: 300})
	if err != nil {
		t.Fatalf("Finalize with opts in Init state should not error, got: %v", err)
	}
}

func TestFinalizeWithTrailingSilenceIntegration(t *testing.T) {
	wsURL := startMockServer(t)

	done := make(chan struct{})
	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Finalize with trailing silence option
	err = client.Finalize(FinalizeOptions{TrailingSilenceMs: 200})
	if err != nil {
		t.Fatalf("Finalize with trailing silence failed: %v", err)
	}

	err = client.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFinished")
	}
}

func TestErrorCallback(t *testing.T) {
	// Server that returns an API error
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read config
		conn.ReadMessage()

		// Send error response
		code := 401
		resp := Response{
			ErrorCode:    &code,
			ErrorMessage: "unauthorized",
		}
		data, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, data)
	}))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var receivedErr *Error
	errDone := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "bad-key",
		WebSocketURL: wsURL,
		OnError: func(err *Error) {
			receivedErr = err
			close(errDone)
		},
	})
	defer client.Close()

	ctx := context.Background()
	err := client.Start(ctx, SessionOptions{})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case <-errDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnError")
	}

	if receivedErr == nil {
		t.Fatal("expected error callback to be called")
	}
	if receivedErr.Status != ErrorStatusAuthError {
		t.Errorf("expected auth_error status for 401, got %s", receivedErr.Status)
	}
	if receivedErr.Code == nil || *receivedErr.Code != 401 {
		t.Errorf("expected error code 401, got %v", receivedErr.Code)
	}
	if client.State() != StateError {
		t.Errorf("expected Error state, got %s", client.State())
	}
}

// --- Tests for MapAPIError typed error mapping ---

func TestMapAPIError(t *testing.T) {
	tests := []struct {
		code     int
		expected ErrorStatus
	}{
		{401, ErrorStatusAuthError},
		{400, ErrorStatusBadRequest},
		{402, ErrorStatusQuotaExceeded},
		{429, ErrorStatusQuotaExceeded},
		{408, ErrorStatusNetworkError},
		{500, ErrorStatusNetworkError},
		{503, ErrorStatusNetworkError},
		{403, ErrorStatusAPIError},
		{404, ErrorStatusAPIError},
		{0, ErrorStatusAPIError},
	}

	for _, tt := range tests {
		err := MapAPIError("test", tt.code)
		if err.Status != tt.expected {
			t.Errorf("code %d: expected %s, got %s", tt.code, tt.expected, err.Status)
		}
		if err.Code == nil || *err.Code != tt.code {
			t.Errorf("code %d: expected Code=%d, got %v", tt.code, tt.code, err.Code)
		}
	}
}

// --- Tests for hasSpecialToken ---

func TestHasSpecialToken(t *testing.T) {
	tokens := []Token{
		{Text: "hello "},
		{Text: "<end>"},
		{Text: "world"},
	}
	if !hasSpecialToken(tokens, "<end>") {
		t.Error("expected to find <end>")
	}
	if hasSpecialToken(tokens, "<fin>") {
		t.Error("did not expect to find <fin>")
	}
	if hasSpecialToken(nil, "<end>") {
		t.Error("nil tokens should return false")
	}
}

// --- Tests for OnEndpoint and OnFinalized callbacks ---

// mockWSHandlerWithSpecialTokens creates a server that sends <end> and <fin> tokens
func mockWSHandlerWithSpecialTokens(t *testing.T) http.HandlerFunc {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read config
		conn.ReadMessage()

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			// Empty text = stop
			if msgType == websocket.TextMessage && len(msg) == 0 {
				resp := Response{Finished: true, Tokens: []Token{}}
				data, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, data)
				return
			}

			// Control messages
			if msgType == websocket.TextMessage {
				// Check if finalize
				var ctrl map[string]interface{}
				if json.Unmarshal(msg, &ctrl) == nil {
					if ctrl["type"] == "finalize" {
						// Send response with <fin> token
						resp := Response{
							Tokens: []Token{
								{Text: "finalized ", IsFinal: true},
								{Text: "<fin>"},
							},
						}
						data, _ := json.Marshal(resp)
						conn.WriteMessage(websocket.TextMessage, data)
					}
				}
				continue
			}

			// Binary = audio, send result with <end> token
			resp := Response{
				Tokens: []Token{
					{Text: "hello ", IsFinal: true},
					{Text: "<end>"},
				},
				FinalAudioProcMs: 100,
				TotalAudioProcMs: 100,
			}
			data, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

func TestOnEndpointCallback(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		conn.ReadMessage() // config

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage && len(msg) == 0 {
				data, _ := json.Marshal(Response{Finished: true})
				conn.WriteMessage(websocket.TextMessage, data)
				return
			}
			if msgType == websocket.TextMessage {
				continue
			}
			// Audio → respond with <end>
			data, _ := json.Marshal(Response{
				Tokens: []Token{{Text: "hello ", IsFinal: true}, {Text: "<end>"}},
			})
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var endpointCount int
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnEndpoint: func() {
			mu.Lock()
			endpointCount++
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	client.SendAudio([]byte("audio"))
	time.Sleep(100 * time.Millisecond)
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	if endpointCount == 0 {
		t.Error("expected OnEndpoint to be called at least once")
	}
}

func TestOnFinalizedCallback(t *testing.T) {
	server := httptest.NewServer(mockWSHandlerWithSpecialTokens(t))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var finalizedCount int
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnFinalized: func() {
			mu.Lock()
			finalizedCount++
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Finalize should trigger <fin> from server
	client.Finalize()
	time.Sleep(100 * time.Millisecond)
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	if finalizedCount == 0 {
		t.Error("expected OnFinalized to be called at least once")
	}
}

// --- Test for OnToken callback ---

func TestOnTokenCallback(t *testing.T) {
	wsURL := startMockServer(t)

	var tokens []Token
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnToken: func(tok Token) {
			mu.Lock()
			tokens = append(tokens, tok)
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	client.SendAudio([]byte("audio"))
	time.Sleep(100 * time.Millisecond)
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(tokens) == 0 {
		t.Fatal("expected at least one token")
	}
	for _, tok := range tokens {
		if tok.Text == "<end>" || tok.Text == "<fin>" {
			t.Errorf("OnToken should not receive special tokens, got %q", tok.Text)
		}
	}
}

// --- Test for OnDisconnected callback ---

func TestOnDisconnectedCallback(t *testing.T) {
	// Server that closes immediately after config
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		conn.ReadMessage() // config
		conn.Close()       // close immediately
	}))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	disconnected := make(chan string, 1)
	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnDisconnected: func(reason string) {
			disconnected <- reason
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	select {
	case <-disconnected:
		// Good — disconnected callback was called
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnDisconnected")
	}

	// State should be Closed
	deadline := time.After(5 * time.Second)
	for {
		if client.State() == StateClosed {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected Closed state, got %s", client.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// --- Test for StateClosed ---

func TestStateClosedIsTerminalAndInactive(t *testing.T) {
	if !StateClosed.IsTerminal() {
		t.Error("StateClosed should be terminal")
	}
	if !StateClosed.IsInactive() {
		t.Error("StateClosed should be inactive")
	}
	if StateClosed.IsActive() {
		t.Error("StateClosed should not be active")
	}
	if StateClosed.IsWebSocketActive() {
		t.Error("StateClosed should not be WebSocket active")
	}
}

// --- Test for typed API error responses ---

func TestTypedAPIErrorBadRequest(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		conn.ReadMessage() // config
		code := 400
		data, _ := json.Marshal(Response{ErrorCode: &code, ErrorMessage: "bad request"})
		conn.WriteMessage(websocket.TextMessage, data)
	}))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var receivedErr *Error
	errDone := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
		OnError: func(err *Error) {
			receivedErr = err
			close(errDone)
		},
	})
	defer client.Close()

	client.Start(context.Background(), SessionOptions{})

	select {
	case <-errDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	if receivedErr.Status != ErrorStatusBadRequest {
		t.Errorf("expected bad_request, got %s", receivedErr.Status)
	}
}

func TestTypedAPIErrorQuota(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		conn.ReadMessage()
		code := 429
		data, _ := json.Marshal(Response{ErrorCode: &code, ErrorMessage: "rate limited"})
		conn.WriteMessage(websocket.TextMessage, data)
	}))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var receivedErr *Error
	errDone := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
		OnError: func(err *Error) {
			receivedErr = err
			close(errDone)
		},
	})
	defer client.Close()

	client.Start(context.Background(), SessionOptions{})

	select {
	case <-errDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	if receivedErr.Status != ErrorStatusQuotaExceeded {
		t.Errorf("expected quota_exceeded, got %s", receivedErr.Status)
	}
}

// --- Test for MaxEndpointDelayMs in request ---

func TestMaxEndpointDelayMsInRequest(t *testing.T) {
	opts := SessionOptions{
		EnableEndpointDetection: true,
		MaxEndpointDelayMs:      1500,
	}
	opts.applyDefaults()
	req := opts.toRequest("test-key")

	if req.MaxEndpointDelayMs != 1500 {
		t.Errorf("expected MaxEndpointDelayMs=1500, got %d", req.MaxEndpointDelayMs)
	}

	data, _ := json.Marshal(req)
	if !strings.Contains(string(data), `"max_endpoint_delay_ms":1500`) {
		t.Errorf("JSON should contain max_endpoint_delay_ms: %s", string(data))
	}

	// Zero value should be omitted
	opts2 := SessionOptions{}
	opts2.applyDefaults()
	req2 := opts2.toRequest("test-key")
	data2, _ := json.Marshal(req2)
	if strings.Contains(string(data2), "max_endpoint_delay_ms") {
		t.Errorf("max_endpoint_delay_ms should be omitted when zero: %s", string(data2))
	}
}

// --- Test that special tokens are filtered but events still fire ---

func TestSpecialTokensFilteredButEventsEmitted(t *testing.T) {
	server := httptest.NewServer(mockWSHandlerWithSpecialTokens(t))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	var resultTokenTexts []string
	var endpointCalled bool
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnResult: func(resp *Response) {
			mu.Lock()
			for _, tok := range resp.Tokens {
				resultTokenTexts = append(resultTokenTexts, tok.Text)
			}
			mu.Unlock()
		},
		OnEndpoint: func() {
			mu.Lock()
			endpointCalled = true
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	client.SendAudio([]byte("audio"))
	time.Sleep(100 * time.Millisecond)
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	// Tokens should NOT contain <end>
	for _, text := range resultTokenTexts {
		if text == "<end>" || text == "<fin>" {
			t.Errorf("special token %q should have been filtered from OnResult", text)
		}
	}

	// But endpoint event should have fired
	if !endpointCalled {
		t.Error("OnEndpoint should have been called even though <end> was filtered from tokens")
	}
}

// --- Test session-level callback override for new callbacks ---

func TestSessionCallbackOverridesClient(t *testing.T) {
	wsURL := startMockServer(t)

	var clientEndpoint, sessionEndpoint bool
	var mu sync.Mutex
	done := make(chan struct{})

	client := NewClient(ClientOptions{
		APIKey:       "test-key",
		WebSocketURL: wsURL,
		OnEndpoint: func() {
			mu.Lock()
			clientEndpoint = true
			mu.Unlock()
		},
	})
	defer client.Close()

	err := client.Start(context.Background(), SessionOptions{
		OnEndpoint: func() {
			mu.Lock()
			sessionEndpoint = true
			mu.Unlock()
		},
		OnFinished: func() {
			close(done)
		},
	})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	client.SendAudio([]byte("audio"))
	time.Sleep(100 * time.Millisecond)
	client.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	if clientEndpoint {
		t.Error("client-level OnEndpoint should NOT be called when session-level is set")
	}
	if !sessionEndpoint {
		t.Error("session-level OnEndpoint should be called")
	}
}
