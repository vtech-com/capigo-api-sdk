package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var (
	cfgProfile string
	cfgTenant  string
	noTenant   bool
	outputMode string
	apiURL     string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "capigo",
	Short: "Capigo CLI — interact with the Capigo Public API",
	Long: `capigo is a command-line interface for the Capigo Public API.

It manages credentials, tenants, tasks, boards, and more.
Configuration is stored in ~/.capigo/config.json.

API keys must start with csk_. Use 'capigo auth login --key <key>' to authenticate.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Wrap cobra arg-validation errors so they render through the output
		// formatter (respecting --output json) and map to exit code 5.
		wrapped := &api.APIError{
			Code:       "VALIDATION_ERROR",
			Message:    err.Error(),
			HTTPStatus: 400,
		}
		output.RenderError(os.Stderr, outputMode, wrapped.Code, wrapped.Message, "")
		os.Exit(api.ExitCodeFor(wrapped))
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgProfile, "profile", "", "config profile to use (default: active_profile from config file)")
	rootCmd.PersistentFlags().StringVar(&cfgTenant, "tenant", "", "scope request to this tenant code")
	rootCmd.PersistentFlags().BoolVar(&noTenant, "no-tenant", false, "force global mode (override default_tenant from config)")
	rootCmd.PersistentFlags().StringVarP(&outputMode, "output", "o", "table", "output format: table, json, or quiet (unknown formats are rejected with an error)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "override API base URL (e.g. http://localhost:3999)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print HTTP request/response details (redacts Authorization header)")

	_ = viper.BindEnv("api_key", "CAPIGO_API_KEY")
	_ = viper.BindEnv("tenant", "CAPIGO_TENANT")
	_ = viper.BindEnv("profile", "CAPIGO_PROFILE")
	_ = viper.BindEnv("api_url", "CAPIGO_API_URL")

	_ = viper.BindPFlag("tenant", rootCmd.PersistentFlags().Lookup("tenant"))
	_ = viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
	_ = viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
}

// initConfig is the cobra.OnInitialize hook. It loads ~/.capigo/config.json
// and merges env-var overrides via viper. Full config store logic lives in
// internal/config — this is intentionally a stub until that package exists.
func initConfig() {
	// TODO(Wave 1): replace this stub with internal/config.Load(cfgProfile)
	// and call viper.SetConfigFile(store.Path()) once internal/config is built.
	viper.SetEnvPrefix("")
	viper.AutomaticEnv()
}
