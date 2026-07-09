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

var variantsCmd = &cobra.Command{
	Use:   "variants",
	Short: "Query PCMS variants (read-only)",
	Long: `Product variants in the Capigo Product Catalog Management System (PCMS).

This group is read-only. Variants are written through products variants,
which upserts them onto their product; this group only reads them back.
Every command here requires a tenant, and every response names the tenant
it resolved to, in meta.

USAGE
  capigo variants <command> --tenant <code> [<args>]`,
}

// --------------------------------------------------------------------------
// variants list
// --------------------------------------------------------------------------

var (
	variantListTenant        string
	variantListBarcodePrefix string
	variantListSort          string
	variantListPage          int
	variantListLimit         int
)

var variantsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List variants by barcode prefix",
	Long: `List product variants, filtered by the leading digits of their barcode.

PURPOSE
  Find variants whose barcode begins with a given string. The usual reason is
  allocation: read the highest barcode already taken under a prefix, so the
  next one can be chosen.

USAGE
  capigo variants list --tenant <code> [--barcode-prefix <p>] [--sort <order>]
                       [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to read from. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

  --barcode-prefix <p>
      Match variants whose barcode starts with p. The special characters %
      and _ are treated literally, not as wildcards. Omit it to list all
      variants in the tenant.

        capigo variants list --tenant acme --barcode-prefix 634007

  --sort <order>
      barcode for ascending, -barcode for descending. Any other value exits
      5. Defaults to -barcode.

        # The highest barcode already used under a prefix
        capigo variants list --tenant acme --barcode-prefix 634007 \
          --sort -barcode --limit 1 | jq -r '.data[0].barcode'

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

OUTPUT
  Each row here is a flat summary — id, barcode, sku, name, product_id — not
  the full variant object; read that with variants get <id>. The rows are
  at .data[]:

      {
        "data": [
          { "id": "6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21", "barcode": "634007",
            "sku": "AT-001-S", "name": "Áo thun / S",
            "product_id": "7c1f2e88-3a4b-4c5d-9e6f-1a2b3c4d5e6f" }
        ],
        "meta": {
          "tenant": "acme",
          "tenant_source": "flag",
          "page": 1, "limit": 20, "total": 1, "has_more": false
        }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic.

  meta.tenant is the tenant this call actually ran against, and
  meta.tenant_source says whether that came from the flag, from CAPIGO_TENANT,
  or from the config file. See capigo help tenancy.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		validatePCMSLimit(variantListLimit)

		validSorts := map[string]bool{"barcode": true, "-barcode": true}
		if variantListSort != "" && !validSorts[variantListSort] {
			failValidation("--sort must be one of: barcode, -barcode")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(variantListTenant, profile)
		requireTenant(tenant, "variants")

		resp, err := client.ListVariants(ctx, tenant, variantListBarcodePrefix, variantListSort, variantListPage, variantListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.VariantRecord]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, envelope.Data, listMeta(tenant, variantListTenant, envelope.Meta))
	},
}

// --------------------------------------------------------------------------
// variants get
// --------------------------------------------------------------------------

var variantGetTenant string

var variantsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a variant by id",
	Long: `Get one variant by id.

PURPOSE
  Read a single variant in full. This command addresses it by id only. To
  find that id, use variants list --barcode-prefix, or read the variants[]
  array of products get <id>.

USAGE
  capigo variants get <id> --tenant <code>

FLAGS
  <id>
      Variant id, a UUID. Positional, required.

  --tenant <code>
      Tenant the variant belongs to. Required. Exits 4 if the variant is not
      in it.

        capigo variants get 6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21 --tenant acme

OUTPUT
  The variant is at .data — an object, where a list puts an array. This is
  the full record, unlike the flat rows variants list returns:

      {
        "data": {
          "id": "6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21", "name": "Áo thun / S",
          "sku": "AT-001-S", "barcode": "634007", "price": 120000,
          "compare_at_price": null, "currency": "VND", "weight": null,
          "dimensions": null, "option1": "S", "option2": null, "option3": null,
          "variant_type": "simple", "manufacturer_code": null,
          "legacy_code": null, "extra_data": null,
          "created_at": "2026-06-01T08:00:00Z",
          "updated_at": "2026-06-01T08:00:00Z"
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  option1..option3 hold the option values in the order the product declares
  them; products get <id> shows that order in options[]. dimensions, when
  present, is { "l": ..., "w": ..., "h": ... }.

  A single-item read carries no pagination meta; there is nothing to page.

  Exit 4 when no such variant exists in the resolved tenant — including one
  that is orphaned, soft-deleted, or belongs to another tenant. Unlike a
  product, a deleted variant is not returned and marked; it is simply
  absent. See capigo help soft-delete.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(variantGetTenant, profile)
		requireTenant(tenant, "variants")

		resp, err := client.Do(ctx, "GET", "/pcms/variants/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.ProductVariant `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, envelope.Data, itemMeta(tenant, variantGetTenant))
	},
}

func init() {
	variantsListCmd.Flags().StringVar(&variantListTenant, "tenant", "", "tenant code (required)")
	variantsListCmd.Flags().StringVar(&variantListBarcodePrefix, "barcode-prefix", "", "filter variants whose barcode starts with this value")
	variantsListCmd.Flags().StringVar(&variantListSort, "sort", "-barcode", `sort order: "barcode" (ascending) or "-barcode" (descending)`)
	variantsListCmd.Flags().IntVar(&variantListPage, "page", 0, "page number")
	variantsListCmd.Flags().IntVar(&variantListLimit, "limit", 20, "items per page (1-100, default 20)")

	variantsGetCmd.Flags().StringVar(&variantGetTenant, "tenant", "", "tenant code (required)")

	variantsCmd.AddCommand(variantsListCmd, variantsGetCmd)
	rootCmd.AddCommand(variantsCmd)
}
