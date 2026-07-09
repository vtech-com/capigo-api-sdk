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
	Long:  `The tenants this API key can reach.`,
}

var tenantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants accessible to the authenticated user",
	Long: `List the tenants this API key can reach.

PURPOSE
  Discover which tenant codes are available before naming one with --tenant.
  This command takes no --tenant of its own.

INPUT
  (no flags)

OUTPUT
  -o json emits the list envelope. Each row is a tenant:

      { tenant_code, name, role, joined_at }

  tenant_code is the value --tenant expects.

  Codes discovered here are merged into known_tenants in
  ~/.capigo/config.json.

  The envelope and list footers: capigo help output

EXAMPLES
  capigo tenants list
  capigo tenants list -o json | jq -r '.data[].tenant_code'

SEE ALSO
  capigo help tenancy     how --tenant resolves, and which commands require it
  config set-default-tenant   stop passing --tenant on every command`,
	Args: cobra.NoArgs,
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

		// Merge discovered tenants back into config.
		codes := make([]string, len(envelope.Data))
		for i, t := range envelope.Data {
			codes[i] = t.TenantCode
		}
		if err := config.MergeKnownTenants(cfg, codes); err == nil {
			_ = config.Save(cfg)
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Tenant{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		// Convert api.Tenant to output.Tenant for table/quiet rendering.
		outTenants := make([]output.Tenant, len(envelope.Data))
		for i, t := range envelope.Data {
			outTenants[i] = output.Tenant{
				Code: t.TenantCode,
				Name: t.Name,
			}
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
