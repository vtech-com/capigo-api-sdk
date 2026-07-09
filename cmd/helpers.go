package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

const defaultBaseURL = "https://platform.capigo.app/api/v1"

// buildClient loads config and constructs an api.Client using the active profile.
// Env var CAPIGO_API_KEY overrides the profile key; CAPIGO_API_URL and --api-url
// override the profile URL; the default base URL is https://platform.capigo.app/api/v1.
func buildClient() (*api.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	profile, err := config.ActiveProfile(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("get active profile: %w", err)
	}

	apiKey := profile.APIKey
	if k := viper.GetString("api_key"); k != "" {
		apiKey = k
	}

	baseURL := profile.APIURL
	if u := viper.GetString("api_url"); u != "" {
		baseURL = u
	}
	if apiURL != "" {
		baseURL = apiURL
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	client, err := api.NewClient(baseURL, apiKey)
	if err != nil {
		return nil, nil, err
	}

	if verbose {
		client.EnableVerbose(os.Stderr)
	}

	return client, cfg, nil
}

// renderCLIError renders err as a self-diagnosing error: the full diagnostic
// block (meaning, capability brake, next step, raw response) goes to stdout
// where an AI agent reads, and a concise line goes to stderr. It enriches the
// error with the catalog interpretation when the code is known. It does not exit.
func renderCLIError(err error) {
	detail := output.ErrorDetail{Code: "ERROR", Message: err.Error()}
	serverResponded := false
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		detail.Code = apiErr.Code
		detail.Message = apiErr.Message
		detail.RequestID = apiErr.RequestID
		detail.HTTPStatus = apiErr.HTTPStatus
		detail.RawBody = string(apiErr.RawBody)
		serverResponded = len(apiErr.RawBody) > 0
	}
	// Only enrich with the catalog interpretation (meaning, next step, capability
	// brake) when the server actually responded. Locally-constructed and cobra
	// arg-validation errors never round-tripped, so the brake — "a failed write
	// is not a missing capability" — would be misleading noise there.
	if serverResponded {
		if info, ok := api.LookupError(detail.Code); ok {
			detail.Meaning = info.Meaning
			detail.Next = info.Next
			detail.CapabilityNote = info.CapabilityNote
		}
	} else if next, ok := nextForRemovedFlag(detail.Message); ok {
		// A flag we deleted. Say so, rather than letting cobra's "unknown
		// shorthand flag" read as a typo the caller should retype.
		detail.Next = next
	} else if next, ok := redirectForUnknownCommand(detail.Message); ok {
		// Curated cross-group redirect (Layer 2, cmd/unknown_command.go) for a
		// conceptual wrong-guess cobra's own distance-based suggestions can't
		// express (the right command lives in a different group). Deliberately
		// does NOT set CapabilityNote: the "a failed write is not a missing
		// capability" brake is reserved for real server-side write failures,
		// not client-side/cobra errors.
		detail.Next = next
	}
	// Avoid printing the raw body twice when it is identical to the parsed message
	// (non-enveloped error responses set both to the same bytes).
	if detail.RawBody == detail.Message {
		detail.RawBody = ""
	}
	output.RenderError(os.Stdout, os.Stderr, detail)
}

// handleErr renders the error and exits with the appropriate exit code.
// Returns nil so RunE callers do not trigger cobra's double-print.
func handleErr(err error) error {
	renderCLIError(err)
	os.Exit(api.ExitCodeFor(err))
	return nil
}

// fail renders a locally-detected error — a bad flag combination, an
// unreadable config — and exits. The server was never called, so there is no
// request id and no catalog entry to enrich it with.
//
// It exists because the alternative, spelled out at eighty-nine call sites,
// was a three-line ritual of building an APIError, rendering it to stderr, and
// looking up its exit code.
func fail(code, message string, httpStatus int) {
	e := &api.APIError{Code: code, Message: message, HTTPStatus: httpStatus}
	renderCLIError(e)
	os.Exit(api.ExitCodeFor(e))
}

// failValidation is the common case: the caller asked for something the CLI
// can see is wrong without asking the server. Exit 5.
func failValidation(format string, args ...any) {
	fail("VALIDATION_ERROR", fmt.Sprintf(format, args...), 400)
}

// requireTenant exits 5 when a tenant-scoped command resolved no tenant. A
// request without a tenant header is not a narrower request; it is a request
// the server cannot place.
//
// subject names what needs the tenant, and is used as written: a whole group
// ("brands commands"), or a single command where only part of the group needs
// one ("tasks create"). The sentence is built to fit either, singular or plural.
func requireTenant(tenant *string, subject string) {
	if tenant == nil {
		failValidation("%s: a tenant is required; pass --tenant <code> or set a default", subject)
	}
}

// readJSONInput reads raw bytes from a file path or from stdin when path is "-".
func readJSONInput(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	return data, nil
}

// resolveTenant resolves the tenant from the per-command flag, CAPIGO_TENANT env,
// or the active profile's default_tenant — in that order of precedence.
// Returns nil if none is resolved.
func resolveTenant(tenantFlag string, profile *config.Profile) *string {
	return api.ResolveTenant(tenantFlag, viper.GetString("tenant"), profile.DefaultTenant)
}

// validatePCMSLimit checks that limit does not exceed 100 for PCMS list endpoints
// (which declare maximum: 100 in the OpenAPI spec). Call before making the HTTP
// request; it exits with code 5 (VALIDATION_ERROR) on violation.
// Pass limit=0 to skip the check (means "use server default").
func validatePCMSLimit(limit int) {
	const maxPCMSLimit = 100
	if limit > maxPCMSLimit {
		failValidation("--limit must be at most %d for this command (got %d)", maxPCMSLimit, limit)
	}
}
