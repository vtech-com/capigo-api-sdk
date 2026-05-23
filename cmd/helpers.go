package cmd

import (
	"errors"
	"fmt"
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

	return client, cfg, nil
}

// handleErr renders the error to stderr and exits with the appropriate exit code.
// Returns nil so RunE callers do not trigger cobra's double-print.
func handleErr(err error) error {
	code, message, requestID := "ERROR", err.Error(), ""
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		code = apiErr.Code
		message = apiErr.Message
		requestID = apiErr.RequestID
	}
	output.RenderError(os.Stderr, outputMode, code, message, requestID)
	os.Exit(api.ExitCodeFor(err))
	return nil
}

// resolveTenant reads --tenant / --no-tenant / CAPIGO_TENANT env / active
// profile default_tenant and delegates to api.ResolveTenant.
func resolveTenant(profile *config.Profile) (tenant *string, isGlobal bool) {
	var tenantPtr *string
	if cfgTenant != "" {
		tenantPtr = &cfgTenant
	}
	return api.ResolveTenant(tenantPtr, noTenant, viper.GetString("tenant"), profile.DefaultTenant)
}
