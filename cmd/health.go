package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check API connectivity and that the API key is accepted",
	Long: `Call GET /health as a preflight. A zero exit confirms the API is reachable
and the configured API key is valid; a non-zero exit code (e.g. 2 for auth)
tells an automated caller exactly why it failed before running real work.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		client, _, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		// /health is not tenant-scoped.
		resp, err := client.Do(context.Background(), "GET", "/health", nil, nil)
		if err != nil {
			return handleErr(err)
		}

		var health api.Health
		if err := json.Unmarshal(resp.Body, &health); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, health)
		}

		status := "ok"
		if !health.OK {
			status = "not ok"
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s\t%s\n", status, health.Timestamp)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
