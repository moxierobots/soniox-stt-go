package soniox

import "time"

const (
	// DefaultWebSocketURL is the default Soniox WebSocket endpoint.
	DefaultWebSocketURL = "wss://stt-rt.soniox.com/transcribe-websocket"

	// DefaultBufferQueueSize is the default maximum number of messages to buffer.
	DefaultBufferQueueSize = 1000

	// DefaultKeepAliveInterval is the default interval for keep-alive messages.
	DefaultKeepAliveInterval = 5 * time.Second

	// DefaultConnectTimeout is the default timeout for establishing a connection.
	DefaultConnectTimeout = 30 * time.Second

	// DefaultWriteTimeout is the default timeout for writing messages.
	DefaultWriteTimeout = 10 * time.Second

	// DefaultModel is the default speech-to-text model.
	DefaultModel = "stt-rt-preview"

	// DefaultAudioFormat is the default audio format.
	DefaultAudioFormat = "auto"
)

// APIKeyFunc is a function that returns an API key.
// This is useful for fetching temporary API keys dynamically.
type APIKeyFunc func() (string, error)

// ClientOptions configures the Soniox client.
type ClientOptions struct {
	// WebSocketURL is the WebSocket endpoint URL.
	// Default: DefaultWebSocketURL
	WebSocketURL string

	// APIKey is the Soniox API key (string or function).
	// If a function is provided, it will be called when the connection is established.
	APIKey string

	// APIKeyFunc is a function that returns the API key.
	// Takes precedence over APIKey if both are set.
	APIKeyFunc APIKeyFunc

	// BufferQueueSize is the maximum number of audio chunks to buffer
	// before the WebSocket connection is established.
	// Default: DefaultBufferQueueSize
	BufferQueueSize int

	// KeepAlive enables sending keep-alive messages during silence.
	// Default: false
	KeepAlive bool

	// KeepAliveInterval is the interval between keep-alive messages.
	// Default: DefaultKeepAliveInterval
	KeepAliveInterval time.Duration

	// ConnectTimeout is the timeout for establishing the WebSocket connection.
	// Default: DefaultConnectTimeout
	ConnectTimeout time.Duration

	// WriteTimeout is the timeout for writing messages to the WebSocket.
	// Default: DefaultWriteTimeout
	WriteTimeout time.Duration

	// OnStateChange is called when the client state changes.
	OnStateChange func(oldState, newState State)

	// OnStarted is called when the transcription session starts.
	OnStarted func()

	// OnResult is called when transcription results are received.
	OnResult func(response *Response)

	// OnFinished is called when the transcription session finishes.
	OnFinished func()

	// OnError is called when an error occurs.
	OnError func(err *Error)
}

// SessionOptions configures a transcription session.
type SessionOptions struct {
	// Model specifies the speech-to-text model to use.
	// Default: DefaultModel
	Model string

	// AudioFormat specifies the format of the audio stream.
	// Default: DefaultAudioFormat
	AudioFormat string

	// SampleRate is the sample rate in Hz (required for raw PCM formats).
	SampleRate int

	// NumChannels is the number of audio channels (required for raw PCM formats).
	NumChannels int

	// LanguageHints provides hints about expected languages.
	LanguageHints []string

	// Context provides domain-specific context for improved accuracy.
	Context *Context

	// EnableSpeakerDiarization enables speaker identification.
	EnableSpeakerDiarization bool

	// EnableLanguageIdentification enables language detection.
	EnableLanguageIdentification bool

	// EnableEndpointDetection enables automatic endpoint detection.
	EnableEndpointDetection bool

	// ClientReferenceID is an optional client-defined identifier.
	ClientReferenceID string

	// Translation configures real-time translation.
	Translation *TranslationConfig

	// OnStateChange overrides the client's OnStateChange callback for this session.
	OnStateChange func(oldState, newState State)

	// OnStarted overrides the client's OnStarted callback for this session.
	OnStarted func()

	// OnResult overrides the client's OnResult callback for this session.
	OnResult func(response *Response)

	// OnFinished overrides the client's OnFinished callback for this session.
	OnFinished func()

	// OnError overrides the client's OnError callback for this session.
	OnError func(err *Error)
}

// applyDefaults applies default values to ClientOptions.
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

// applyDefaults applies default values to SessionOptions.
func (o *SessionOptions) applyDefaults() {
	if o.Model == "" {
		o.Model = DefaultModel
	}
	if o.AudioFormat == "" {
		o.AudioFormat = DefaultAudioFormat
	}
}

// toRequest converts SessionOptions to an API Request.
func (o *SessionOptions) toRequest(apiKey string) *Request {
	return &Request{
		APIKey:                       apiKey,
		Model:                        o.Model,
		AudioFormat:                  o.AudioFormat,
		SampleRate:                   o.SampleRate,
		NumChannels:                  o.NumChannels,
		LanguageHints:                o.LanguageHints,
		Context:                      o.Context,
		EnableSpeakerDiarization:     o.EnableSpeakerDiarization,
		EnableLanguageIdentification: o.EnableLanguageIdentification,
		EnableEndpointDetection:      o.EnableEndpointDetection,
		ClientReferenceID:            o.ClientReferenceID,
		Translation:                  o.Translation,
	}
}
