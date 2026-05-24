// Package httpx provides internal HTTP helpers for the Talon Sandbox SDK.
package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// apiError is an HTTP-level error returned by the server. Lives here to avoid
// import cycles; the root package re-wraps it into its public APIError.
type apiError struct {
	statusCode int
	message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("sandbox API error %d: %s", e.statusCode, e.message)
}

// NewAPIError constructs an internal apiError.
func NewAPIError(statusCode int, message string) error {
	return &apiError{statusCode: statusCode, message: message}
}

// IsAPIError reports whether err is an httpx api error.
func IsAPIError(err error) bool { _, ok := err.(*apiError); return ok }

// StatusCode extracts the HTTP status from an httpx api error.
func StatusCode(err error) int {
	if e, ok := err.(*apiError); ok {
		return e.statusCode
	}
	return 0
}

// Message extracts the server message from an httpx api error.
func Message(err error) string {
	if e, ok := err.(*apiError); ok {
		return e.message
	}
	return ""
}

// urlError wraps transport-level errors.
type urlError struct{ cause error }

func (e *urlError) Error() string { return e.cause.Error() }
func (e *urlError) Unwrap() error { return e.cause }

// NewNetworkError wraps a transport-level error.
func NewNetworkError(cause error) error { return &urlError{cause: cause} }

// IsNetworkError reports whether err is a transport-level error.
func IsNetworkError(err error) bool { _, ok := err.(*urlError); return ok }

// DoJSON executes req, decodes JSON body into out (if non-nil), maps 4xx/5xx
// to apiError, transport failures to urlError. Returns raw response for header
// inspection.
func DoJSON(client *http.Client, req *http.Request, out any) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, &urlError{cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, &urlError{cause: fmt.Errorf("read response body: %w", err)}
	}

	if resp.StatusCode >= 400 {
		return resp, NewAPIError(resp.StatusCode, extractErrorMessage(body))
	}

	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp, nil
}

// ExtractErrorMessage parses {"error":"..."} JSON; falls back to raw body.
func ExtractErrorMessage(body []byte) string { return extractErrorMessage(body) }

func extractErrorMessage(body []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		return e.Error
	}
	s := string(body)
	if len(s) > 256 {
		s = s[:256] + "..."
	}
	return s
}
