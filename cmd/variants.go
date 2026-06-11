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
	Short: "Query PCMS variants",
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
	Long: `List product variants from the PCMS catalog.

Use --barcode-prefix to filter variants whose barcode starts with the given string.
Use --sort to control sort order: "barcode" (ascending) or "-barcode" (descending).
The primary use case is finding the highest barcode in a prefix namespace; use
--limit 1 --sort -barcode to get the top result.`,
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
				Shown:   len(envelope.Data),
				Page:    envelope.Meta.Page,
				Limit:   envelope.Meta.Limit,
				Total:   envelope.Meta.Total,
				HasMore: envelope.Meta.HasMore,
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
	Long: `Get a single product variant by UUID from PCMS. Tenant is required.

Returns the full variant shape (id, name, sku, barcode, price, compare_at_price,
currency, weight, dimensions, option1/2/3, variant_type, created_at, updated_at).
Orphaned, soft-deleted, and cross-tenant variants return 404.

Response: { "data": { ... } } — full PublicProductVariantResponse shape.`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

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
