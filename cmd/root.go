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
  capigo <group> <cmd> --help    purpose, usage, flags, output`,
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
