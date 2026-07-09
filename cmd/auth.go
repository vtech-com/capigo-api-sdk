package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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
  capigo auth login --key <csk_...> [-o table|json]

FLAGS
  --key <csk_...>
      API key to store. Required. Must start with csk_; if it does not,
      login exits 1 without making a request. The value is scrubbed from
      the process argument list right after startup, so it never shows up
      in ps.

        capigo auth login --key csk_live_9f2a1c...

  -o, --output table|json
      table and quiet both print the one-line confirmation below; json
      prints the object instead. See capigo help output.

OUTPUT
  table / quiet:

      Logged in as profile "default"

  -o json emits:

      { "profile": "default", "status": "logged_in" }

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
  This command takes no flags. --output is ignored: it prints the same
  text in every mode.

OUTPUT
  A one-line confirmation naming the profile:

      Logged out of profile "default"

  Or, if the profile already had no key:

      Profile "default" has no stored credentials

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
  capigo auth whoami [-o table|json|quiet]

FLAGS
  -o, --output table|json|quiet
      table prints ID, Name and Email on three lines; quiet prints the id
      alone; json emits the bare user object. Defaults to table.
      See capigo help output.

OUTPUT
  table:

      ID:    3f9c2a10-6b1e-4d2f-8a77-0e4c9b2f7a31
      Name:  Trâm Nguyễn
      Email: tram@example.com

  -o json emits the bare object, with no envelope:

      { "id": "3f9c2a10-6b1e-4d2f-8a77-0e4c9b2f7a31",
        "display_name": "Trâm Nguyễn", "email": "tram@example.com",
        "avatar_url": null }

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

func runLogin(cmd *cobra.Command, _ []string) error {
	if !strings.HasPrefix(loginKey, "csk_") {
		err := fmt.Errorf("invalid API key: must start with csk_")
		output.RenderError(os.Stderr, outputMode, "INVALID_API_KEY", err.Error(), "")
		os.Exit(api.ExitCodeFor(err))
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
		output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
		os.Exit(api.ExitCodeFor(err))
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
		output.RenderError(os.Stderr, outputMode, "CONFIG_SAVE_ERROR", err.Error(), "")
		os.Exit(api.ExitCodeFor(err))
	}

	if outputMode == "json" {
		return output.WriteJSONObject(cmd.OutOrStdout(), map[string]string{
			"profile": profileName,
			"status":  "logged_in",
		})
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Logged in as profile %q\n", profileName)
	return err
}

func runLogout(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
		os.Exit(api.ExitCodeFor(err))
	}

	profileName := cfg.ActiveProfile
	if profileName == "" {
		profileName = "default"
	}

	if cfg.Profiles == nil {
		_, err = fmt.Fprintf(os.Stdout, "Profile %q has no stored credentials\n", profileName)
		return err
	}

	p, ok := cfg.Profiles[profileName]
	if !ok || p.APIKey == "" {
		_, err = fmt.Fprintf(os.Stdout, "Profile %q has no stored credentials\n", profileName)
		return err
	}

	p.APIKey = ""
	cfg.Profiles[profileName] = p

	if err := config.Save(cfg); err != nil {
		output.RenderError(os.Stderr, outputMode, "CONFIG_SAVE_ERROR", err.Error(), "")
		os.Exit(api.ExitCodeFor(err))
	}

	_, err = fmt.Fprintf(os.Stdout, "Logged out of profile %q\n", profileName)
	return err
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

	if outputMode == "json" {
		return output.WriteJSONObject(os.Stdout, me)
	}

	if outputMode == "quiet" {
		_, err = fmt.Fprintln(os.Stdout, me.ID)
		return err
	}

	// table / default
	_, err = fmt.Fprintf(os.Stdout, "ID:    %s\nName:  %s\nEmail: %s\n", me.ID, me.DisplayName, me.Email)
	return err
}
