// Example: Real-time speech-to-text transcription from an audio file.
//
// Usage:
//
//	go run main.go -api-key YOUR_API_KEY -file audio.wav
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	soniox "github.com/moxierobots/soniox-stt-go"
)

func main() {
	apiKey := flag.String("api-key", "", "Soniox API key")
	audioFile := flag.String("file", "", "Audio file to transcribe")
	model := flag.String("model", "stt-rt-preview", "Model to use")
	languages := flag.String("languages", "", "Comma-separated language hints (e.g., en,es)")
	enableDiarization := flag.Bool("diarization", false, "Enable speaker diarization")
	enableLangID := flag.Bool("lang-id", false, "Enable language identification")
	enableEndpoint := flag.Bool("endpoint", false, "Enable endpoint detection")
	flag.Parse()

	if *apiKey == "" {
		// Try environment variable
		*apiKey = os.Getenv("SONIOX_API_KEY")
		if *apiKey == "" {
			log.Fatal("API key is required. Use -api-key flag or SONIOX_API_KEY environment variable.")
		}
	}

	if *audioFile == "" {
		log.Fatal("Audio file is required. Use -file flag.")
	}

	// Open audio file
	file, err := os.Open(*audioFile)
	if err != nil {
		log.Fatalf("Failed to open audio file: %v", err)
	}
	defer file.Close()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted, stopping...")
		cancel()
	}()

	// Create client
	client := soniox.NewClient(soniox.ClientOptions{
		APIKey: *apiKey,
		OnStateChange: func(oldState, newState soniox.State) {
			log.Printf("State: %s -> %s", oldState, newState)
		},
		OnError: func(err *soniox.Error) {
			log.Printf("Error: %v", err)
		},
	})
	defer client.Close()

	// Prepare session options
	sessionOpts := soniox.SessionOptions{
		Model:                        *model,
		AudioFormat:                  "auto",
		EnableSpeakerDiarization:     *enableDiarization,
		EnableLanguageIdentification: *enableLangID,
		EnableEndpointDetection:      *enableEndpoint,
		OnResult: func(response *soniox.Response) {
			for _, token := range response.Tokens {
				if token.IsFinal {
					fmt.Print(token.Text)
				}
			}
		},
		OnFinished: func() {
			fmt.Println("\n--- Transcription complete ---")
		},
	}

	// Parse language hints
	if *languages != "" {
		sessionOpts.LanguageHints = strings.Split(*languages, ",")
	}

	// Start transcription
	if err := client.Start(ctx, sessionOpts); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	// Read and send audio data
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			client.Cancel()
			return
		default:
			n, err := file.Read(buffer)
			if err == io.EOF {
				// End of file, stop gracefully
				if err := client.Stop(); err != nil {
					log.Printf("Error stopping: %v", err)
				}

				// Wait for completion
				for client.State().IsActive() {
					time.Sleep(100 * time.Millisecond)
				}
				return
			}
			if err != nil {
				log.Fatalf("Error reading file: %v", err)
			}

			if err := client.SendAudio(buffer[:n]); err != nil {
				log.Printf("Error sending audio: %v", err)
			}

			// Simulate real-time streaming (adjust based on audio sample rate)
			time.Sleep(10 * time.Millisecond)
		}
	}
}
