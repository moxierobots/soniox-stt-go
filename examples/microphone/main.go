// Example: Real-time speech-to-text transcription from microphone.
//
// This example uses PortAudio for microphone capture. You'll need to install PortAudio:
//
//	macOS:   brew install portaudio
//	Ubuntu:  sudo apt-get install portaudio19-dev
//	Windows: Download from http://www.portaudio.com/
//
// Usage:
//
//	go run main.go -api-key YOUR_API_KEY
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	soniox "github.com/moxierobots/soniox-speech-to-text-go"
)

// Note: This is a skeleton example. For actual microphone capture,
// you would need to use a library like:
// - github.com/gordonklaus/portaudio
// - github.com/gen2brain/malgo
//
// The audio capture implementation depends on your specific requirements
// and platform.

func main() {
	apiKey := flag.String("api-key", "", "Soniox API key")
	model := flag.String("model", "stt-rt-preview", "Model to use")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("SONIOX_API_KEY")
		if *apiKey == "" {
			log.Fatal("API key is required. Use -api-key flag or SONIOX_API_KEY environment variable.")
		}
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create client
	client := soniox.NewClient(soniox.ClientOptions{
		APIKey:    *apiKey,
		KeepAlive: true, // Useful for microphone input with pauses
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
		AudioFormat:                  "s16le", // 16-bit signed little-endian PCM
		SampleRate:                   16000,
		NumChannels:                  1,
		EnableSpeakerDiarization:     true,
		EnableLanguageIdentification: true,
		EnableEndpointDetection:      true,
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

	// Start transcription
	if err := client.Start(ctx, sessionOpts); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	fmt.Println("Microphone transcription started. Press Ctrl+C to stop.")
	fmt.Println("NOTE: This example requires microphone capture implementation.")
	fmt.Println("      See comments in source code for guidance.")

	// TODO: Implement microphone capture using PortAudio or similar library
	// Example with PortAudio:
	//
	// import "github.com/gordonklaus/portaudio"
	//
	// portaudio.Initialize()
	// defer portaudio.Terminate()
	//
	// buffer := make([]int16, 512)
	// stream, _ := portaudio.OpenDefaultStream(1, 0, 16000, len(buffer), buffer)
	// stream.Start()
	//
	// for {
	//     stream.Read()
	//     // Convert int16 slice to bytes
	//     audioBytes := int16ToBytes(buffer)
	//     client.SendAudio(audioBytes)
	// }

	// Wait for interrupt
	<-sigChan
	fmt.Println("\nStopping...")

	if err := client.Stop(); err != nil {
		log.Printf("Error stopping: %v", err)
	}
}
