package api

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// APIError represents a structured error returned by the Capigo API.
type APIError struct {
	Code       string
	Message    string
	RequestID  string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s: %s (request_id=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ExitCodeFor maps an error to the standardized exit codes defined in §4.4.
//
// 0 — success (nil error)
// 1 — general / unexpected error
// 2 — auth error (HTTP 401)
// 3 — permission error (HTTP 403)
// 4 — not found (HTTP 404)
// 5 — validation error (HTTP 400)
// 6 — network error (DNS, timeout, connection refused)
// 7 — rate limit (HTTP 429)
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}

	// Network errors: context deadline or net.Error (timeout, DNS, etc.)
	if errors.Is(err, context.DeadlineExceeded) {
		return 6
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return 6
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatus {
		case 401:
			return 2
		case 403:
			return 3
		case 404:
			return 4
		case 400:
			return 5
		case 429:
			return 7
		}
	}

	return 1
}
