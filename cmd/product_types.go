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

var productTypesCmd = &cobra.Command{
	Use:   "product-types",
	Short: "Manage PCMS product types",
}

// --------------------------------------------------------------------------
// product-types list
// --------------------------------------------------------------------------

var (
	productTypeListQuery string
	productTypeListPage  int
	productTypeListLimit int
)

var productTypesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List product types",
	Long: `List product types from the PCMS catalog.

Use --query / -q for a name-contains search.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "product-types commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListProductTypes(ctx, tenant, productTypeListQuery, productTypeListPage, productTypeListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.ProductType]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.ProductType, len(envelope.Data))
		for i, pt := range envelope.Data {
			items[i] = output.ProductType{
				ID:   pt.ID,
				Name: pt.Name,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product_type",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			if envelope.Meta.HasMore {
				fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate.\n",
					len(envelope.Data), envelope.Meta.Total)
			}
		}

		return nil
	},
}

func init() {
	productTypesListCmd.Flags().StringVarP(&productTypeListQuery, "query", "q", "", "name-contains filter (min 2 chars)")
	productTypesListCmd.Flags().IntVar(&productTypeListPage, "page", 1, "page number")
	productTypesListCmd.Flags().IntVar(&productTypeListLimit, "limit", 20, "items per page (1-100)")

	productTypesCmd.AddCommand(productTypesListCmd)
	rootCmd.AddCommand(productTypesCmd)
}
