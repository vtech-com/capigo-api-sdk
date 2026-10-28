package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

var (
	apiURL  string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "capigo",
	Short: "Capigo CLI — interact with the Capigo Public API",
	Long: `capigo is the command-line interface to the Capigo Public API.

It stores your API key, attaches the tenant header, maps API errors to stable
exit codes, and prints JSON. Configuration lives in ~/.capigo/config.json.

Every command emits the same envelope on stdout — { "data": …, "meta": … } —
whether it read one record, read a page of them, or wrote one. There is no
output flag, and no second shape to branch on.

USAGE
  capigo [--api-url <url>] [-v] <group> <command> [<args>]

  capigo <group> --help          the commands in that group
  capigo <group> <cmd> --help    purpose, usage, flags, output

GETTING STARTED
  Every command that reaches the API needs a key — which is all of them but
  version, config, and auth login/logout. Store one once, then confirm the API
  accepts it:

      $ capigo auth login --key csk_...
      $ capigo health

  Until a key is configured the CLI does not contact the server at all: it
  fails locally with exit 2 and AUTH_MISSING_HEADER, so nothing you read here
  is an answer from the API. CAPIGO_API_KEY, if set, overrides the stored key
  and needs no login.

  When CAPIGO_API_KEY is set, the environment supplies the key: run
  capigo health and skip login.

KEY FROM THE ENVIRONMENT
  CAPIGO_API_KEY being present at all — even empty — means the environment
  running this CLI manages the key. It is then the only key source: an empty
  value exits 2 with AUTH_EMPTY_ENV_KEY instead of using the stored key, auth
  login exits 1 with KEY_MANAGED_BY_ENV, and tenants list caches nothing.

  CAPIGO_NO_CONFIG=1 (exactly 1) takes ~/.capigo/config.json out entirely: it
  is neither read nor written, no HOME is needed, and every auth and config
  command that would touch it exits 1 with CONFIG_DISABLED.

  In either case every error object carries key_source: "env" or "none", and
  API commands do not read the config file. The base URL comes only from
  --api-url, then CAPIGO_API_URL, then the default host; the tenant only from
  --tenant, then CAPIGO_TENANT. A stored api_url or default_tenant is not
  applied, and neither is an unprefixed API_URL or TENANT. CAPIGO_API_URL set
  but empty exits 1 with EMPTY_ENV_API_URL and sends nothing; CAPIGO_TENANT
  set but empty means no tenant.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	// Runs once, after every command file's init() has registered its
	// subcommands (main.go calls Execute() only after package init completes).
	// See unknown_command.go for why this is needed: without it, cobra silently
	// swallows unmatched subcommand-like args below the root (exit 0, no
	// suggestion, no error) instead of raising the "unknown command" error both
	// Layer 1 (cobra's own suggestions) and Layer 2 (curated redirects) need.
	enableUnknownSubcommandErrors(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		// Wrap cobra arg-validation errors so they render through the output
		// formatter (respecting --output json) and map to exit code 5.
		wrapped := &api.APIError{
			Code:       "VALIDATION_ERROR",
			Message:    err.Error(),
			HTTPStatus: 400,
		}
		renderCLIError(wrapped)
		os.Exit(api.ExitCodeFor(wrapped))
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "override the API base URL")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print the HTTP request and response (key redacted)")

	_ = viper.BindEnv("api_key", "CAPIGO_API_KEY")
	_ = viper.BindEnv("tenant", "CAPIGO_TENANT")
	_ = viper.BindEnv("api_url", "CAPIGO_API_URL")

	_ = viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
}

// initConfig is the cobra.OnInitialize hook. It loads ~/.capigo/config.json
// and merges env-var overrides via viper. Full config store logic lives in
// internal/config — this is intentionally a stub until that package exists.
func initConfig() {
	// TODO(Wave 1): replace this stub with internal/config.Load()
	// and call viper.SetConfigFile(store.Path()) once internal/config is built.
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
}
