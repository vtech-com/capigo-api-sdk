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

--tenant is optional on both commands here: list and get search across every
tenant this key can reach when it is omitted.
  capigo help tenancy

USAGE
  capigo boards <command> [--tenant <code>] [<args>]`,
}

var (
	boardListTenant string
	boardListQuery  string
	boardListPage   int
	boardListLimit  int
)

var boardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List boards",
	Long: `List boards.

PURPOSE
  Find boards by name, across one tenant or across every tenant this key can
  reach.

USAGE
  capigo boards list [--tenant <code>] [-q <term>] [--page <n>]
                      [--limit <n>] [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to search. Optional — omit it to span every tenant this key can
      reach; table output then gains a Tenant column.
      See capigo help tenancy.

        capigo boards list --tenant acme

  -q, --query <term>
      Case-insensitive substring search against the board name.

        capigo boards list -q sprint

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page. Defaults to 20.

        capigo boards list --tenant acme --page 2 --limit 50

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of boards, then a summary line. With --tenant omitted, a Tenant
  column is added.

      ┌──────────┬─────────────────┬────────┬─────────────────┐
      │ ID       │ Title           │ Public │ Description     │
      ├──────────┼─────────────────┼────────┼─────────────────┤
      │ 7c1f2e88 │ Product Roadmap │ yes    │ Q1 2026 roadmap │
      │ 9ab2c744 │ Internal Ops    │ no     │                 │
      └──────────┴─────────────────┴────────┴─────────────────┘
      Tenant: acme · Total: 2 · showing 2 (page 1/1) (all rows shown)

  Ids are shortened here to fit the page; the command prints them in full.

  -o json emits the list envelope; each row is a board:

      { "id": "7c1f2e88-...", "name": "Product Roadmap",
        "description": "Q1 2026 roadmap", "is_public": true,
        "created_at": "..." }

  The lists on a board are not included here; boards get returns them.

  The envelope, meta.total and list footers: capigo help output`,
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
	Short: "Get a board by id",
	Long: `Get one board by id, with its lists.

PURPOSE
  Read a single board and the lists it contains. A board list id is what
  tasks update --list expects.

USAGE
  capigo boards get <id> [--tenant <code>] [-o table|json|quiet]

FLAGS
  <id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant to scope the lookup to. Optional; omit it to search every
      tenant this key can reach.

        capigo boards get 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table:

      ┌──────────────────────────────────────┬─────────────────┬───────┐
      │ ID                                   │ Title           │ Lists │
      ├──────────────────────────────────────┼─────────────────┼───────┤
      │ 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 │ Product Roadmap │ 3     │
      └──────────────────────────────────────┴─────────────────┴───────┘

  Lists holds the count only; the lists themselves are not in the table.

  -o json emits the bare board object, with its lists:

      { "id": "7c1f2e88-...", "name": "Product Roadmap",
        "description": "...", "is_public": true, "created_at": "...",
        "lists": [ { "id": "...", "name": "Backlog", "position": 0 } ] }

  Exit 4 when no such board is reachable.

  Output modes and the JSON contract: capigo help output`,
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
