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

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Manage PCMS categories",
}

// --------------------------------------------------------------------------
// categories list
// --------------------------------------------------------------------------

var (
	categoryListQuery string
	categoryListPage  int
	categoryListLimit int
)

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List categories",
	Long: `List categories from the PCMS catalog.

Use --query / -q for a name-contains search (minimum 2 characters).`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if categoryListQuery != "" && len(categoryListQuery) < 2 {
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
				Message:    "categories commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListCategories(ctx, tenant, categoryListQuery, categoryListPage, categoryListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Category]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.Category, len(envelope.Data))
		for i, c := range envelope.Data {
			parentID := ""
			if c.ParentID != nil {
				parentID = *c.ParentID
			}
			items[i] = output.Category{
				ID:       c.ID,
				Name:     c.Name,
				ParentID: parentID,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "category",
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
	categoriesListCmd.Flags().StringVarP(&categoryListQuery, "query", "q", "", "name-contains filter (min 2 chars)")
	categoriesListCmd.Flags().IntVar(&categoryListPage, "page", 1, "page number")
	categoriesListCmd.Flags().IntVar(&categoryListLimit, "limit", 20, "items per page (1-100)")

	categoriesCmd.AddCommand(categoriesListCmd)
	rootCmd.AddCommand(categoriesCmd)
}
