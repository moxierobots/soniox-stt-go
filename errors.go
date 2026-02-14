package soniox

import (
	"errors"
	"fmt"
)

type ErrorStatus string

const (
	ErrorStatusAPIKeyFetchFailed ErrorStatus = "api_key_fetch_failed"
	ErrorStatusQueueLimitExceeded ErrorStatus = "queue_limit_exceeded"
	ErrorStatusAPIError          ErrorStatus = "api_error"
	ErrorStatusAuthError         ErrorStatus = "auth_error"
	ErrorStatusBadRequest        ErrorStatus = "bad_request"
	ErrorStatusQuotaExceeded     ErrorStatus = "quota_exceeded"
	ErrorStatusNetworkError      ErrorStatus = "network_error"
	ErrorStatusWebSocketError    ErrorStatus = "websocket_error"
	ErrorStatusConnectionClosed  ErrorStatus = "connection_closed"
	ErrorStatusInvalidState      ErrorStatus = "invalid_state"
)

type Error struct {
	Status  ErrorStatus
	Message string
	Code    *int
	Cause   error
}

func (e *Error) Error() string {
	if e.Code != nil {
		return fmt.Sprintf("soniox: %s (code=%d): %s", e.Status, *e.Code, e.Message)
	}
	return fmt.Sprintf("soniox: %s: %s", e.Status, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func NewError(status ErrorStatus, message string) *Error {
	return &Error{
		Status:  status,
		Message: message,
	}
}

func NewErrorWithCode(status ErrorStatus, message string, code int) *Error {
	return &Error{
		Status:  status,
		Message: message,
		Code:    &code,
	}
}

func NewErrorWithCause(status ErrorStatus, message string, cause error) *Error {
	return &Error{
		Status:  status,
		Message: message,
		Cause:   cause,
	}
}

func IsErrorStatus(err error, status ErrorStatus) bool {
	var sonioxErr *Error
	if errors.As(err, &sonioxErr) {
		return sonioxErr.Status == status
	}
	return false
}

var (
	ErrClientNotConnected  = NewError(ErrorStatusInvalidState, "client is not connected")
	ErrClientAlreadyActive = NewError(ErrorStatusInvalidState, "client is already active")
	ErrClientClosed        = NewError(ErrorStatusInvalidState, "client is closed")
)

// MapAPIError maps an API error code to a typed ErrorStatus.
func MapAPIError(message string, code int) *Error {
	var status ErrorStatus
	switch code {
	case 401:
		status = ErrorStatusAuthError
	case 400:
		status = ErrorStatusBadRequest
	case 402, 429:
		status = ErrorStatusQuotaExceeded
	case 408, 500, 503:
		status = ErrorStatusNetworkError
	default:
		status = ErrorStatusAPIError
	}
	return NewErrorWithCode(status, message, code)
}
