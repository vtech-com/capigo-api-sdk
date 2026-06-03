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

var boardCmd = &cobra.Command{
	Use:   "boards",
	Short: "Manage boards",
}

var boardListTenant string

var boardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List boards",
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

		tenant := resolveTenant(boardListTenant, profile)

		resp, err := client.Do(ctx, "GET", "/mission/boards", nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Board]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.Board, len(envelope.Data))
		for i, b := range envelope.Data {
			items[i] = output.Board{
				ID:    b.ID,
				Title: b.Name,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "board",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" && envelope.Meta.HasMore {
			fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate.\n",
				len(envelope.Data), envelope.Meta.Total)
		}

		return nil
	},
}

func init() {
	boardsListCmd.Flags().StringVar(&boardListTenant, "tenant", "", "scope to this tenant code")
	boardCmd.AddCommand(boardsListCmd)
	rootCmd.AddCommand(boardCmd)
}
