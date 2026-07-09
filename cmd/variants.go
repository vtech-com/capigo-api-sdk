package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

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
Every command here requires a tenant.

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
                       [--page <n>] [--limit <n>] [-o table|json|quiet]

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
          --sort -barcode --limit 1 -o json | jq -r '.data[0].barcode'

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of variants, then a summary line.

      ┌──────────┬─────────┬──────────┬──────────────┬──────────┐
      │ ID       │ Barcode │ SKU      │ Name         │ ProductID│
      ├──────────┼─────────┼──────────┼──────────────┼──────────┤
      │ 6f1c9a3d │ 634007  │ AT-001-S │ Áo thun / S  │ 7c1f2e88 │
      └──────────┴─────────┴──────────┴──────────────┴──────────┘
      Tenant: acme · Total: 1 · showing 1 (page 1/1)

  Ids are shortened here to fit the page; the command prints them in full.

  -o json emits the list envelope. Each row here is a flat summary — id,
  barcode, sku, name, product_id — not the full variant object; read that
  with variants get <id>:

      {
        "data": [
          { "id": "6f1c9a3d-...", "barcode": "634007", "sku": "AT-001-S",
            "name": "Áo thun / S", "product_id": "7c1f2e88-..." }
        ],
        "meta": { "page": 1, "limit": 20, "total": 1, "has_more": false }
      }

  The envelope, meta.total and list footers: capigo help output`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		validatePCMSLimit(variantListLimit)

		validSorts := map[string]bool{"barcode": true, "-barcode": true}
		if variantListSort != "" && !validSorts[variantListSort] {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "--sort must be one of: barcode, -barcode",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
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
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "variants commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListVariants(ctx, tenant, variantListBarcodePrefix, variantListSort, variantListPage, variantListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.VariantRecord]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.VariantRecord{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.VariantRecord, len(envelope.Data))
		for i, v := range envelope.Data {
			barcode := ""
			if v.Barcode != nil {
				barcode = *v.Barcode
			}
			sku := ""
			if v.SKU != nil {
				sku = *v.SKU
			}
			items[i] = output.VariantRecord{
				ID:        v.ID,
				Barcode:   barcode,
				SKU:       sku,
				Name:      v.Name,
				ProductID: v.ProductID,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "variant_record",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, variantListTenant),
				Shown:      len(envelope.Data),
				Page:       envelope.Meta.Page,
				Limit:      envelope.Meta.Limit,
				Total:      envelope.Meta.Total,
				HasMore:    envelope.Meta.HasMore,
			})
		}

		return nil
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
  capigo variants get <id> --tenant <code> [-o table|json|quiet]

FLAGS
  <id>
      Variant id, a UUID. Positional, required.

  --tenant <code>
      Tenant the variant belongs to. Required. Exits 4 if the variant is not
      in it.

        capigo variants get 6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21 --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table. Ids are shortened here to fit the page; the command
  prints them in full.

      ┌──────────┬─────────────┬──────────┬─────────┬────────┬────────┐
      │ ID       │ Name        │ SKU      │ Barcode │ Price  │ Type   │
      ├──────────┼─────────────┼──────────┼─────────┼────────┼────────┤
      │ 6f1c9a3d │ Áo thun / S │ AT-001-S │ 634007  │ 120000 │ simple │
      └──────────┴─────────────┴──────────┴─────────┴────────┴────────┘

  -o json emits the bare object. A get is not a list, so there is no envelope
  and no .data to reach for:

      { "id": "6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21", "name": "Áo thun / S",
        "sku": "AT-001-S", "barcode": "634007", "price": 120000,
        "compare_at_price": null, "currency": "VND", "weight": null,
        "dimensions": null, "option1": "S", "option2": null, "option3": null,
        "variant_type": "simple", "manufacturer_code": null,
        "legacy_code": null, "extra_data": null,
        "created_at": "...", "updated_at": "..." }

  option1..option3 hold the option values in the order the product declares
  them; products get <id> shows that order in options[].

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
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "variants commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.Do(ctx, "GET", "/pcms/variants/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		// The endpoint returns the full PublicProductVariantResponse shape;
		// decode into api.ProductVariant so JSON mode emits every field
		// (price, dimensions, options, variant_type, timestamps, etc.).
		var envelope struct {
			Data api.ProductVariant `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		barcode := ""
		if envelope.Data.Barcode != nil {
			barcode = *envelope.Data.Barcode
		}
		sku := ""
		if envelope.Data.SKU != nil {
			sku = *envelope.Data.SKU
		}
		price := ""
		if envelope.Data.Price != nil {
			price = strconv.FormatFloat(*envelope.Data.Price, 'f', -1, 64)
		}
		if err := output.Render(os.Stdout, outputMode, output.Variant{
			ID:          envelope.Data.ID,
			Name:        envelope.Data.Name,
			SKU:         sku,
			Barcode:     barcode,
			Price:       price,
			VariantType: envelope.Data.VariantType,
		}, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "variant",
		}); err != nil {
			return handleErr(err)
		}

		return nil
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
