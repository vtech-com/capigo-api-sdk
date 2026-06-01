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

var brandsCmd = &cobra.Command{
	Use:   "brands",
	Short: "Manage PCMS brands",
}

// --------------------------------------------------------------------------
// brands list
// --------------------------------------------------------------------------

var (
	brandListQuery string
	brandListPage  int
	brandListLimit int
)

var brandsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List brands",
	Long: `List brands from the PCMS catalog.

Use --query / -q for a name-contains search (minimum 2 characters).`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if brandListQuery != "" && len(brandListQuery) < 2 {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "--query must be at least 2 characters",
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

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "brands commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListBrands(ctx, tenant, brandListQuery, brandListPage, brandListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Brand]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.Brand, len(envelope.Data))
		for i, b := range envelope.Data {
			items[i] = output.Brand{
				ID:   b.ID,
				Name: b.Name,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "brand",
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
	brandsListCmd.Flags().StringVarP(&brandListQuery, "query", "q", "", "name-contains filter (min 2 chars)")
	brandsListCmd.Flags().IntVar(&brandListPage, "page", 1, "page number")
	brandsListCmd.Flags().IntVar(&brandListLimit, "limit", 20, "items per page (1-100)")

	brandsCmd.AddCommand(brandsListCmd)
	rootCmd.AddCommand(brandsCmd)
}
