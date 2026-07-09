package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
  The rows are at .data[], exactly as the API sends them. They carry less
  than variants get returns — no price, no options, no timestamps — but
  everything this endpoint has:

      {
        "data": [
          { "id": "6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21", "barcode": "634007",
            "sku": "AT-001-S", "name": "Áo thun / S",
            "product_id": "7c1f2e88-3a4b-4c5d-9e6f-1a2b3c4d5e6f",
            "manufacturer_code": null, "legacy_code": null,
            "extra_data": null }
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
		requireTenant(tenant, "variants commands")

		resp, err := client.ListVariants(ctx, tenant, variantListBarcodePrefix, variantListSort, variantListPage, variantListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, variantListTenant, envelope.Meta))
	},
}

// --------------------------------------------------------------------------
// variants get
// --------------------------------------------------------------------------

var (
	variantGetTenant string
	variantGetSKU    string
)

var variantsGetCmd = &cobra.Command{
	Use:   "get [<id>]",
	Short: "Get a variant by id or by sku",
	Long: `Get one variant, addressed by id or by sku.

PURPOSE
  Read a single variant in full. A variant has two addresses — its id, and its
  sku, which is unique within a tenant — and both return the same record. Use
  --sku when the sku is what you have; it is the key a person quotes.

  To find an id, use variants list --barcode-prefix, or read the variants[]
  array of products get <id>.

USAGE
  capigo variants get (<id> | --sku <sku>) --tenant <code>

FLAGS
  <id>
      Variant id, a UUID. Positional. Give this or --sku, never both: a bare
      argument is never guessed at, so a sku shaped like a UUID cannot be sent
      to the wrong address. Giving neither, or both, exits 5.

  --sku <sku>
      Address the variant by its sku instead. Tenant-scoped: the same sku may
      exist in another tenant, and that one is not found here.

        capigo variants get --sku AT-001-S --tenant acme

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

  Exit 4 when no such variant exists in the resolved tenant — by either
  address, and including one that is orphaned, soft-deleted, or belongs to
  another tenant. Unlike a product, a deleted variant is not returned and
  marked; it is simply absent, and an unknown sku and a deleted one give the
  same answer. See capigo help soft-delete.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		// One address or the other, never both and never neither. Guessing which
		// a bare argument is — a UUID or a sku that happens to look like one —
		// would put a request on the wrong endpoint and say nothing about it.
		switch {
		case len(args) == 1 && variantGetSKU != "":
			failValidation("give an id or --sku, not both")
		case len(args) == 0 && variantGetSKU == "":
			failValidation("a variant address is required: an id, or --sku <sku>")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(variantGetTenant, profile)
		requireTenant(tenant, "variants commands")

		// A sku is a value someone typed. Escaped, so one containing a slash
		// addresses a variant rather than a different endpoint.
		var path string
		if variantGetSKU != "" {
			path = "/pcms/variants/sku/" + url.PathEscape(variantGetSKU)
		} else {
			path = "/pcms/variants/" + url.PathEscape(args[0])
		}

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, variantGetTenant))
	},
}

func init() {
	variantsListCmd.Flags().StringVar(&variantListTenant, "tenant", "", "tenant code (required)")
	variantsListCmd.Flags().StringVar(&variantListBarcodePrefix, "barcode-prefix", "", "filter variants whose barcode starts with this value")
	variantsListCmd.Flags().StringVar(&variantListSort, "sort", "-barcode", `sort order: "barcode" (ascending) or "-barcode" (descending)`)
	variantsListCmd.Flags().IntVar(&variantListPage, "page", 0, "page number")
	variantsListCmd.Flags().IntVar(&variantListLimit, "limit", 20, "items per page (1-100, default 20)")

	variantsGetCmd.Flags().StringVar(&variantGetTenant, "tenant", "", "tenant code (required)")
	variantsGetCmd.Flags().StringVar(&variantGetSKU, "sku", "", "address the variant by sku instead of by id")

	variantsCmd.AddCommand(variantsListCmd, variantsGetCmd)
	rootCmd.AddCommand(variantsCmd)
}
