package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// configValidationErr exits with code 5 (validation) via the standard error path.
func configValidationErr(msg string) {
	e := &api.APIError{Code: "VALIDATION_ERROR", Message: msg, HTTPStatus: 400}
	output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
	os.Exit(api.ExitCodeFor(e))
}

// configNotFoundErr exits with code 4 (not found) via the standard error path.
func configNotFoundErr(msg string) {
	e := &api.APIError{Code: "NOT_FOUND", Message: msg, HTTPStatus: 404}
	output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
	os.Exit(api.ExitCodeFor(e))
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage capigo CLI configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration key",
	Long: `Set a configuration key in the active profile.

Supported keys:
  api_url          - Override the API base URL
  default_profile  - Set the active profile name`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		cfg, err := config.Load()
		if err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
			os.Exit(api.ExitCodeFor(err))
		}

		switch key {
		case "api_url":
			profileName := cfg.ActiveProfile
			if profileName == "" {
				profileName = "default"
			}
			p, ok := cfg.Profiles[profileName]
			if !ok {
				configNotFoundErr(fmt.Sprintf("profile %q not found", profileName))
			}
			p.APIURL = value
			cfg.Profiles[profileName] = p

		case "default_profile":
			if err := config.SetProfile(cfg, value); err != nil {
				configValidationErr(err.Error())
			}

		default:
			configValidationErr(fmt.Sprintf("unknown key %q; supported: api_url, default_profile", key))
		}

		if err := config.Save(cfg); err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_SAVE_ERROR", err.Error(), "")
			os.Exit(1)
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Print the value of a configuration key from the active profile.

Supported keys:
  api_url          - The API base URL
  default_profile  - The active profile name
  default_tenant   - The default tenant code for the active profile`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		cfg, err := config.Load()
		if err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
			os.Exit(api.ExitCodeFor(err))
		}

		switch key {
		case "default_profile":
			name := cfg.ActiveProfile
			if name == "" {
				name = "default"
			}
			fmt.Println(name)

		case "api_url":
			p, err := config.ActiveProfile(cfg)
			if err != nil {
				configNotFoundErr(err.Error())
			}
			fmt.Println(p.APIURL)

		case "default_tenant":
			p, err := config.ActiveProfile(cfg)
			if err != nil {
				configNotFoundErr(err.Error())
			}
			fmt.Println(p.DefaultTenant)

		default:
			configValidationErr(fmt.Sprintf("unknown key %q; supported: api_url, default_profile, default_tenant", key))
		}
		return nil
	},
}

var configSetDefaultTenantCmd = &cobra.Command{
	Use:   "set-default-tenant <code>",
	Short: "Set the default tenant for the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]

		cfg, err := config.Load()
		if err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
			os.Exit(api.ExitCodeFor(err))
		}

		profileName := cfg.ActiveProfile
		if profileName == "" {
			profileName = "default"
		}
		p, ok := cfg.Profiles[profileName]
		if !ok {
			configNotFoundErr(fmt.Sprintf("profile %q not found", profileName))
		}
		p.DefaultTenant = code
		cfg.Profiles[profileName] = p

		if err := config.Save(cfg); err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_SAVE_ERROR", err.Error(), "")
			os.Exit(1)
		}
		return nil
	},
}

var configUnsetDefaultTenantCmd = &cobra.Command{
	Use:   "unset-default-tenant",
	Short: "Clear the default tenant for the active profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_LOAD_ERROR", err.Error(), "")
			os.Exit(api.ExitCodeFor(err))
		}

		profileName := cfg.ActiveProfile
		if profileName == "" {
			profileName = "default"
		}
		p, ok := cfg.Profiles[profileName]
		if !ok {
			configNotFoundErr(fmt.Sprintf("profile %q not found", profileName))
		}
		p.DefaultTenant = ""
		cfg.Profiles[profileName] = p

		if err := config.Save(cfg); err != nil {
			output.RenderError(os.Stderr, outputMode, "CONFIG_SAVE_ERROR", err.Error(), "")
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetDefaultTenantCmd)
	configCmd.AddCommand(configUnsetDefaultTenantCmd)
	rootCmd.AddCommand(configCmd)
}
