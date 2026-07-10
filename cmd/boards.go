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
                      [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to search. Optional — omit it to span every tenant this key can
      reach; meta.tenant is then empty.
      See capigo help tenancy.

        capigo boards list --tenant acme

  -q, --query <term>
      Case-insensitive substring search against the board name.

        capigo boards list -q sprint

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page, 1 to 50. Defaults to 20. Above 50 the server rejects the
      call with exit 5 — mission endpoints cap at 50, unlike the PCMS lists.

        capigo boards list --tenant acme --page 2 --limit 50

OUTPUT
  The boards are at .data[]:

      {
        "data": [
          { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
            "name": "Product Roadmap", "description": "Q1 2026 roadmap",
            "is_public": true, "created_at": "2026-01-05T09:00:00Z" }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic.

  With --tenant omitted, meta.tenant and meta.tenant_source are absent: the
  results span every tenant this key can reach, and a board does not name its
  own tenant either. Pass --tenant when you need to know where a board lives.

  The lists on a board are not included here; boards get returns them.`,
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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, boardListTenant, envelope.Meta))
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
  capigo boards get <id> [--tenant <code>]

FLAGS
  <id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant to scope the lookup to. Optional; omit it to search every
      tenant this key can reach.

        capigo boards get 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 --tenant acme

OUTPUT
  The board, with its lists, is at .data:

      {
        "data": { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
                  "name": "Product Roadmap", "description": "Q1 2026 roadmap",
                  "is_public": true, "created_at": "2026-01-05T09:00:00Z",
                  "lists": [ { "id": "9ab2c744-...", "name": "Backlog",
                              "position": 0 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag", "list_count": 5 }
      }

  With --tenant omitted, meta.tenant and meta.tenant_source are absent.

  Exit 4 when no such board is reachable.`,
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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, boardGetTenant, envelope.Meta))
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
