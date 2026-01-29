package soniox

import (
	"errors"
	"fmt"
)

// ErrorStatus represents the type of error that occurred.
type ErrorStatus string

const (
	// ErrorStatusAPIKeyFetchFailed indicates the API key fetch failed.
	ErrorStatusAPIKeyFetchFailed ErrorStatus = "api_key_fetch_failed"

	// ErrorStatusQueueLimitExceeded indicates the message queue limit was exceeded.
	ErrorStatusQueueLimitExceeded ErrorStatus = "queue_limit_exceeded"

	// ErrorStatusAPIError indicates an error returned by the Soniox API.
	ErrorStatusAPIError ErrorStatus = "api_error"

	// ErrorStatusWebSocketError indicates a WebSocket communication error.
	ErrorStatusWebSocketError ErrorStatus = "websocket_error"

	// ErrorStatusConnectionClosed indicates the connection was unexpectedly closed.
	ErrorStatusConnectionClosed ErrorStatus = "connection_closed"

	// ErrorStatusInvalidState indicates an operation was attempted in an invalid state.
	ErrorStatusInvalidState ErrorStatus = "invalid_state"
)

// Error represents a Soniox client error.
type Error struct {
	// Status categorizes the error type.
	Status ErrorStatus

	// Message provides a human-readable error description.
	Message string

	// Code is the API error code (only set for API errors).
	Code *int

	// Cause is the underlying error, if any.
	Cause error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Code != nil {
		return fmt.Sprintf("soniox: %s (code=%d): %s", e.Status, *e.Code, e.Message)
	}
	return fmt.Sprintf("soniox: %s: %s", e.Status, e.Message)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// NewError creates a new Soniox error.
func NewError(status ErrorStatus, message string) *Error {
	return &Error{
		Status:  status,
		Message: message,
	}
}

// NewErrorWithCode creates a new Soniox error with an API error code.
func NewErrorWithCode(status ErrorStatus, message string, code int) *Error {
	return &Error{
		Status:  status,
		Message: message,
		Code:    &code,
	}
}

// NewErrorWithCause creates a new Soniox error with an underlying cause.
func NewErrorWithCause(status ErrorStatus, message string, cause error) *Error {
	return &Error{
		Status:  status,
		Message: message,
		Cause:   cause,
	}
}

// IsErrorStatus checks if the error is a Soniox error with the given status.
func IsErrorStatus(err error, status ErrorStatus) bool {
	var sonioxErr *Error
	if errors.As(err, &sonioxErr) {
		return sonioxErr.Status == status
	}
	return false
}

// Common errors
var (
	// ErrClientNotConnected is returned when an operation requires an active connection.
	ErrClientNotConnected = NewError(ErrorStatusInvalidState, "client is not connected")

	// ErrClientAlreadyActive is returned when starting a client that is already active.
	ErrClientAlreadyActive = NewError(ErrorStatusInvalidState, "client is already active")

	// ErrClientClosed is returned when operating on a closed client.
	ErrClientClosed = NewError(ErrorStatusInvalidState, "client is closed")
)
