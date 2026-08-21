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

// --------------------------------------------------------------------------
// boards create
// --------------------------------------------------------------------------

var (
	boardCreateTenant      string
	boardCreateName        string
	boardCreateDescription string
	boardCreateIsPublic    bool
	boardCreateFromJSON    string
)

var boardsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a board",
	Long: `Create a board.

PURPOSE
  Stand up a board from the CLI. The plain flags cover the board's own fields;
  use --from-json to also create its initial lists in the same call.

USAGE
  capigo boards create --tenant <code> --name <text> [--description <text>]
                       [--is-public[=false]] [--from-json <path|->]

FLAGS
  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      Board name, at most 200 characters. Required unless --from-json is used.

  --description <text>
      Board description.

  --is-public
      Make the board public. Omit it and the server defaults to public (true);
      pass --is-public=false to make it private.

  --from-json <path|->
      A JSON object for the full request body, where - reads stdin. Use it to
      create lists in the same call. --tenant is still required and overrides
      any tenant_code in the file.

OUTPUT
  The created board, with its lists, is at .data:

      {
        "data": { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
                  "name": "Sprint 5", "description": null,
                  "is_public": true, "created_at": "2026-08-21T09:00:00Z",
                  "lists": [ { "id": "9ab2c744-...", "name": "Backlog",
                               "position": 0 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "list_count": 1, "server_time": "2026-08-21T09:00:00Z" }
      }

  Read meta.tenant: a board written to the wrong tenant looks identical to a
  success.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(boardCreateTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards create")

		var body any
		if boardCreateFromJSON != "" {
			raw, err := readJSONInput(boardCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return handleErr(fmt.Errorf("--from-json must be a JSON object: %w", err))
			}
			if m == nil {
				failValidation("--from-json must be a JSON object, not null")
			}
			m["tenant_code"] = *tenant
			body = m
		} else {
			if boardCreateName == "" {
				failValidation("--name is required (or use --from-json)")
			}
			req := api.CreateBoardRequest{TenantCode: *tenant, Name: boardCreateName}
			if boardCreateDescription != "" {
				req.Description = &boardCreateDescription
			}
			if cmd.Flags().Changed("is-public") {
				req.IsPublic = &boardCreateIsPublic
			}
			body = req
		}

		resp, err := client.CreateBoard(ctx, body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, boardCreateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// --------------------------------------------------------------------------
// boards update
// --------------------------------------------------------------------------

var (
	boardUpdateTenant      string
	boardUpdateName        string
	boardUpdateDescription string
	boardUpdateIsPublic    bool
	boardUpdateFromJSON    string
)

var boardsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a board",
	Long: `Change some fields of a board.

PURPOSE
  Rename a board, change its description or visibility. Fields you do not send
  are left unchanged. Use --from-json to also manage its lists.

USAGE
  capigo boards update <id> --tenant <code> [--name <text>]
                        [--description <text>] [--is-public[=false]]
                        [--from-json <path|->]

FLAGS
  <id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      New board name.

  --description <text>
      New board description.

  --is-public
      Set is_public (pass =false to make it private).

  --from-json <path|->
      A JSON object for the update body, where - reads stdin. Use it to manage
      lists in the same call. --tenant overrides any tenant_code in the file.

  At least one of --name, --description, --is-public, --from-json is required.

OUTPUT
  The board as it now stands is at .data, the same shape as boards get:

      {
        "data": { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
                  "name": "Sprint 5", "description": null,
                  "is_public": true, "created_at": "2026-08-21T09:00:00Z",
                  "lists": [ { "id": "9ab2c744-...", "name": "Backlog",
                               "position": 0 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "list_count": 1, "server_time": "2026-08-21T09:00:00Z" }
      }

  Read meta.tenant: a write into the wrong tenant looks exactly like a success.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(boardUpdateTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards update")

		var body any
		if boardUpdateFromJSON != "" {
			raw, err := readJSONInput(boardUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return handleErr(fmt.Errorf("--from-json must be a JSON object: %w", err))
			}
			if m == nil {
				failValidation("--from-json must be a JSON object, not null")
			}
			m["tenant_code"] = *tenant
			body = m
		} else {
			req := api.UpdateBoardRequest{TenantCode: *tenant}
			if cmd.Flags().Changed("name") {
				req.Name = &boardUpdateName
			}
			if cmd.Flags().Changed("description") {
				req.Description = &boardUpdateDescription
			}
			if cmd.Flags().Changed("is-public") {
				req.IsPublic = &boardUpdateIsPublic
			}
			if req.Name == nil && req.Description == nil && req.IsPublic == nil {
				failValidation("at least one of --name, --description, --is-public is required (or use --from-json)")
			}
			body = req
		}

		resp, err := client.UpdateBoard(ctx, args[0], body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, boardUpdateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// --------------------------------------------------------------------------
// boards lists (group)
// --------------------------------------------------------------------------

var boardListsCmd = &cobra.Command{
	Use:   "lists",
	Short: "Manage the lists inside a board",
	Long: `The lists (columns) inside a board.

A list is what a task is placed into via tasks update --list. These commands
create and update lists directly, rather than through a board write.

USAGE
  capigo boards lists <command> --tenant <code> [<args>]`,
}

var (
	boardListsCreateTenant   string
	boardListsCreateName     string
	boardListsCreateLimit    int
	boardListsCreateFromJSON string
)

var boardListsCreateCmd = &cobra.Command{
	Use:   "create <board-id>",
	Short: "Create a list in a board",
	Long: `Create a list (column) in a board.

PURPOSE
  Add a column to an existing board so tasks can be placed into it.

USAGE
  capigo boards lists create <board-id> --tenant <code> --name <text>
                             [--limit <n>] [--from-json <path|->]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      List name, at most 200 characters. Required unless --from-json is used.

  --limit <n>
      WIP limit for the list. Omit it for no limit.

  --from-json <path|->
      A JSON object for the full request body, where - reads stdin. --tenant
      overrides any tenant_code in the file.

OUTPUT
  The created list is at .data:

      {
        "data": { "id": "9ab2c744-...", "name": "Backlog", "position": 0 },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-08-21T09:00:00Z" }
      }

  Read meta.tenant: a write into the wrong tenant looks exactly like a success.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(boardListsCreateTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards lists create")

		var body any
		if boardListsCreateFromJSON != "" {
			raw, err := readJSONInput(boardListsCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return handleErr(fmt.Errorf("--from-json must be a JSON object: %w", err))
			}
			if m == nil {
				failValidation("--from-json must be a JSON object, not null")
			}
			m["tenant_code"] = *tenant
			body = m
		} else {
			if boardListsCreateName == "" {
				failValidation("--name is required (or use --from-json)")
			}
			req := api.CreateBoardListRequest{TenantCode: *tenant, Name: boardListsCreateName}
			if cmd.Flags().Changed("limit") {
				req.Limit = &boardListsCreateLimit
			}
			body = req
		}

		resp, err := client.CreateBoardList(ctx, args[0], body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, boardListsCreateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

var (
	boardListsUpdateTenant     string
	boardListsUpdateName       string
	boardListsUpdateLimit      int
	boardListsUpdateIsArchived bool
	boardListsUpdateFromJSON   string
)

var boardListsUpdateCmd = &cobra.Command{
	Use:   "update <board-id> <list-id>",
	Short: "Update a list in a board",
	Long: `Change some fields of a board list.

PURPOSE
  Rename a list, change its WIP limit, or archive/unarchive it.

USAGE
  capigo boards lists update <board-id> <list-id> --tenant <code>
                              [--name <text>] [--limit <n>]
                              [--is-archived[=false]] [--from-json <path|->]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  <list-id>
      List UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      New list name.

  --limit <n>
      New WIP limit. Use --from-json with "limit": null to clear it.

  --is-archived
      Archive (true) or unarchive (false) the list.

  --from-json <path|->
      A JSON object for the update body, where - reads stdin. --tenant overrides
      any tenant_code in the file.

  At least one of --name, --limit, --is-archived, --from-json is required.

OUTPUT
  The list as it now stands is at .data:

      {
        "data": { "id": "9ab2c744-...", "name": "Backlog", "position": 0 },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-08-21T09:00:00Z" }
      }

  Read meta.tenant: a write into the wrong tenant looks exactly like a success.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(boardListsUpdateTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards lists update")

		var body any
		if boardListsUpdateFromJSON != "" {
			raw, err := readJSONInput(boardListsUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				return handleErr(fmt.Errorf("--from-json must be a JSON object: %w", err))
			}
			if m == nil {
				failValidation("--from-json must be a JSON object, not null")
			}
			m["tenant_code"] = *tenant
			body = m
		} else {
			req := api.UpdateBoardListRequest{TenantCode: *tenant}
			if cmd.Flags().Changed("name") {
				req.Name = &boardListsUpdateName
			}
			if cmd.Flags().Changed("limit") {
				req.Limit = &boardListsUpdateLimit
			}
			if cmd.Flags().Changed("is-archived") {
				req.IsArchived = &boardListsUpdateIsArchived
			}
			if req.Name == nil && req.Limit == nil && req.IsArchived == nil {
				failValidation("at least one of --name, --limit, --is-archived is required (or use --from-json)")
			}
			body = req
		}

		resp, err := client.UpdateBoardList(ctx, args[0], args[1], body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, boardListsUpdateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	boardsListCmd.Flags().StringVar(&boardListTenant, "tenant", "", "scope to this tenant code")
	boardsListCmd.Flags().StringVarP(&boardListQuery, "query", "q", "", "case-insensitive search against board name")
	boardsListCmd.Flags().IntVar(&boardListPage, "page", 0, "page number")
	boardsListCmd.Flags().IntVar(&boardListLimit, "limit", 20, "items per page")

	boardsGetCmd.Flags().StringVar(&boardGetTenant, "tenant", "", "scope to this tenant code")

	boardsCreateCmd.Flags().StringVar(&boardCreateTenant, "tenant", "", "tenant code (required)")
	boardsCreateCmd.Flags().StringVar(&boardCreateName, "name", "", "board name (required unless --from-json is used)")
	boardsCreateCmd.Flags().StringVar(&boardCreateDescription, "description", "", "board description")
	boardsCreateCmd.Flags().BoolVar(&boardCreateIsPublic, "is-public", false, "make the board public (pass =false for private)")
	boardsCreateCmd.Flags().StringVar(&boardCreateFromJSON, "from-json", "", "path to a JSON object with the full request body (use - for stdin)")

	boardsUpdateCmd.Flags().StringVar(&boardUpdateTenant, "tenant", "", "tenant code (required)")
	boardsUpdateCmd.Flags().StringVar(&boardUpdateName, "name", "", "new board name")
	boardsUpdateCmd.Flags().StringVar(&boardUpdateDescription, "description", "", "new board description")
	boardsUpdateCmd.Flags().BoolVar(&boardUpdateIsPublic, "is-public", false, "set is_public (pass =false for private)")
	boardsUpdateCmd.Flags().StringVar(&boardUpdateFromJSON, "from-json", "", "path to a JSON object with the update body (use - for stdin)")

	boardListsCreateCmd.Flags().StringVar(&boardListsCreateTenant, "tenant", "", "tenant code (required)")
	boardListsCreateCmd.Flags().StringVar(&boardListsCreateName, "name", "", "list name (required unless --from-json is used)")
	boardListsCreateCmd.Flags().IntVar(&boardListsCreateLimit, "limit", 0, "WIP limit")
	boardListsCreateCmd.Flags().StringVar(&boardListsCreateFromJSON, "from-json", "", "path to a JSON object with the full request body (use - for stdin)")

	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateTenant, "tenant", "", "tenant code (required)")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateName, "name", "", "new list name")
	boardListsUpdateCmd.Flags().IntVar(&boardListsUpdateLimit, "limit", 0, "new WIP limit")
	boardListsUpdateCmd.Flags().BoolVar(&boardListsUpdateIsArchived, "is-archived", false, "archive (true) or unarchive (false)")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateFromJSON, "from-json", "", "path to a JSON object with the update body (use - for stdin)")

	boardListsCmd.AddCommand(boardListsCreateCmd, boardListsUpdateCmd)
	boardCmd.AddCommand(boardsListCmd, boardsGetCmd, boardsCreateCmd, boardsUpdateCmd, boardListsCmd)
	rootCmd.AddCommand(boardCmd)
}
