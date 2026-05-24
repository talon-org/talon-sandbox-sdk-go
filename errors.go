package talonsandbox

import (
	"errors"
	"fmt"
)

// Sentinel errors for errors.Is matching.
var (
	// ErrAuth is returned for 401 and 403 responses.
	ErrAuth = errors.New("authentication / authorization error")

	// ErrNotFound is returned for 404 responses.
	ErrNotFound = errors.New("not found")

	// ErrQuota is returned for 422 responses (quota exceeded).
	ErrQuota = errors.New("quota exceeded")

	// ErrRateLimit is returned for 429 responses.
	ErrRateLimit = errors.New("rate limit exceeded")

	// ErrServer is returned for 5xx responses.
	ErrServer = errors.New("server error")

	// ErrNetwork is returned when the HTTP transport itself fails.
	ErrNetwork = errors.New("network error")

	// ErrTimeout is returned when a wait-for-state poll exceeds its deadline.
	ErrTimeout = errors.New("timed out waiting for sandbox state")

	// ErrPTYClosed is returned when writing to an already-closed PTY session.
	ErrPTYClosed = errors.New("PTY session is closed")

	// ErrNotImplemented is returned when a server endpoint is not yet available.
	ErrNotImplemented = errors.New("not implemented by server")
)

// APIError carries the HTTP status and server message, wrapping a sentinel.
type APIError struct {
	// StatusCode is the HTTP response status code.
	StatusCode int
	// Message is the "error" field from the JSON response body.
	Message string
	// RequestID is the X-Request-ID response header value (when present).
	RequestID string
	sentinel  error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sandbox API error %d: %s", e.StatusCode, e.Message)
}

// Unwrap enables errors.Is(err, ErrAuth) etc.
func (e *APIError) Unwrap() error { return e.sentinel }

func newAPIError(statusCode int, message string) *APIError {
	var sentinel error
	switch {
	case statusCode == 401 || statusCode == 403:
		sentinel = ErrAuth
	case statusCode == 404:
		sentinel = ErrNotFound
	case statusCode == 422:
		sentinel = ErrQuota
	case statusCode == 429:
		sentinel = ErrRateLimit
	case statusCode >= 500:
		sentinel = ErrServer
	default:
		sentinel = fmt.Errorf("http %d", statusCode)
	}
	return &APIError{StatusCode: statusCode, Message: message, sentinel: sentinel}
}

// NetworkError wraps transport-level failures and unwraps to ErrNetwork.
type NetworkError struct {
	Cause error
}

func (e *NetworkError) Error() string { return fmt.Sprintf("network error: %v", e.Cause) }
func (e *NetworkError) Unwrap() error { return ErrNetwork }
