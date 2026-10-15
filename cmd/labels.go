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

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "List task labels",
	Long: `Task labels.

PURPOSE
  A label is a name and a colour a task can carry. Labels have no create
  command: naming one on tasks create --label or tasks update --label creates
  it. tasks list --label filters by them. This group lists what exists.

USAGE
  capigo labels list [--tenant <code>]`,
}

var labelListTenant string

var labelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every label in a tenant",
	Long: `List every label in a tenant.

PURPOSE
  See which labels exist, including ones no task carries, to reuse a name
  instead of coining a near-duplicate.

USAGE
  capigo labels list [--tenant <code>]

FLAGS
  --tenant <code>
      Tenant to list. Required when the key reaches several tenants; without
      it the API answers 400 INVALID_TENANT_SCOPE (exit 5).

        capigo labels list --tenant acme

OUTPUT
  Labels, ordered by name, are at .data. There is no pagination.

      {
        "data": [ { "name": "urgent", "color": "rose" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  color is one of sky, orange, green, rose, teal, amber, violet, indigo,
  fuchsia, lime.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(labelListTenant, activeProfileOrEmpty(cfg))

		resp, err := client.Do(context.Background(), "GET", "/mission/labels", nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), itemMeta(tenant, labelListTenant, envelope.Meta))
	},
}

func init() {
	labelsListCmd.Flags().StringVar(&labelListTenant, "tenant", "", "tenant code (required if the key reaches several tenants)")
	labelsCmd.AddCommand(labelsListCmd)
	rootCmd.AddCommand(labelsCmd)
}
