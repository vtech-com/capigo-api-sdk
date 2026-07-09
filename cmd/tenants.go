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
  This command takes no --tenant of its own.

USAGE
  capigo tenants list

FLAGS
  This command takes no flags.

OUTPUT
  The tenants are at .data[]; tenant_code is the value --tenant expects:

      {
        "data": [
          { "tenant_code": "acme", "name": "Acme Co.", "role": "owner",
            "joined_at": "2026-01-14T09:00:00Z" },
          { "tenant_code": "demo", "name": "Demo Tenant", "role": "member",
            "joined_at": "2026-02-03T09:00:00Z" }
        ],
        "meta": {}
      }

  This endpoint does not paginate and sends no pagination meta, so meta is
  empty here. Count .data[] — it is the whole list. Every other list command
  reports meta.total, and on those you must read it rather than count.

  capigo help tenancy  how --tenant resolves, and which commands require it`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
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

		// No listMeta here: this endpoint neither paginates nor sends meta, and
		// there is no tenant to name — a tenants list is how you learn the tenants.
		return output.Write(os.Stdout, envelope.Data, output.Meta{})
	},
}

func init() {
	tenantsCmd.AddCommand(tenantsListCmd)
	rootCmd.AddCommand(tenantsCmd)
}
