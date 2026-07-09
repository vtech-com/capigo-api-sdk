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
	Long: `Check that the API is reachable and the key is accepted.

PURPOSE
  The preflight to run before a batch of work. Exit 0 means the API answered
  and accepted the stored key. It is not tenant-scoped, and unlike auth
  whoami it does not depend on the /me endpoint — run this one first.

USAGE
  capigo health [-o table|json|quiet]

FLAGS
  -o, --output table|json|quiet
      json emits the object below; table and quiet both print the status
      word and server timestamp on one line. Defaults to table.
      See capigo help output.

OUTPUT
  table / quiet:

      ok	2026-07-09T10:15:00Z

  -o json emits:

      { "ok": true, "timestamp": "2026-07-09T10:15:00Z" }

  Exit 2 when the key is rejected, exit 6 when the API cannot be reached.

  capigo help exit-codes  what each non-zero exit means`,
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
