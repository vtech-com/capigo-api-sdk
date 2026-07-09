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
	Long: `Manage the API key this CLI sends.

The key is read from CAPIGO_API_KEY first, then from the active profile in
~/.capigo/config.json (file mode 600).`,
}

var loginKey string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save an API key to the active profile",
	Long: `Store an API key in the active profile.

PURPOSE
  Save the key this CLI will send on every request. The key is written to
  ~/.capigo/config.json with file mode 600.

INPUT
  --key <csk_...>   required. An API key; it must begin with csk_

OUTPUT
  -o json emits:

      { "profile": "default", "status": "logged_in" }

  table prints a one-line confirmation naming the profile.

  Output modes: capigo help output

EXAMPLES
  capigo auth login --key csk_...

SEE ALSO
  health                  confirm the key is accepted (exit 0 = ok)
  auth whoami             show who the key belongs to
  capigo help exit-codes  what exit 2 means`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the API key from the active profile",
	Long: `Clear the stored API key from the active profile.

PURPOSE
  Remove the key from ~/.capigo/config.json. A profile with no key is not an
  error; logging out twice is harmless.

INPUT
  (no flags)

OUTPUT
  A one-line confirmation naming the profile.

  This command ignores --output: it prints the same text in every mode.

EXAMPLES
  capigo auth logout

SEE ALSO
  auth login --key csk_...   store a key again`,
	RunE: runLogout,
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated user",
	Long: `Show the user the stored API key belongs to.

PURPOSE
  Identify the caller. To check that a key is accepted before a batch of work,
  prefer health: it is the preflight, and it does not depend on this endpoint
  being deployed.

INPUT
  (no flags)

OUTPUT
  -o json emits the bare user object:

      { id, display_name, email, avatar_url }

  quiet prints the id alone. table prints ID, Name and Email on three lines.

  Exit 2 when the key is missing, malformed or rejected.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo auth whoami
  capigo auth whoami -o quiet

SEE ALSO
  health                  the preflight to run before a batch of work
  auth login --key csk_   store a different key
  capigo help exit-codes  what exit 2 means`,
	RunE: runWhoami,
}

func init() {
	loginCmd.Flags().StringVar(&loginKey, "key", "", "API key (must start with csk_)")
	_ = loginCmd.MarkFlagRequired("key")

	authCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
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
