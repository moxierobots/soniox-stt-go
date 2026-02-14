// Package soniox provides a Go SDK for the Soniox Speech-to-Text WebSocket API.
package soniox

type TranslationType string

const (
	TranslationTypeOneWay TranslationType = "one_way"
	TranslationTypeTwoWay TranslationType = "two_way"
)

type TranslationConfig struct {
	Type           TranslationType `json:"type"`
	TargetLanguage string          `json:"target_language,omitempty"`
	LanguageA      string          `json:"language_a,omitempty"`
	LanguageB      string          `json:"language_b,omitempty"`
}

type ContextEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TranslationTerm struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type Context struct {
	General          []ContextEntry    `json:"general,omitempty"`
	Text             string            `json:"text,omitempty"`
	Terms            []string          `json:"terms,omitempty"`
	TranslationTerms []TranslationTerm `json:"translation_terms,omitempty"`
}

// Request is the initial configuration message sent to the API.
type Request struct {
	APIKey                       string             `json:"api_key"`
	Model                        string             `json:"model"`
	AudioFormat                  string             `json:"audio_format,omitempty"`
	SampleRate                   int                `json:"sample_rate,omitempty"`
	NumChannels                  int                `json:"num_channels,omitempty"`
	LanguageHints                []string           `json:"language_hints,omitempty"`
	LanguageHintsStrict          bool               `json:"language_hints_strict,omitempty"`
	Context                      *Context           `json:"context,omitempty"`
	EnableSpeakerDiarization     bool               `json:"enable_speaker_diarization,omitempty"`
	EnableLanguageIdentification bool               `json:"enable_language_identification,omitempty"`
	EnableEndpointDetection      bool               `json:"enable_endpoint_detection,omitempty"`
	MaxEndpointDelayMs           int                `json:"max_endpoint_delay_ms,omitempty"`
	ClientReferenceID            string             `json:"client_reference_id,omitempty"`
	Translation                  *TranslationConfig `json:"translation,omitempty"`
}

type TranslationStatus string

const (
	TranslationStatusOriginal    TranslationStatus = "original"
	TranslationStatusTranslation TranslationStatus = "translation"
	TranslationStatusNone        TranslationStatus = "none"
)

type Token struct {
	Text              string            `json:"text"`
	StartMs           *int              `json:"start_ms,omitempty"`
	EndMs             *int              `json:"end_ms,omitempty"`
	Confidence        float64           `json:"confidence"`
	IsFinal           bool              `json:"is_final"`
	Speaker           string            `json:"speaker,omitempty"`
	TranslationStatus TranslationStatus `json:"translation_status,omitempty"`
	Language          string            `json:"language,omitempty"`
	SourceLanguage    string            `json:"source_language,omitempty"`
}

type Response struct {
	Tokens           []Token `json:"tokens"`
	FinalAudioProcMs int     `json:"final_audio_proc_ms"`
	TotalAudioProcMs int     `json:"total_audio_proc_ms"`
	Finished         bool    `json:"finished,omitempty"`
	ErrorCode        *int    `json:"error_code,omitempty"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

type FinalizeMessage struct {
	Type              string `json:"type"`
	TrailingSilenceMs *int   `json:"trailing_silence_ms,omitempty"`
}

type FinalizeOptions struct {
	TrailingSilenceMs int
}

type KeepAliveMessage struct {
	Type string `json:"type"`
}

func NewFinalizeMessage(opts ...FinalizeOptions) FinalizeMessage {
	msg := FinalizeMessage{Type: "finalize"}
	if len(opts) > 0 && opts[0].TrailingSilenceMs > 0 {
		v := opts[0].TrailingSilenceMs
		msg.TrailingSilenceMs = &v
	}
	return msg
}

func NewKeepAliveMessage() KeepAliveMessage {
	return KeepAliveMessage{Type: "keepalive"}
}
