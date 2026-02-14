package soniox

import "time"

const (
	DefaultWebSocketURL    = "wss://stt-rt.soniox.com/transcribe-websocket"
	DefaultBufferQueueSize = 1000
	DefaultKeepAliveInterval = 5 * time.Second
	DefaultConnectTimeout    = 30 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultModel             = "stt-rt-v4"
	DefaultAudioFormat       = "auto"
	DefaultStreamChunkSize   = 4096
)

// APIKeyFunc returns an API key dynamically (e.g. temporary keys).
type APIKeyFunc func() (string, error)

type ClientOptions struct {
	WebSocketURL      string
	APIKey            string
	APIKeyFunc        APIKeyFunc // takes precedence over APIKey
	BufferQueueSize   int
	KeepAlive         bool
	KeepAliveInterval time.Duration
	ConnectTimeout    time.Duration
	WriteTimeout      time.Duration

	OnStateChange  func(oldState, newState State)
	OnStarted      func()
	OnResult       func(response *Response)
	OnToken        func(token Token)
	OnEndpoint     func()
	OnFinalized    func()
	OnFinished     func()
	OnDisconnected func(reason string)
	OnError        func(err *Error)
}

type SessionOptions struct {
	Model                        string
	AudioFormat                  string
	SampleRate                   int
	NumChannels                  int
	LanguageHints                []string
	LanguageHintsStrict          bool
	Context                      *Context
	EnableSpeakerDiarization     bool
	EnableLanguageIdentification bool
	EnableEndpointDetection      bool
	MaxEndpointDelayMs           int
	ClientReferenceID            string
	Translation                  *TranslationConfig

	// Per-session callback overrides (take precedence over ClientOptions).
	OnStateChange  func(oldState, newState State)
	OnStarted      func()
	OnResult       func(response *Response)
	OnToken        func(token Token)
	OnEndpoint     func()
	OnFinalized    func()
	OnFinished     func()
	OnDisconnected func(reason string)
	OnError        func(err *Error)
}

func (o *ClientOptions) applyDefaults() {
	if o.WebSocketURL == "" {
		o.WebSocketURL = DefaultWebSocketURL
	}
	if o.BufferQueueSize == 0 {
		o.BufferQueueSize = DefaultBufferQueueSize
	}
	if o.KeepAliveInterval == 0 {
		o.KeepAliveInterval = DefaultKeepAliveInterval
	}
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = DefaultConnectTimeout
	}
	if o.WriteTimeout == 0 {
		o.WriteTimeout = DefaultWriteTimeout
	}
}

func (o *SessionOptions) applyDefaults() {
	if o.Model == "" {
		o.Model = DefaultModel
	}
	if o.AudioFormat == "" {
		o.AudioFormat = DefaultAudioFormat
	}
}

func (o *SessionOptions) toRequest(apiKey string) *Request {
	return &Request{
		APIKey:                       apiKey,
		Model:                        o.Model,
		AudioFormat:                  o.AudioFormat,
		SampleRate:                   o.SampleRate,
		NumChannels:                  o.NumChannels,
		LanguageHints:                o.LanguageHints,
		LanguageHintsStrict:          o.LanguageHintsStrict,
		Context:                      o.Context,
		EnableSpeakerDiarization:     o.EnableSpeakerDiarization,
		EnableLanguageIdentification: o.EnableLanguageIdentification,
		EnableEndpointDetection:      o.EnableEndpointDetection,
		MaxEndpointDelayMs:           o.MaxEndpointDelayMs,
		ClientReferenceID:            o.ClientReferenceID,
		Translation:                  o.Translation,
	}
}

type SendStreamOptions struct {
	ChunkSize    int
	PaceInterval time.Duration
	Finish       bool // calls Stop() after the stream is fully sent
}
