# Soniox Speech-to-Text Go SDK

**Unofficial** Go SDK for the [Soniox Speech-to-Text](https://soniox.com) WebSocket API. Enable real-time speech-to-text transcription and translation in your Go applications.

> **Note:** This is an unofficial SDK maintained by Moxie Robots Inc. For the official JavaScript/TypeScript SDK, see [soniox/speech-to-text-web](https://github.com/soniox/speech-to-text-web).

## Features

- Real-time streaming transcription via WebSocket
- Language identification
- Speaker diarization
- Real-time translation (one-way and two-way)
- Domain-specific context for improved accuracy
- Endpoint detection for faster finalization
- Keep-alive for long-running sessions

## Installation

```bash
go get github.com/moxierobots/soniox-speech-to-text-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    soniox "github.com/moxierobots/soniox-speech-to-text-go"
)

func main() {
    client := soniox.NewClient(soniox.ClientOptions{
        APIKey: "your-api-key",
        OnResult: func(response *soniox.Response) {
            for _, token := range response.Tokens {
                if token.IsFinal {
                    fmt.Print(token.Text)
                }
            }
        },
        OnError: func(err *soniox.Error) {
            log.Printf("Error: %v", err)
        },
    })
    defer client.Close()

    err := client.Start(context.Background(), soniox.SessionOptions{
        Model:                    "stt-rt-preview",
        EnableSpeakerDiarization: true,
    })
    if err != nil {
        log.Fatal(err)
    }

    // Send audio data
    // client.SendAudio(audioBytes)

    // Stop when done
    client.Stop()
}
```

## Configuration

### Client Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `APIKey` | `string` | - | Your Soniox API key |
| `APIKeyFunc` | `func() (string, error)` | - | Function to fetch API key dynamically |
| `WebSocketURL` | `string` | `wss://stt-rt.soniox.com/transcribe-websocket` | WebSocket endpoint |
| `BufferQueueSize` | `int` | `1000` | Max messages to buffer before connection |
| `KeepAlive` | `bool` | `false` | Send keep-alive messages during silence |
| `KeepAliveInterval` | `time.Duration` | `5s` | Interval between keep-alive messages |
| `ConnectTimeout` | `time.Duration` | `30s` | Connection timeout |
| `WriteTimeout` | `time.Duration` | `10s` | Write timeout |

### Session Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Model` | `string` | `stt-rt-preview` | Speech-to-text model |
| `AudioFormat` | `string` | `auto` | Audio format (`auto`, `s16le`, `f32le`) |
| `SampleRate` | `int` | - | Sample rate in Hz (required for raw PCM) |
| `NumChannels` | `int` | - | Number of audio channels |
| `LanguageHints` | `[]string` | - | Expected language codes |
| `Context` | `*Context` | - | Domain-specific context |
| `EnableSpeakerDiarization` | `bool` | `false` | Enable speaker identification |
| `EnableLanguageIdentification` | `bool` | `false` | Enable language detection |
| `EnableEndpointDetection` | `bool` | `false` | Enable endpoint detection |
| `Translation` | `*TranslationConfig` | - | Translation configuration |

## Examples

### Basic Transcription

```go
client := soniox.NewClient(soniox.ClientOptions{
    APIKey: os.Getenv("SONIOX_API_KEY"),
})

err := client.Start(ctx, soniox.SessionOptions{
    Model:                        "stt-rt-preview",
    EnableSpeakerDiarization:     true,
    EnableLanguageIdentification: true,
    EnableEndpointDetection:      true,
    LanguageHints:                []string{"en", "es"},
    OnResult: func(response *soniox.Response) {
        for _, token := range response.Tokens {
            if token.IsFinal {
                fmt.Printf("[%s] %s", token.Speaker, token.Text)
            }
        }
    },
})
```

### One-Way Translation

Translate all spoken languages to a target language:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-preview",
    Translation: &soniox.TranslationConfig{
        Type:           soniox.TranslationTypeOneWay,
        TargetLanguage: "en",
    },
    OnResult: func(response *soniox.Response) {
        for _, token := range response.Tokens {
            if token.IsFinal {
                if token.TranslationStatus == soniox.TranslationStatusTranslation {
                    fmt.Print("[Translation] ", token.Text)
                } else {
                    fmt.Print("[Original] ", token.Text)
                }
            }
        }
    },
})
```

### Two-Way Translation

Translate between two languages:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-preview",
    Translation: &soniox.TranslationConfig{
        Type:      soniox.TranslationTypeTwoWay,
        LanguageA: "en",
        LanguageB: "es",
    },
    OnResult: func(response *soniox.Response) {
        for _, token := range response.Tokens {
            if token.IsFinal {
                fmt.Printf("[%s] %s", token.Language, token.Text)
            }
        }
    },
})
```

### Using Context for Better Accuracy

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-preview",
    Context: &soniox.Context{
        General: []soniox.ContextEntry{
            {Key: "domain", Value: "Healthcare"},
            {Key: "topic", Value: "Diabetes management"},
        },
        Terms: []string{"Celebrex", "Zyrtec", "Xanax", "Prilosec"},
    },
    OnResult: func(response *soniox.Response) {
        // Handle results
    },
})
```

### Using Temporary API Keys

For production applications, use temporary API keys to avoid exposing your main API key:

```go
client := soniox.NewClient(soniox.ClientOptions{
    APIKeyFunc: func() (string, error) {
        // Fetch temporary API key from your backend
        resp, err := http.Post("https://your-backend/api/get-temporary-key", "", nil)
        if err != nil {
            return "", err
        }
        defer resp.Body.Close()

        var result struct {
            APIKey string `json:"api_key"`
        }
        if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
            return "", err
        }
        return result.APIKey, nil
    },
})
```

### Keep-Alive for Long Sessions

For applications with periods of silence:

```go
client := soniox.NewClient(soniox.ClientOptions{
    APIKey:            os.Getenv("SONIOX_API_KEY"),
    KeepAlive:         true,
    KeepAliveInterval: 5 * time.Second,
})
```

## Audio Formats

| Format | Description |
|--------|-------------|
| `auto` | Automatic detection (recommended for most cases) |
| `s16le` | 16-bit signed little-endian PCM |
| `f32le` | 32-bit float little-endian PCM |

For raw PCM formats, you must specify `SampleRate` and `NumChannels`:

```go
sessionOpts := soniox.SessionOptions{
    Model:       "stt-rt-preview",
    AudioFormat: "s16le",
    SampleRate:  16000,
    NumChannels: 1,
}
```

## State Management

The client tracks its state throughout the session:

| State | Description |
|-------|-------------|
| `Init` | Initial state |
| `Connecting` | Establishing WebSocket connection |
| `Running` | Actively processing audio |
| `Finishing` | Processing remaining audio |
| `Finished` | Session completed |
| `Error` | An error occurred |
| `Canceled` | Session was canceled |

```go
// Check if client is active
if client.State().IsActive() {
    // Client is processing
}

// Listen for state changes
client := soniox.NewClient(soniox.ClientOptions{
    APIKey: apiKey,
    OnStateChange: func(oldState, newState soniox.State) {
        log.Printf("State: %s -> %s", oldState, newState)
    },
})
```

## Error Handling

```go
err := client.Start(ctx, sessionOpts)
if err != nil {
    var sonioxErr *soniox.Error
    if errors.As(err, &sonioxErr) {
        switch sonioxErr.Status {
        case soniox.ErrorStatusAPIError:
            log.Printf("API error (code %d): %s", *sonioxErr.Code, sonioxErr.Message)
        case soniox.ErrorStatusWebSocketError:
            log.Printf("WebSocket error: %s", sonioxErr.Message)
        default:
            log.Printf("Error: %s", sonioxErr.Message)
        }
    }
}
```

## Stop vs Cancel

- **`Stop()`**: Gracefully stops the session, waiting for all buffered audio to be processed and final results to be received.
- **`Cancel()`**: Immediately terminates the session without waiting for results.

Use `Stop()` for user-initiated stops (e.g., "Stop Recording" button).
Use `Cancel()` when you need to immediately close resources (e.g., component unmount).

## Running Examples

```bash
# Transcribe an audio file
cd examples/transcribe
go run main.go -api-key YOUR_API_KEY -file audio.wav

# Translate an audio file
cd examples/translate
go run main.go -api-key YOUR_API_KEY -file audio.wav -target en
```

## API Documentation

For complete API documentation, see:
- [Soniox Documentation](https://soniox.com/docs)
- [WebSocket API Reference](https://soniox.com/docs/stt/api-reference/websocket-api)
- [Go Package Documentation](https://pkg.go.dev/github.com/moxierobots/soniox-speech-to-text-go)

## License

MIT License - see [LICENSE](LICENSE) for details.
