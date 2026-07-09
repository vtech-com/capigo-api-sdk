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
	Long: `The tenants this API key can reach.

Codes discovered by tenants list are cached in known_tenants in
~/.capigo/config.json, for reference — nothing here validates a --tenant
value against that cache.

USAGE
  capigo tenants <command> [<args>]`,
}

var tenantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants accessible to the authenticated user",
	Long: `List the tenants this API key can reach.

PURPOSE
  Discover which tenant codes are available before naming one with --tenant.
  This command takes no --tenant of its own, and unlike other list commands
  its table has no summary footer.

USAGE
  capigo tenants list [-o table|json|quiet]

FLAGS
  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare tenant codes. Defaults to
      table. See capigo help output.

OUTPUT
  A table of tenants:

      ┌──────┬─────────────┐
      │ Code │ Name        │
      ├──────┼─────────────┤
      │ acme │ Acme Co.    │
      │ demo │ Demo Tenant │
      └──────┴─────────────┘

  -o json emits the list envelope; each row is a tenant:

      {
        "data": [
          { "tenant_code": "acme", "name": "Acme Co.", "role": "owner",
            "joined_at": "2026-01-14T09:00:00Z" }
        ],
        "meta": { "page": 1, "limit": 20, "total": 2, "has_more": false }
      }

  tenant_code is the value --tenant expects. quiet prints tenant_code, one
  per line.

  capigo help tenancy  how --tenant resolves, and which commands require it`,
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
