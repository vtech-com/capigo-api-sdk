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

var boardCmd = &cobra.Command{
	Use:   "boards",
	Short: "Manage boards",
	Long: `Boards and their lists in Capigo Mission.

Reads may span tenants: omit --tenant to see every tenant this key can reach.
  capigo help tenancy`,
}

var (
	boardListTenant string
	boardListQuery  string
	boardListPage   int
	boardListLimit  int
)

var boardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List boards (supports --query for name search)",
	Long: `List boards.

PURPOSE
  Find boards by name, across one tenant or across every tenant this key can
  reach.

INPUT
  --tenant <code>     optional; omit it to span every accessible tenant
  -q, --query <term>  case-insensitive search against the board name
  --page <n>          page number
  --limit <n>         items per page (default 20)

OUTPUT
  -o json emits the list envelope. Each row is a board:

      { id, name, description, is_public, created_at }

  The lists on a board are not included here; boards get returns them.

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo boards list -q sprint
  capigo boards list --tenant acme -o json | jq -r '.data[].id'

SEE ALSO
  boards get <id>       one board, with its lists
  tasks list --board-id the tasks on a board
  capigo help tenancy   when --tenant may be omitted`,
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

		params := url.Values{}
		if boardListQuery != "" {
			params.Set("q", boardListQuery)
		}
		if boardListPage > 0 {
			params.Set("page", strconv.Itoa(boardListPage))
		}
		if boardListLimit > 0 {
			params.Set("limit", strconv.Itoa(boardListLimit))
		}

		path := "/mission/boards"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Board]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Board{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Board, len(envelope.Data))
		for i, b := range envelope.Data {
			desc := ""
			if b.Description != nil {
				desc = *b.Description
			}
			items[i] = output.Board{
				ID:          b.ID,
				Title:       b.Name,
				IsPublic:    b.IsPublic,
				Description: desc,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "board",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, boardListTenant),
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

var (
	boardGetTenant string
)

var boardsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a board by ID",
	Long: `Get one board by UUID, with its lists.

PURPOSE
  Read a single board and the lists it contains. A board list id is what
  tasks update --list expects.

INPUT
  <id>              board UUID (positional, required)
  --tenant <code>   optional; scopes the lookup

OUTPUT
  -o json emits the bare board object, with its lists:

      { id, name, description, is_public, created_at,
        lists: [ { id, name, position } ] }

  Exit 4 when no such board is reachable.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo boards get <uuid>
  capigo boards get <uuid> -o json | jq '.lists[] | {id, name}'

SEE ALSO
  boards list             find a board by name
  tasks update <id>       move a task onto one of these lists
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

		tenant := resolveTenant(boardGetTenant, profile)

		resp, err := client.Do(ctx, "GET", "/mission/boards/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.BoardDetail `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, output.BoardDetail{
			ID:        envelope.Data.ID,
			Title:     envelope.Data.Name,
			ListCount: len(envelope.Data.Lists),
		}, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "board_detail",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

func init() {
	boardsListCmd.Flags().StringVar(&boardListTenant, "tenant", "", "scope to this tenant code")
	boardsListCmd.Flags().StringVarP(&boardListQuery, "query", "q", "", "case-insensitive search against board name")
	boardsListCmd.Flags().IntVar(&boardListPage, "page", 0, "page number")
	boardsListCmd.Flags().IntVar(&boardListLimit, "limit", 20, "items per page")

	boardsGetCmd.Flags().StringVar(&boardGetTenant, "tenant", "", "scope to this tenant code")

	boardCmd.AddCommand(boardsListCmd, boardsGetCmd)
	rootCmd.AddCommand(boardCmd)
}
