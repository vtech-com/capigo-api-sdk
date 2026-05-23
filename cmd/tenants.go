package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var tenantsCmd = &cobra.Command{
	Use:   "tenants",
	Short: "Manage tenants",
}

var tenantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants accessible to the authenticated user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		resp, err := client.Do(context.Background(), "GET", "/tenants", nil, nil)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Tenant]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// Convert api.Tenant to output.Tenant for rendering.
		outTenants := make([]output.Tenant, len(envelope.Data))
		codes := make([]string, len(envelope.Data))
		for i, t := range envelope.Data {
			outTenants[i] = output.Tenant{
				Code: t.TenantCode,
				Name: t.Name,
			}
			codes[i] = t.TenantCode
		}

		// Merge discovered tenants back into config.
		if err := config.MergeKnownTenants(cfg, codes); err == nil {
			_ = config.Save(cfg)
		}

		if err := output.Render(os.Stdout, outputMode, outTenants, output.RenderOpts{
			ResourceKind: "tenant",
		}); err != nil {
			return handleErr(err)
		}
		return nil
	},
}

func init() {
	tenantsCmd.AddCommand(tenantsListCmd)
	rootCmd.AddCommand(tenantsCmd)
}
