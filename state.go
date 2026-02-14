package soniox

type State string

const (
	StateInit       State = "Init"
	StateConnecting State = "Connecting"
	StateRunning    State = "Running"
	StateFinishing  State = "Finishing"
	StateFinished   State = "Finished"
	StateError      State = "Error"
	StateCanceled   State = "Canceled"
	StateClosed     State = "Closed"
)

func (s State) IsActive() bool {
	switch s {
	case StateConnecting, StateRunning, StateFinishing:
		return true
	default:
		return false
	}
}

func (s State) IsInactive() bool {
	switch s {
	case StateInit, StateFinished, StateError, StateCanceled, StateClosed:
		return true
	default:
		return false
	}
}

func (s State) IsWebSocketActive() bool {
	switch s {
	case StateConnecting, StateRunning, StateFinishing:
		return true
	default:
		return false
	}
}

func (s State) IsTerminal() bool {
	switch s {
	case StateFinished, StateError, StateCanceled, StateClosed:
		return true
	default:
		return false
	}
}

func (s State) String() string {
	return string(s)
}
