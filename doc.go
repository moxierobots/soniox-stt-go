// Package soniox provides a Go SDK for the Soniox Speech-to-Text WebSocket API.
//
// This SDK enables real-time speech-to-text transcription and translation
// directly from your Go applications. It supports features like:
//
//   - Real-time streaming transcription
//   - Language identification
//   - Speaker diarization
//   - Real-time translation (one-way and two-way)
//   - Domain-specific context for improved accuracy
//   - Endpoint detection
//
// # Quick Start
//
// Create a client and start a transcription session:
//
//	client := soniox.NewClient(soniox.ClientOptions{
//	    APIKey: "your-api-key",
//	    OnResult: func(response *soniox.Response) {
//	        for _, token := range response.Tokens {
//	            if token.IsFinal {
//	                fmt.Print(token.Text)
//	            }
//	        }
//	    },
//	})
//
//	err := client.Start(ctx, soniox.SessionOptions{
//	    Model: "stt-rt-v4",
//	    EnableSpeakerDiarization: true,
//	})
//
//	// Send audio data
//	client.SendAudio(audioBytes)
//
//	// Stop when done
//	client.Stop()
//
// # Audio Formats
//
// The SDK supports various audio formats:
//
//   - "auto": Automatically detect format (recommended for most cases)
//   - "pcm_s16le": 16-bit signed little-endian PCM
//   - "pcm_f32le": 32-bit float little-endian PCM
//   - Other formats as supported by the Soniox API (wav, mp3, flac, ogg, etc.)
//
// For raw PCM formats, you must specify SampleRate and NumChannels.
//
// # Translation
//
// Enable real-time translation with TranslationConfig:
//
//	// One-way: translate all languages to a target language
//	sessionOpts.Translation = &soniox.TranslationConfig{
//	    Type:           soniox.TranslationTypeOneWay,
//	    TargetLanguage: "en",
//	}
//
//	// Two-way: translate between two languages
//	sessionOpts.Translation = &soniox.TranslationConfig{
//	    Type:      soniox.TranslationTypeTwoWay,
//	    LanguageA: "en",
//	    LanguageB: "es",
//	}
//
// # Error Handling
//
// All errors implement the standard error interface and can be type-asserted
// to *soniox.Error for detailed information:
//
//	if err != nil {
//	    var sonioxErr *soniox.Error
//	    if errors.As(err, &sonioxErr) {
//	        fmt.Printf("Status: %s, Code: %v\n", sonioxErr.Status, sonioxErr.Code)
//	    }
//	}
//
// # Keep-Alive
//
// For applications with periods of silence (e.g., voice-activated apps),
// enable keep-alive to prevent connection timeouts:
//
//	client := soniox.NewClient(soniox.ClientOptions{
//	    APIKey:            "your-api-key",
//	    KeepAlive:         true,
//	    KeepAliveInterval: 5 * time.Second,
//	})
package soniox
