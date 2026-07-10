package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// configNotFoundErr exits with code 4 (not found) via the standard error path.
func configNotFoundErr(msg string) {
	fail("NOT_FOUND", msg, 404)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage capigo CLI configuration",
	Long: `Local settings for this CLI.

Every command here reads and writes ~/.capigo/config.json (file mode 600)
directly; none of them calls the API. There is exactly one active profile;
this CLI takes no --profile flag. Settings apply to that active profile.

USAGE
  capigo config <command> [<args>]`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration key",
	Long: `Set a configuration value.

PURPOSE
  Write one setting into ~/.capigo/config.json. default_tenant is not
  settable here — use config set-default-tenant instead.

USAGE
  capigo config set <key> <value>

FLAGS
  <key>
      One of api_url, default_profile. Positional, required. An unrecognised
      key — including default_tenant — exits 5 and names the keys that are
      recognised.

  <value>
      The value to store. Positional, required.

        capigo config set api_url https://platform.capigo.app/api/v1
        capigo config set default_profile staging

OUTPUT
  Nothing on success: exit 0 and silence. Confirm with config get <key>.

  Exit 5 for an unrecognised key, or when default_profile names a profile
  that does not exist. Exit 4 when api_url is set before any profile exists —
  run auth login first.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		cfg, err := config.Load()
		if err != nil {
			fail("CONFIG_LOAD_ERROR", err.Error(), 0)
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
				failValidation("%s", err.Error())
			}

		default:
			failValidation("unknown key %q; supported: api_url, default_profile", key)
		}

		if err := config.Save(cfg); err != nil {
			fail("CONFIG_SAVE_ERROR", err.Error(), 0)
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Read a configuration value.

PURPOSE
  Print one setting from ~/.capigo/config.json. To change it, use config set
  or config set-default-tenant.

USAGE
  capigo config get <key>

FLAGS
  <key>
      One of api_url, default_profile, default_tenant. Positional, required.
      An unrecognised key exits 5 and names the keys that are recognised.

        capigo config get default_tenant
        capigo config get api_url

OUTPUT
  The key and its value are at .data:

      { "data": { "key": "default_tenant", "value": "acme" }, "meta": {} }

  value is "" when the key exists but is unset (e.g. no default_tenant has
  been stored yet) — never null.

  Exit 4 if the active profile itself is not in the config file yet.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		cfg, err := config.Load()
		if err != nil {
			fail("CONFIG_LOAD_ERROR", err.Error(), 0)
		}

		var value string
		switch key {
		case "default_profile":
			value = cfg.ActiveProfile
			if value == "" {
				value = "default"
			}

		case "api_url":
			p, err := config.ActiveProfile(cfg)
			if err != nil {
				configNotFoundErr(err.Error())
			}
			value = p.APIURL

		case "default_tenant":
			p, err := config.ActiveProfile(cfg)
			if err != nil {
				configNotFoundErr(err.Error())
			}
			value = p.DefaultTenant

		default:
			failValidation("unknown key %q; supported: api_url, default_profile, default_tenant", key)
		}

		return output.Write(os.Stdout, map[string]string{"key": key, "value": value}, output.Meta{})
	},
}

var configSetDefaultTenantCmd = &cobra.Command{
	Use:   "set-default-tenant <code>",
	Short: "Set the default tenant for the active profile",
	Long: `Set the tenant used when --tenant is omitted.

PURPOSE
  Store default_tenant as the last fallback in tenant resolution: --tenant,
  then CAPIGO_TENANT, then this value. See capigo help tenancy for the full
  order.

USAGE
  capigo config set-default-tenant <code>

FLAGS
  <code>
      A tenant code. Positional, required. Stored as given: this command does
      not call the API, so it does not check that the tenant exists. Confirm
      the code first with tenants list.

        capigo config set-default-tenant acme

OUTPUT
  Nothing on success: exit 0 and silence. Confirm with config get
  default_tenant.

  Exit 4 if the active profile itself is not in the config file yet.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]

		cfg, err := config.Load()
		if err != nil {
			fail("CONFIG_LOAD_ERROR", err.Error(), 0)
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
			fail("CONFIG_SAVE_ERROR", err.Error(), 0)
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
  given one explicitly via --tenant or CAPIGO_TENANT. See capigo help
  tenancy for the full resolution order.

USAGE
  capigo config unset-default-tenant

FLAGS
  (none)

OUTPUT
  Nothing on success: exit 0 and silence. Confirm with config get
  default_tenant, which then reports value: "".

  Exit 4 if the active profile itself is not in the config file yet.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fail("CONFIG_LOAD_ERROR", err.Error(), 0)
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
			fail("CONFIG_SAVE_ERROR", err.Error(), 0)
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configSetDefaultTenantCmd)
	configCmd.AddCommand(configUnsetDefaultTenantCmd)
	rootCmd.AddCommand(configCmd)
}
