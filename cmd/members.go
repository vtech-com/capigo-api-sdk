package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var memberCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage members",
}

var (
	memberListTenant string
	memberListQuery  string
	memberListPage   int
	memberListLimit  int
)

var membersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace members",
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

		tenant := resolveTenant(memberListTenant, profile)

		params := url.Values{}
		if memberListQuery != "" {
			params.Set("q", memberListQuery)
		}
		if memberListPage > 0 {
			params.Set("page", strconv.Itoa(memberListPage))
		}
		if memberListLimit > 0 {
			params.Set("limit", strconv.Itoa(memberListLimit))
		}

		path := "/members"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Member]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Member{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Member, len(envelope.Data))
		for i, m := range envelope.Data {
			items[i] = output.Member{
				ID:    m.ID,
				Name:  m.DisplayName,
				Email: m.Email,
				Role:  m.Role,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "member",
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
	membersListCmd.Flags().StringVar(&memberListTenant, "tenant", "", "scope to this tenant code")
	membersListCmd.Flags().StringVarP(&memberListQuery, "query", "q", "", "filter by member name or email")
	membersListCmd.Flags().IntVar(&memberListPage, "page", 0, "page number (0 = server default)")
	membersListCmd.Flags().IntVar(&memberListLimit, "limit", 0, "items per page (0 = server default)")
	memberCmd.AddCommand(membersListCmd)
	rootCmd.AddCommand(memberCmd)
}
