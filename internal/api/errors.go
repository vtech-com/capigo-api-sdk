package api

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Product-domain error codes returned by /pcms/* endpoints.
// These are informational constants; ExitCodeFor() maps HTTP status → exit code.
const (
	ErrProductNotFound     = "E9417" // product not found (404)
	ErrProductCreateFailed = "E9418" // create failed (500)
	ErrProductUpdateFailed = "E9419" // update failed (500)
	ErrVariantNotFound     = "E9425" // variant not found (404)
	ErrProductValidation1  = "E9443" // validation: name
	ErrProductValidation2  = "E9444" // validation: status
	ErrProductValidation3  = "E9445" // validation: currency
	ErrProductValidation4  = "E9446" // validation: options/variants pairing
	ErrProductVariantLimit = "E9447" // validation: variant limit exceeded (max 50)
	ErrTenantDenied        = "E9103" // tenant access denied (403)
	ErrAuthRequired        = "E0004" // auth required (401)
	ErrInsufficientPerms   = "E0102" // insufficient permissions (403)
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
// 8 — conflict (HTTP 409)
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
		case 409:
			return 8
		case 429:
			return 7
		}
	}

	return 1
}
