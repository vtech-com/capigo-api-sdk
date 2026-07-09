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
	Long: `Query product variants in PCMS.

Variants are written through products variants, which upserts them onto their
product. Reading them lives here.

Every variants command requires a tenant.
  capigo help tenancy`,
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

INPUT
  --tenant <code>          required
  --barcode-prefix <p>     match variants whose barcode starts with p
  --sort <order>           barcode (ascending) or -barcode (descending).
                           Default -barcode.
  --page <n>               page number
  --limit <n>              items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Each row is a variant object; its shape is
  documented on variants get.

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  # The highest barcode already used under a prefix
  capigo variants list --tenant acme --barcode-prefix 634007 \
    --sort -barcode --limit 1 -o json | jq -r '.data[0].barcode'

SEE ALSO
  variants get <id>       one variant in full
  products variants       create or update variants
  capigo help output      output modes and the JSON contract
  capigo help tenancy     how --tenant resolves`,
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
	Short: "Get a variant by ID",
	Long: `Get one variant by UUID.

PURPOSE
  Read a single variant in full. This command addresses a variant by UUID only.
  To find that UUID, use variants list --barcode-prefix, or read the variants[]
  array of products get <id>.

INPUT
  <id>              variant UUID (positional, required)
  --tenant <code>   required

OUTPUT
  -o json emits the bare variant object:

      { id, name, sku, barcode, price, compare_at_price, currency, weight,
        dimensions { l, w, h }, option1, option2, option3, variant_type,
        manufacturer_code, legacy_code, extra_data, created_at, updated_at }

  option1..option3 hold the option values in the order the product declares
  them; products get <id> shows that order in options[].

  Exit 4 when the variant is orphaned, soft-deleted, or in another tenant.
  Unlike a product, a deleted variant is not returned and marked — it is
  simply absent. See capigo help soft-delete

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo variants get 6f1c-... --tenant acme

SEE ALSO
  variants list           find variants by barcode prefix
  products get <id>       the product and all of its variants at once
  products variants       create or update variants
  capigo help exit-codes  what exit 4 means`,
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
