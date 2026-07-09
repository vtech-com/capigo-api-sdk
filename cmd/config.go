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
	Long: `Read and write this CLI's local configuration.

Settings live in ~/.capigo/config.json (file mode 600). There is exactly one
active profile; this CLI takes no --profile flag.

Recognised keys: api_url, default_profile, default_tenant`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration key",
	Long: `Set a configuration value.

PURPOSE
  Write one setting into ~/.capigo/config.json.

INPUT
  <key>     one of api_url, default_profile, default_tenant
  <value>   the value to store

  An unrecognised key exits 5 and names the keys that are recognised.

OUTPUT
  A one-line confirmation.

  This command ignores --output: configuration is not a resource, so there is
  no JSON envelope to emit.

EXAMPLES
  capigo config set api_url https://platform.capigo.app/api/v1
  capigo config set default_tenant acme

SEE ALSO
  config get <key>            read one back
  config set-default-tenant   the same thing, with tenant validation
  capigo help tenancy         how default_tenant is used`,
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
	Long: `Read a configuration value.

PURPOSE
  Print one setting from ~/.capigo/config.json.

INPUT
  <key>   one of api_url, default_profile, default_tenant

OUTPUT
  The raw value, on one line, with no quoting and no key name.

  This command ignores --output: it prints the same bare value in every mode,
  which makes it safe to capture directly.

EXAMPLES
  capigo config get default_tenant
  TENANT=$(capigo config get default_tenant)

SEE ALSO
  config set <key> <value>   write one
  capigo help tenancy        how default_tenant is used`,
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
	Long: `Set the tenant used when --tenant is omitted.

PURPOSE
  Store default_tenant so that commands which need a tenant can find one
  without --tenant on every invocation.

INPUT
  <code>   a tenant code, as reported by tenants list

OUTPUT
  A one-line confirmation.

EXAMPLES
  capigo config set-default-tenant acme

SEE ALSO
  tenants list                  the codes this key can reach
  config unset-default-tenant   clear it again
  capigo help tenancy           the full resolution order`,
	Args: cobra.ExactArgs(1),
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
	Long: `Clear the default tenant.

PURPOSE
  Remove default_tenant, so that every command which needs a tenant must be
  given one explicitly.

INPUT
  (no arguments)

OUTPUT
  A one-line confirmation.

EXAMPLES
  capigo config unset-default-tenant

SEE ALSO
  config set-default-tenant   set it again
  capigo help tenancy         the full resolution order`,
	Args: cobra.NoArgs,
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
