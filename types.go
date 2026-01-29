// Package soniox provides a Go SDK for the Soniox Speech-to-Text WebSocket API.
package soniox

// TranslationType defines the type of translation to perform.
type TranslationType string

const (
	// TranslationTypeOneWay translates all spoken languages into a single target language.
	TranslationTypeOneWay TranslationType = "one_way"
	// TranslationTypeTwoWay translates back and forth between two specified languages.
	TranslationTypeTwoWay TranslationType = "two_way"
)

// TranslationConfig configures real-time translation settings.
type TranslationConfig struct {
	// Type specifies the translation mode: "one_way" or "two_way".
	Type TranslationType `json:"type"`

	// TargetLanguage is the language to translate to (for one-way translation).
	TargetLanguage string `json:"target_language,omitempty"`

	// LanguageA is the first language (for two-way translation).
	LanguageA string `json:"language_a,omitempty"`

	// LanguageB is the second language (for two-way translation).
	LanguageB string `json:"language_b,omitempty"`
}

// ContextEntry represents a key-value pair for general context.
type ContextEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TranslationTerm represents a source-target translation term pair.
type TranslationTerm struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Context provides contextual information to improve transcription accuracy.
type Context struct {
	// General provides key-value pairs for domain and topic context.
	General []ContextEntry `json:"general,omitempty"`

	// Text provides additional textual context.
	Text string `json:"text,omitempty"`

	// Terms is a list of specific terms to improve recognition.
	Terms []string `json:"terms,omitempty"`

	// TranslationTerms provides source-target pairs for translation.
	TranslationTerms []TranslationTerm `json:"translation_terms,omitempty"`
}

// Request represents the initial configuration message sent to the API.
type Request struct {
	// APIKey is the Soniox API key or temporary API key.
	APIKey string `json:"api_key"`

	// Model specifies the speech-to-text model to use.
	Model string `json:"model"`

	// AudioFormat specifies the format of the audio stream (e.g., "auto", "s16le", "f32le").
	AudioFormat string `json:"audio_format,omitempty"`

	// SampleRate is the sample rate in Hz (required for raw PCM formats).
	SampleRate int `json:"sample_rate,omitempty"`

	// NumChannels is the number of audio channels (required for raw PCM formats).
	NumChannels int `json:"num_channels,omitempty"`

	// LanguageHints provides hints about expected languages to improve accuracy.
	LanguageHints []string `json:"language_hints,omitempty"`

	// Context provides domain-specific context to improve recognition.
	Context *Context `json:"context,omitempty"`

	// EnableSpeakerDiarization enables speaker identification.
	EnableSpeakerDiarization bool `json:"enable_speaker_diarization,omitempty"`

	// EnableLanguageIdentification enables language detection.
	EnableLanguageIdentification bool `json:"enable_language_identification,omitempty"`

	// EnableEndpointDetection enables endpoint detection to finalize tokens faster.
	EnableEndpointDetection bool `json:"enable_endpoint_detection,omitempty"`

	// ClientReferenceID is an optional client-defined identifier for tracking.
	ClientReferenceID string `json:"client_reference_id,omitempty"`

	// Translation configures real-time translation settings.
	Translation *TranslationConfig `json:"translation,omitempty"`
}

// TranslationStatus indicates the translation status of a token.
type TranslationStatus string

const (
	// TranslationStatusOriginal indicates the token is in its original language.
	TranslationStatusOriginal TranslationStatus = "original"
	// TranslationStatusTranslation indicates the token is a translation.
	TranslationStatusTranslation TranslationStatus = "translation"
	// TranslationStatusNone indicates no translation status.
	TranslationStatusNone TranslationStatus = "none"
)

// Token represents a recognized speech token.
type Token struct {
	// Text is the recognized text.
	Text string `json:"text"`

	// StartMs is the start time in milliseconds.
	StartMs *int `json:"start_ms,omitempty"`

	// EndMs is the end time in milliseconds.
	EndMs *int `json:"end_ms,omitempty"`

	// Confidence is the recognition confidence score (0.0 to 1.0).
	Confidence float64 `json:"confidence"`

	// IsFinal indicates if the token is finalized and won't change.
	IsFinal bool `json:"is_final"`

	// Speaker is the identified speaker label (when speaker diarization is enabled).
	Speaker string `json:"speaker,omitempty"`

	// TranslationStatus indicates if this token is original or translated.
	TranslationStatus TranslationStatus `json:"translation_status,omitempty"`

	// Language is the detected language code.
	Language string `json:"language,omitempty"`

	// SourceLanguage is the original language when translation is enabled.
	SourceLanguage string `json:"source_language,omitempty"`
}

// Response represents a response from the Speech-to-Text API.
type Response struct {
	// Text is the complete transcribed text.
	Text string `json:"text"`

	// Tokens contains individual recognized tokens with metadata.
	Tokens []Token `json:"tokens"`

	// FinalAudioProcMs is the processing time for finalized audio in milliseconds.
	FinalAudioProcMs int `json:"final_audio_proc_ms"`

	// TotalAudioProcMs is the total audio processing time in milliseconds.
	TotalAudioProcMs int `json:"total_audio_proc_ms"`

	// Finished indicates if the transcription session has completed.
	Finished bool `json:"finished,omitempty"`

	// ErrorCode contains the error code if an error occurred.
	ErrorCode *int `json:"error_code,omitempty"`

	// ErrorMessage contains the error message if an error occurred.
	ErrorMessage string `json:"error_message,omitempty"`
}

// FinalizeMessage is sent to trigger manual finalization.
type FinalizeMessage struct {
	Type string `json:"type"`
}

// KeepAliveMessage is sent to maintain the connection during silence.
type KeepAliveMessage struct {
	Type string `json:"type"`
}

// NewFinalizeMessage creates a new finalize message.
func NewFinalizeMessage() FinalizeMessage {
	return FinalizeMessage{Type: "finalize"}
}

// NewKeepAliveMessage creates a new keep-alive message.
func NewKeepAliveMessage() KeepAliveMessage {
	return KeepAliveMessage{Type: "keepalive"}
}
