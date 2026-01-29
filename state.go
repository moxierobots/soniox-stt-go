package soniox

// State represents the current state of the client.
type State string

const (
	// StateInit is the initial state before any operation.
	StateInit State = "Init"

	// StateConnecting indicates the client is establishing a WebSocket connection.
	StateConnecting State = "Connecting"

	// StateRunning indicates the client is actively streaming and receiving transcriptions.
	StateRunning State = "Running"

	// StateFinishing indicates the client is finishing processing buffered audio.
	StateFinishing State = "Finishing"

	// StateFinished indicates the transcription has completed successfully.
	StateFinished State = "Finished"

	// StateError indicates an error occurred during transcription.
	StateError State = "Error"

	// StateCanceled indicates the transcription was canceled by the user.
	StateCanceled State = "Canceled"
)

// IsActive returns true if the state indicates an active transcription session.
func (s State) IsActive() bool {
	switch s {
	case StateConnecting, StateRunning, StateFinishing:
		return true
	default:
		return false
	}
}

// IsInactive returns true if the state indicates an inactive transcription session.
func (s State) IsInactive() bool {
	switch s {
	case StateInit, StateFinished, StateError, StateCanceled:
		return true
	default:
		return false
	}
}

// IsWebSocketActive returns true if a WebSocket connection should be active.
func (s State) IsWebSocketActive() bool {
	switch s {
	case StateConnecting, StateRunning, StateFinishing:
		return true
	default:
		return false
	}
}

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}
