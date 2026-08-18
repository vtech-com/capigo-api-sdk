package api

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Product-domain error codes returned by /pcms/* endpoints.
// These are informational constants; ExitCodeFor() maps HTTP status → exit code.
// Meanings are sourced from the backend error-codes.ts — keep them in sync there
// and in error_catalog.go (the user-facing interpretation), never guess.
const (
	ErrProductNotFound     = "E9417" // product not found (404)
	ErrProductCreateFailed = "E9418" // product create failed
	ErrProductUpdateFailed = "E9419" // product update failed
	ErrVariantNotFound     = "E9425" // variant not found / wrong tenant (404)
	ErrVariantCreateFailed = "E9426" // variant create failed
	ErrVariantUpdateFailed = "E9427" // variant update failed
	ErrOptionInvalidName   = "E9443" // product option: invalid name
	ErrOptionInvalidValues = "E9444" // product option: invalid values
	ErrVariantSKUExists    = "E9445" // variant SKU already exists in tenant
	ErrVariantDupCombo     = "E9446" // variant: duplicate option combination
	ErrProductVariantLimit = "E9447" // variant limit exceeded (max 50)
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
	// RawBody is the verbatim response body from the server, when available.
	// It is surfaced on error so callers (especially AI agents) see the real
	// server response without having to re-run with --verbose.
	RawBody []byte
	// LocalDiagnosis marks an error the CLI diagnosed itself, without a round
	// trip, but whose Code is nonetheless a real catalog code the caller should
	// get the full interpretation for. The renderer normally withholds the
	// catalog from errors the server never answered — a cobra arg-validation
	// failure carries no server truth to interpret — so a locally-detected
	// condition that IS the catalog condition has to say so explicitly.
	LocalDiagnosis bool
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
		case 400, 422:
			return 5
		case 409:
			return 8
		case 429:
			return 7
		}
	}

	return 1
}
