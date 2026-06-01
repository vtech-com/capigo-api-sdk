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

var unitsCmd = &cobra.Command{
	Use:   "units",
	Short: "Manage PCMS units",
}

// --------------------------------------------------------------------------
// units list
// --------------------------------------------------------------------------

var (
	unitListQuery string
	unitListPage  int
	unitListLimit int
)

var unitsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List units",
	Long: `List product units from the PCMS catalog.

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
				Message:    "units commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListUnits(ctx, tenant, unitListQuery, unitListPage, unitListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Unit]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.Unit, len(envelope.Data))
		for i, u := range envelope.Data {
			items[i] = output.Unit{
				ID:           u.ID,
				Name:         u.Name,
				Abbreviation: u.Abbreviation,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "unit",
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
	unitsListCmd.Flags().StringVarP(&unitListQuery, "query", "q", "", "name-contains filter (min 2 chars)")
	unitsListCmd.Flags().IntVar(&unitListPage, "page", 1, "page number")
	unitsListCmd.Flags().IntVar(&unitListLimit, "limit", 20, "items per page (1-100)")

	unitsCmd.AddCommand(unitsListCmd)
	rootCmd.AddCommand(unitsCmd)
}
