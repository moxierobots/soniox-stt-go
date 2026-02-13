// Example: Real-time translation from an audio file.
//
// Usage:
//
//	# One-way translation (translate everything to English)
//	go run main.go -api-key YOUR_API_KEY -file audio.wav -target en
//
//	# Two-way translation (between English and Spanish)
//	go run main.go -api-key YOUR_API_KEY -file audio.wav -lang-a en -lang-b es
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
	targetLang := flag.String("target", "", "Target language for one-way translation")
	langA := flag.String("lang-a", "", "Language A for two-way translation")
	langB := flag.String("lang-b", "", "Language B for two-way translation")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("SONIOX_API_KEY")
		if *apiKey == "" {
			log.Fatal("API key is required. Use -api-key flag or SONIOX_API_KEY environment variable.")
		}
	}

	if *audioFile == "" {
		log.Fatal("Audio file is required. Use -file flag.")
	}

	// Determine translation mode
	var translation *soniox.TranslationConfig
	if *targetLang != "" {
		translation = &soniox.TranslationConfig{
			Type:           soniox.TranslationTypeOneWay,
			TargetLanguage: *targetLang,
		}
		log.Printf("One-way translation to: %s", *targetLang)
	} else if *langA != "" && *langB != "" {
		translation = &soniox.TranslationConfig{
			Type:      soniox.TranslationTypeTwoWay,
			LanguageA: *langA,
			LanguageB: *langB,
		}
		log.Printf("Two-way translation between: %s <-> %s", *langA, *langB)
	} else {
		log.Fatal("Translation configuration required. Use -target for one-way or -lang-a and -lang-b for two-way.")
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

	// Track tokens by translation status
	var transcription strings.Builder
	var translationText strings.Builder

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
		EnableLanguageIdentification: true,
		Translation:                  translation,
		OnResult: func(response *soniox.Response) {
			for _, token := range response.Tokens {
				if token.IsFinal {
					if token.TranslationStatus == soniox.TranslationStatusTranslation {
						translationText.WriteString(token.Text)
					} else {
						transcription.WriteString(token.Text)
					}
				}
			}
		},
		OnFinished: func() {
			fmt.Println("\n--- Results ---")
			fmt.Println("\nTranscription:")
			fmt.Println(transcription.String())
			fmt.Println("\nTranslation:")
			fmt.Println(translationText.String())
		},
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
				if err := client.Stop(); err != nil {
					log.Printf("Error stopping: %v", err)
				}

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

			time.Sleep(10 * time.Millisecond)
		}
	}
}
