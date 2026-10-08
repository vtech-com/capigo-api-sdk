package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// structureDelete describes one catalog-structure delete command. The four
// resources share a contract (soft delete, refused while a live item uses the
// resource, a repeat answers 404), so one builder keeps them identical.
type structureDelete struct {
	group    string // "brands"
	noun     string // "brand"
	apiPath  string // "/pcms/brands"
	freed    string // "slug" or "name": what a new item may take at once
	notFound string // the 404 error code
	inUse    string // the 409 refusal, in words
}

func newStructureDeleteCmd(d structureDelete) *cobra.Command {
	var tenantFlag string
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a " + d.noun,
		Long: fmt.Sprintf(`Soft-delete a %[1]s, as the PCMS editor does.

PURPOSE
  The %[1]s leaves every read, is never purged, and its %[2]s is free for a new %[1]s at
  once. Only the tenant owner or a member with the catalog admin permission may call it
  (exit 3 otherwise).

  %[3]s Nothing is deleted when it is refused (exit 8). Deleted products do not
  count.

  A repeated delete is exit 4 (%[4]s): the first one succeeded and the %[1]s is gone,
  so a script that lost the answer and retries should treat exit 4 as done. An id that
  is unknown, malformed, already deleted or another tenant's is the same exit 4.

USAGE
  capigo %[5]s delete <id> --tenant <code>

FLAGS
  <id>
      %[1]s UUID. Positional, required.

  --tenant <code>
      Tenant the %[1]s belongs to. Required.

        capigo %[5]s delete 4d9a1c07-... --tenant acme

OUTPUT
  The answer names the item by id, because every read answers exit 4 once the delete
  has run:

      { "data": { "id": "4d9a1c07-…" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`, d.noun, d.freed, d.inUse, d.notFound, d.group),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			client, cfg, err := buildClient()
			if err != nil {
				return handleErr(err)
			}
			profile := activeProfileOrEmpty(cfg)
			tenant := resolveTenant(tenantFlag, profile)
			requireTenant(tenant, d.group+" delete")

			resp, err := client.Do(ctx, "DELETE", d.apiPath+"/"+url.PathEscape(args[0]), nil, tenant)
			if err != nil {
				return handleErr(err)
			}
			var envelope api.RawEnvelope
			if err := json.Unmarshal(resp.Body, &envelope); err != nil {
				return handleErr(fmt.Errorf("decode response: %w", err))
			}
			meta := itemMeta(tenant, tenantFlag, envelope.Meta)
			meta.ServerTime = resp.ServerTime
			return output.Write(os.Stdout, rawItem(envelope.Data), meta)
		},
	}
	cmd.Flags().StringVar(&tenantFlag, "tenant", "", "tenant the "+d.noun+" belongs to (required)")
	return cmd
}

var (
	brandsDeleteCmd = newStructureDeleteCmd(structureDelete{
		group: "brands", noun: "brand", apiPath: "/pcms/brands", freed: "slug", notFound: "E9401",
		inUse: "A brand a live product uses, whatever the product's status, is refused (E9435).",
	})
	categoriesDeleteCmd = newStructureDeleteCmd(structureDelete{
		group: "categories", noun: "category", apiPath: "/pcms/categories", freed: "slug", notFound: "E9405",
		inUse: "A category with a live child category (E9439) or a live product of any status filed under it (E9440) is refused.",
	})
	productTypesDeleteCmd = newStructureDeleteCmd(structureDelete{
		group: "product-types", noun: "product type", apiPath: "/pcms/product-types", freed: "name", notFound: "E9409",
		inUse: "A product type a live product uses is refused (E9437).",
	})
	unitsDeleteCmd = newStructureDeleteCmd(structureDelete{
		group: "units", noun: "unit", apiPath: "/pcms/units", freed: "name", notFound: "E9413",
		inUse: "A unit a live product uses is refused (E9442).",
	})
)
