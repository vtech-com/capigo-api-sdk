package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

var (
	outputMode string
	apiURL     string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "capigo",
	Short: "Capigo CLI — interact with the Capigo Public API",
	Long: `capigo is the command-line interface to the Capigo Public API.

It stores your API key, attaches the tenant header, maps API errors to stable
exit codes, and formats results as table, JSON, or bare ids. Configuration lives
in ~/.capigo/config.json.

HOW HELP IS ORGANISED
  capigo --help                  which domain?
  capigo <group> --help          which command in that domain?
  capigo <group> <cmd> --help    how to call it: flags, input, output,
                                 caveats, examples, related commands

  Every command page states both the input it accepts and the output it
  returns, so a call can be built without running one first. A fact that is
  true of many commands lives in one of the help topics listed below, and is
  referenced from a command page rather than repeated on it.

GETTING STARTED
  $ capigo auth login --key csk_...   store your API key (keys begin with csk_)
  $ capigo health                     confirm the key is accepted (exit 0 = ok)
  $ capigo tenants list               see which tenants you can reach`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// Nudge toward -o json when output is being captured but table is still in
	// effect (see maybeWarnNonTTYTable). No child command defines its own
	// PersistentPreRunE, so cobra runs this for every command.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		maybeWarnNonTTYTable(cmd)
		return nil
	},
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

	rootCmd.PersistentFlags().StringVarP(&outputMode, "output", "o", "table", "output format: table, json, or quiet (unknown formats are rejected with an error)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "override API base URL (e.g. http://localhost:3999)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print HTTP request/response details (redacts Authorization header)")

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
