package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	soniox "github.com/moxierobots/soniox-stt-go"
)

func main() {
	apiKey := os.Getenv("SONIOX_API_KEY")
	if apiKey == "" {
		log.Fatal("SONIOX_API_KEY environment variable is required")
	}

	// Get audio files from command line
	audioFiles := os.Args[1:]
	if len(audioFiles) == 0 {
		log.Fatal("Usage: go run main.go <audio_file1> [audio_file2] ...")
	}

	for _, filepath := range audioFiles {
		fmt.Printf("\n=== Transcribing: %s ===\n", filepath)

		if err := transcribeFile(apiKey, filepath); err != nil {
			log.Printf("Error transcribing %s: %v", filepath, err)
		}

		fmt.Println()
	}
}

func transcribeFile(apiKey, filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	var finalText string
	var transcribeErr error

	client := soniox.NewClient(soniox.ClientOptions{
		APIKey: apiKey,
		OnStateChange: func(oldState, newState soniox.State) {
			log.Printf("State: %s -> %s", oldState, newState)
		},
		OnError: func(err *soniox.Error) {
			log.Printf("Error: %v", err)
			transcribeErr = err
			wg.Done()
		},
	})
	defer client.Close()

	sessionOpts := soniox.SessionOptions{
		Model:                        "stt-rt-preview",
		AudioFormat:                  "auto",
		EnableLanguageIdentification: true,
		EnableEndpointDetection:      true,
		OnResult: func(response *soniox.Response) {
			for _, token := range response.Tokens {
				// Skip endpoint detection markers
				if token.Text == "<end>" {
					continue
				}
				if token.IsFinal {
					finalText += token.Text
					fmt.Print(token.Text)
				}
			}
		},
		OnFinished: func() {
			log.Println("Transcription finished")
			wg.Done()
		},
	}

	if err := client.Start(ctx, sessionOpts); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	// Read and send audio data
	buffer := make([]byte, 4096)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		if err := client.SendAudio(buffer[:n]); err != nil {
			return fmt.Errorf("error sending audio: %w", err)
		}

		// Small delay to simulate streaming
		time.Sleep(5 * time.Millisecond)
	}

	// Stop and wait for completion
	if err := client.Stop(); err != nil {
		return fmt.Errorf("error stopping: %w", err)
	}

	// Wait for finished callback
	wg.Wait()

	if transcribeErr != nil {
		return transcribeErr
	}

	fmt.Printf("\n\nFinal transcription: %s\n", finalText)
	return nil
}
