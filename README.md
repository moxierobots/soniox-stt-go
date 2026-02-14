# Soniox Go SDK

**Unofficial** Go SDK for the [Soniox Speech-to-Text](https://soniox.com) real-time WebSocket API. Enable real-time speech-to-text transcription and translation in your Go applications.

> **Reference SDK:** This SDK is built to be feature-compatible with the official server-side Node.js SDK [`@soniox/node`](https://github.com/soniox/soniox-js) (specifically the `packages/node/src/realtime/` module). API surface, event semantics, error mapping, and special token handling all match the Node.js SDK behavior.
>
> **Note:** The older [`soniox/soniox-node`](https://github.com/soniox/soniox-node) repo is archived and should not be used. The current official SDK lives at **[github.com/soniox/soniox-js](https://github.com/soniox/soniox-js)**.

## Feature Comparison with Official Node SDK

| Feature | Node SDK (`@soniox/node`) | Go SDK (this package) |
|---------|:------------------------:|:---------------------:|
| Real-time WebSocket transcription | ✅ | ✅ |
| Pause / Resume | ✅ | ✅ |
| Keep-alive | ✅ | ✅ |
| SendStream (`io.Reader`) | ✅ | ✅ |
| Finalize (with trailing silence) | ✅ | ✅ |
| Speaker diarization | ✅ | ✅ |
| Language identification | ✅ | ✅ |
| Endpoint detection (`<end>` → `OnEndpoint`) | ✅ | ✅ |
| Finalized event (`<fin>` → `OnFinalized`) | ✅ | ✅ |
| Max endpoint delay (`max_endpoint_delay_ms`) | ✅ | ✅ |
| Per-token callback (`OnToken`) | ✅ | ✅ |
| Disconnected event (`OnDisconnected`) | ✅ | ✅ |
| Typed error mapping (auth, quota, etc.) | ✅ | ✅ |
| Special token filtering (`<end>`, `<fin>`) | ✅ | ✅ |
| Real-time translation (one-way & two-way) | ✅ | ✅ |
| Domain-specific context | ✅ | ✅ |
| Language hints (strict mode) | ✅ | ✅ |
| Context cancellation / AbortSignal | ✅ | ✅ |
| `Closed` state for unexpected disconnects | ✅ | ✅ |
| Async file transcription | ✅ | ❌ |
| Files API (upload, list, delete) | ✅ | ❌ |
| Models API | ✅ | ❌ |
| Temporary API keys (Auth API) | ✅ | ❌ |
| Webhooks | ✅ | ❌ |
| Segment / Utterance buffers | ✅ | ❌ |
| Async iterator (`for await`) | ✅ | ❌ |

## Installation

```bash
go get github.com/moxierobots/soniox-stt-go
```

Requires Go 1.21+.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    soniox "github.com/moxierobots/soniox-stt-go"
)

func main() {
    client := soniox.NewClient(soniox.ClientOptions{
        APIKey: os.Getenv("SONIOX_API_KEY"),
    })
    defer client.Close()

    err := client.Start(context.Background(), soniox.SessionOptions{
        Model:                   "stt-rt-v4",
        EnableEndpointDetection: true,
        OnResult: func(response *soniox.Response) {
            for _, token := range response.Tokens {
                if token.IsFinal {
                    fmt.Print(token.Text)
                }
            }
        },
        OnEndpoint: func() {
            fmt.Println() // newline on utterance boundary
        },
        OnError: func(err *soniox.Error) {
            log.Printf("Error [%s]: %v", err.Status, err)
        },
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
| `APIKey` | `string` | – | Your Soniox API key |
| `APIKeyFunc` | `func() (string, error)` | – | Function to fetch API key dynamically (takes precedence over `APIKey`) |
| `WebSocketURL` | `string` | `wss://stt-rt.soniox.com/transcribe-websocket` | WebSocket endpoint |
| `BufferQueueSize` | `int` | `1000` | Max messages to buffer before connection is established |
| `KeepAlive` | `bool` | `false` | Send keep-alive messages during silence |
| `KeepAliveInterval` | `time.Duration` | `5s` | Interval between keep-alive messages |
| `ConnectTimeout` | `time.Duration` | `30s` | WebSocket connection timeout |
| `WriteTimeout` | `time.Duration` | `10s` | WebSocket write timeout |

### Session Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Model` | `string` | `stt-rt-v4` | Speech-to-text model |
| `AudioFormat` | `string` | `auto` | Audio format (see [Audio Formats](#audio-formats)) |
| `SampleRate` | `int` | – | Sample rate in Hz (required for raw PCM) |
| `NumChannels` | `int` | – | Number of audio channels (required for raw PCM) |
| `LanguageHints` | `[]string` | – | Expected language codes (e.g. `[]string{"en", "es"}`) |
| `LanguageHintsStrict` | `bool` | `false` | Enforce strict adherence to language hints |
| `Context` | `*Context` | – | Domain-specific context for improved accuracy |
| `EnableSpeakerDiarization` | `bool` | `false` | Enable speaker identification |
| `EnableLanguageIdentification` | `bool` | `false` | Enable language detection |
| `EnableEndpointDetection` | `bool` | `false` | Enable endpoint detection for faster finalization |
| `MaxEndpointDelayMs` | `int` | `2000` | Max endpoint delay in ms (500–3000, requires endpoint detection) |
| `ClientReferenceID` | `string` | – | Client-defined identifier for tracking |
| `Translation` | `*TranslationConfig` | – | Translation configuration (see [Translation](#translation)) |

### Callbacks

Both `ClientOptions` and `SessionOptions` support these callbacks. Session-level callbacks take precedence over client-level ones.

| Callback | Signature | Description |
|----------|-----------|-------------|
| `OnStateChange` | `func(oldState, newState State)` | Called on state transitions |
| `OnStarted` | `func()` | Called when the session starts streaming |
| `OnResult` | `func(response *Response)` | Called for each transcription result (special tokens filtered) |
| `OnToken` | `func(token Token)` | Called for each individual non-special token |
| `OnEndpoint` | `func()` | Called when an endpoint is detected (`<end>` token). Critical for voice-agent workflows. |
| `OnFinalized` | `func()` | Called when manual finalization completes (`<fin>` token). Critical for push-to-talk workflows. |
| `OnFinished` | `func()` | Called when the session completes |
| `OnDisconnected` | `func(reason string)` | Called when the WebSocket connection closes unexpectedly |
| `OnError` | `func(err *Error)` | Called when an error occurs |

> **How special tokens work:** The Soniox API sends `<end>` tokens when it detects an utterance boundary and `<fin>` tokens when a manual `Finalize()` completes. These tokens are detected *before* filtering, then removed from `OnResult`/`OnToken` so users never see them. Instead, `OnEndpoint` and `OnFinalized` callbacks fire. This matches the Node.js SDK `endpoint` and `finalized` events exactly.

## Client API

### Core Methods

| Method | Description |
|--------|-------------|
| `NewClient(opts)` | Create a new client |
| `Start(ctx, sessionOpts)` | Begin a transcription session |
| `SendAudio(data)` | Send raw audio bytes |
| `SendStream(reader, opts?)` | Stream audio from an `io.Reader` |
| `Finalize(opts?)` | Trigger manual finalization of non-final tokens |
| `Stop()` | Gracefully stop, waiting for final results |
| `Cancel()` | Immediately terminate the session |
| `Close()` | Close the client and release all resources |

### Pause / Resume

Pause audio transmission while keeping the connection alive. Useful for push-to-talk or voice-activated interfaces.

| Method | Description |
|--------|-------------|
| `Pause()` | Pause audio — `SendAudio` silently drops data; keep-alive messages maintain the connection |
| `Resume()` | Resume audio transmission |
| `Paused()` | Returns `true` if the client is paused |

```go
// Push-to-talk example
client.Pause()   // User releases button — stop sending audio
// ... silence ...
client.Resume()  // User presses button — start sending audio again
```

### Endpoint Detection & Finalize

For voice-agent and push-to-talk workflows, use endpoint detection and finalization:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model:                   "stt-rt-v4",
    EnableEndpointDetection: true,
    MaxEndpointDelayMs:      1000, // faster endpoint detection (default: 2000)
    OnEndpoint: func() {
        // Speaker finished an utterance — process it
        fmt.Println("[endpoint detected]")
    },
    OnFinalized: func() {
        // Manual finalization confirmed by server
        fmt.Println("[finalization complete]")
    },
})

// Later: manually finalize non-final tokens
client.Finalize()

// Or with trailing silence
client.Finalize(soniox.FinalizeOptions{TrailingSilenceMs: 500})
```

### SendStream

Stream audio from any `io.Reader` (file, HTTP body, pipe, etc.):

```go
file, _ := os.Open("audio.wav")
defer file.Close()

err := client.SendStream(file, soniox.SendStreamOptions{
    ChunkSize:    4096,              // bytes per chunk (default: 4096)
    PaceInterval: 10 * time.Millisecond, // delay between chunks
    Finish:       true,              // call Stop() when stream ends
})
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ChunkSize` | `int` | `4096` | Bytes to read per chunk |
| `PaceInterval` | `time.Duration` | `0` | Delay between sending chunks |
| `Finish` | `bool` | `false` | Automatically call `Stop()` after EOF |

## Audio Formats

| Format | Description |
|--------|-------------|
| `auto` | Automatic detection — works with WAV, MP3, FLAC, OGG, WebM, etc. (recommended) |
| `pcm_s16le` | 16-bit signed little-endian PCM |
| `pcm_f32le` | 32-bit float little-endian PCM |
| `mulaw` | μ-law encoded audio |
| `alaw` | A-law encoded audio |

For raw PCM formats, you must specify `SampleRate` and `NumChannels`:

```go
sessionOpts := soniox.SessionOptions{
    Model:       "stt-rt-v4",
    AudioFormat: "pcm_s16le",
    SampleRate:  16000,
    NumChannels: 1,
}
```

## Translation

### One-Way Translation

Translate all spoken languages into a single target language:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-v4",
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

Translate back and forth between two languages:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-v4",
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

## Domain-Specific Context

Provide contextual hints to improve transcription accuracy:

```go
err := client.Start(ctx, soniox.SessionOptions{
    Model: "stt-rt-v4",
    Context: &soniox.Context{
        General: []soniox.ContextEntry{
            {Key: "domain", Value: "Healthcare"},
            {Key: "topic", Value: "Diabetes management"},
        },
        Terms: []string{"Celebrex", "Zyrtec", "Xanax", "Prilosec"},
        TranslationTerms: []soniox.TranslationTerm{
            {Source: "Celebrex", Target: "セレブレックス"},
        },
    },
})
```

## Using Temporary API Keys

For production applications, use temporary API keys to avoid exposing your main key. Generate them via the [Soniox Auth API](https://soniox.com/docs/stt/api-reference/auth) from your backend:

```go
client := soniox.NewClient(soniox.ClientOptions{
    APIKeyFunc: func() (string, error) {
        // Fetch a temporary API key from your backend
        resp, err := http.Post("https://your-backend/api/soniox-key", "", nil)
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

## Keep-Alive

For applications with periods of silence (voice-activated apps, long sessions):

```go
client := soniox.NewClient(soniox.ClientOptions{
    APIKey:            os.Getenv("SONIOX_API_KEY"),
    KeepAlive:         true,
    KeepAliveInterval: 5 * time.Second,
})
```

Keep-alive messages are also sent automatically when the client is paused via `Pause()`.

## State Management

The client tracks its lifecycle through these states:

| State | Description |
|-------|-------------|
| `Init` | Initial state, before any session |
| `Connecting` | Establishing the WebSocket connection |
| `Running` | Actively streaming and receiving transcriptions |
| `Finishing` | Processing remaining buffered audio after `Stop()` |
| `Finished` | Session completed successfully |
| `Error` | An error occurred |
| `Canceled` | Session was canceled via `Cancel()` or context cancellation |
| `Closed` | WebSocket connection closed unexpectedly (server dropped connection) |

```go
// State helpers
client.State().IsActive()    // true for Connecting, Running, Finishing
client.State().IsInactive()  // true for Init, Finished, Error, Canceled, Closed
client.State().IsTerminal()  // true for Finished, Error, Canceled, Closed

// Listen for state changes
client := soniox.NewClient(soniox.ClientOptions{
    APIKey: apiKey,
    OnStateChange: func(oldState, newState soniox.State) {
        log.Printf("State: %s -> %s", oldState, newState)
    },
})
```

## Context Cancellation

The `context.Context` passed to `Start()` controls the session lifetime. Cancelling it will close the WebSocket connection and set the state to `Canceled`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

client.Start(ctx, sessionOpts)

// Context timeout or cancel() will tear down the session gracefully.
```

## Error Handling

All errors implement the standard `error` interface and can be type-asserted to `*soniox.Error`. API errors are automatically mapped to typed statuses matching the Node.js SDK:

```go
err := client.Start(ctx, sessionOpts)
if err != nil {
    var sonioxErr *soniox.Error
    if errors.As(err, &sonioxErr) {
        switch sonioxErr.Status {
        case soniox.ErrorStatusAuthError:
            log.Printf("Authentication failed (code %d): %s", *sonioxErr.Code, sonioxErr.Message)
        case soniox.ErrorStatusBadRequest:
            log.Printf("Bad request (code %d): %s", *sonioxErr.Code, sonioxErr.Message)
        case soniox.ErrorStatusQuotaExceeded:
            log.Printf("Quota/rate limit exceeded: %s", sonioxErr.Message)
        case soniox.ErrorStatusNetworkError:
            log.Printf("Server error: %s", sonioxErr.Message)
        case soniox.ErrorStatusWebSocketError:
            log.Printf("WebSocket error: %s", sonioxErr.Message)
        default:
            log.Printf("Error [%s]: %s", sonioxErr.Status, sonioxErr.Message)
        }
    }
}
```

| Error Status | HTTP Code(s) | Description |
|-------------|:------------:|-------------|
| `auth_error` | 401 | Authentication failed (invalid API key) |
| `bad_request` | 400 | Bad request (invalid parameters) |
| `quota_exceeded` | 402, 429 | Quota or rate limit exceeded |
| `network_error` | 408, 500, 503 | Server-side network error |
| `api_error` | other | Generic API error |
| `websocket_error` | – | WebSocket communication error |
| `connection_closed` | – | Connection unexpectedly closed |
| `api_key_fetch_failed` | – | `APIKeyFunc` returned an error |
| `queue_limit_exceeded` | – | Buffer or write queue is full |
| `invalid_state` | – | Operation attempted in wrong state |

## Stop vs Cancel

- **`Stop()`**: Gracefully stops the session — sends an end-of-audio signal and waits for the server to process remaining audio. The `OnFinished` callback fires when complete.
- **`Cancel()`**: Immediately terminates the session — closes the WebSocket connection without waiting. No `OnFinished` callback.

Use `Stop()` for user-initiated stops (e.g., "Stop Recording" button).
Use `Cancel()` when you need to immediately discard resources (e.g., page navigation, timeout).

## Performance

The Go SDK includes several optimizations not present in the Node.js SDK:

- **TCP_NODELAY** — Disables Nagle's algorithm so each audio chunk hits the wire immediately instead of being buffered up to 40ms.
- **TLS Session Cache** — Reused across sessions on the same `Client` instance for faster reconnection via TLS session tickets.
- **Zero-hop Direct Writes** — `SendAudio` writes directly to the kernel socket buffer via a mutex. No channel, no goroutine hop. The path is: caller → mutex → syscall.
- **No Write Goroutine** — Only 2 background goroutines (readLoop + keepAliveLoop) instead of the typical 3. Fewer goroutines = less scheduling overhead.
- **RWMutex** — State checks use `RLock` for concurrent readers. Only mutations take the full lock.
- **In-place Token Filtering** — `filterSpecialTokens` reslices in-place with zero allocation.
- **Pre-allocated Message Queue** — Queue capacity is allocated once during `Start()`.

## Running Examples

```bash
# Set your API key
export SONIOX_API_KEY=your-api-key

# Transcribe an audio file
cd examples/transcribe
go run main.go -file audio.wav

# Translate an audio file (one-way to English)
cd examples/translate
go run main.go -file audio.wav -target en

# Two-way translation
cd examples/translate
go run main.go -file audio.wav -lang-a en -lang-b es

# Microphone example (skeleton — see source for setup)
cd examples/microphone
go run main.go
```

## Documentation

- [Soniox Documentation](https://soniox.com/docs)
- [Official Node.js SDK (`@soniox/node`)](https://github.com/soniox/soniox-js) — the reference implementation this Go SDK is built against
- [Node SDK Documentation](https://soniox.com/docs/stt/SDKs/node-SDK)
- [WebSocket API Reference](https://soniox.com/docs/stt/api-reference/websocket-api)
- [Go Package Documentation](https://pkg.go.dev/github.com/moxierobots/soniox-stt-go)

## License

MIT License — see [LICENSE](LICENSE) for details.
