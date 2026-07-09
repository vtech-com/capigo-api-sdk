package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials",
	Long: `The API key this CLI sends with every request.

CAPIGO_API_KEY, if set, overrides the key stored here on every call.
Otherwise the key comes from the active profile in ~/.capigo/config.json
(file mode 600). None of these commands take --tenant.

USAGE
  capigo auth <command> [<args>]`,
}

var loginKey string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save an API key to the active profile",
	Long: `Store an API key in the active profile.

PURPOSE
  Save the key this CLI sends on every request. This only writes to
  ~/.capigo/config.json (file mode 600); it makes no API call, so it cannot
  itself confirm the key is accepted. Run health next to confirm that.

USAGE
  capigo auth login --key <csk_...>

FLAGS
  --key <csk_...>
      API key to store. Required. Must start with csk_; if it does not,
      login exits 1 without making a request. The value is scrubbed from
      the process argument list right after startup, so it never shows up
      in ps.

        capigo auth login --key csk_live_9f2a1c...

OUTPUT
  The key itself is never echoed back, in any form:

      {
        "data": { "profile": "default",
                  "api_url": "https://platform.capigo.app/api/v1" },
        "meta": {}
      }

  Exit 1 if --key does not start with csk_, or if ~/.capigo/config.json
  cannot be read or written.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the API key from the active profile",
	Long: `Clear the stored API key from the active profile.

PURPOSE
  Remove the key from ~/.capigo/config.json. A profile with no key is not an
  error; logging out twice is harmless.

USAGE
  capigo auth logout

FLAGS
  This command takes no flags.

OUTPUT
  The profile and its resulting state:

      { "data": { "profile": "default", "status": "logged_out" }, "meta": {} }

  status is "no_credentials" instead, when the profile had no stored key to
  begin with.

  Exit 1 if ~/.capigo/config.json cannot be read or written.`,
	RunE: runLogout,
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated user",
	Long: `Show the user the stored API key belongs to.

PURPOSE
  Identify the caller by calling GET /me. That endpoint is not deployed on
  production and returns HTTP 404 there (exit 4) — a 404 here is not
  evidence the key is invalid. To confirm a key is accepted before a batch
  of work, run health instead: it is the working preflight, and it does not
  depend on /me.

USAGE
  capigo auth whoami

FLAGS
  This command takes no flags.

OUTPUT
  The user object is at .data:

      {
        "data": { "id": "3f9c2a10-6b1e-4d2f-8a77-0e4c9b2f7a31",
                  "display_name": "Trâm Nguyễn", "email": "tram@example.com",
                  "avatar_url": null },
        "meta": {}
      }

  Exit 2 when the key itself is missing, malformed or rejected. Exit 4 on
  production today, because /me is not deployed there — see PURPOSE above.
  What each exit code means: capigo help exit-codes.`,
	RunE: runWhoami,
}

func init() {
	loginCmd.Flags().StringVar(&loginKey, "key", "", "API key (must start with csk_)")
	_ = loginCmd.MarkFlagRequired("key")

	authCmd.AddCommand(loginCmd, whoamiCmd, logoutCmd)
	rootCmd.AddCommand(authCmd)
}

func runLogin(_ *cobra.Command, _ []string) error {
	if !strings.HasPrefix(loginKey, "csk_") {
		fail("INVALID_API_KEY", "invalid API key: must start with csk_", 0)
	}

	// Overwrite the flag value in os.Args so the key does not leak via `ps`.
	for i, arg := range os.Args {
		if arg == loginKey {
			os.Args[i] = ""
			break
		}
		if strings.HasPrefix(arg, "--key=") && strings.HasSuffix(arg, loginKey) {
			os.Args[i] = "--key="
			break
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return handleErr(fmt.Errorf("load config: %w", err))
	}

	profileName := cfg.ActiveProfile
	if profileName == "" {
		profileName = "default"
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.Profile)
	}

	p := cfg.Profiles[profileName]
	p.APIKey = loginKey
	cfg.Profiles[profileName] = p

	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = profileName
	}
	cfg.Version = 1

	if err := config.Save(cfg); err != nil {
		return handleErr(fmt.Errorf("save config: %w", err))
	}

	// Same precedence as buildClient, without requiring a key to already work.
	baseURL := p.APIURL
	if u := viper.GetString("api_url"); u != "" {
		baseURL = u
	}
	if apiURL != "" {
		baseURL = apiURL
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return output.Write(os.Stdout, map[string]string{
		"profile": profileName,
		"api_url": baseURL,
	}, output.Meta{})
}

func runLogout(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return handleErr(fmt.Errorf("load config: %w", err))
	}

	profileName := cfg.ActiveProfile
	if profileName == "" {
		profileName = "default"
	}

	p, ok := cfg.Profiles[profileName]
	if !ok || p.APIKey == "" {
		return output.Write(os.Stdout, map[string]string{
			"profile": profileName,
			"status":  "no_credentials",
		}, output.Meta{})
	}

	p.APIKey = ""
	cfg.Profiles[profileName] = p

	if err := config.Save(cfg); err != nil {
		return handleErr(fmt.Errorf("save config: %w", err))
	}

	return output.Write(os.Stdout, map[string]string{
		"profile": profileName,
		"status":  "logged_out",
	}, output.Meta{})
}

func runWhoami(_ *cobra.Command, _ []string) error {
	client, _, err := buildClient()
	if err != nil {
		return handleErr(err)
	}

	resp, err := client.Do(context.Background(), "GET", "/me", nil, nil)
	if err != nil {
		return handleErr(err)
	}

	// Try envelope {"data": {...}} first; fall back to bare object.
	var me api.Me
	var envelope struct {
		Data api.Me `json:"data"`
	}
	if jerr := json.Unmarshal(resp.Body, &envelope); jerr == nil && envelope.Data.ID != "" {
		me = envelope.Data
	} else if jerr := json.Unmarshal(resp.Body, &me); jerr != nil {
		return handleErr(fmt.Errorf("decode response: %w", jerr))
	}

	return output.Write(os.Stdout, me, output.Meta{})
}
