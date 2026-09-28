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

--tenant is optional on list and get only, which search across every tenant this
key can reach when it is omitted. Every other command here — create, update,
delete, lists and members — needs it, because it addresses one workspace's board.
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

  The lists on a board are not included here; boards get returns them.

  Only public boards are listed. A board whose is_public is false is absent
  from every page, so a board you cannot find here may exist and be private
  rather than not exist. See boards create --is-public.`,
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

  Exit 4 when no such board is reachable. "Board not found" also covers a
  board that exists but has is_public false: the API serves public boards
  only, and does not distinguish the two cases. See boards create --is-public.`,
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

      Passing =false is one-way through this CLI. Checked against the API on
      2026-08-27: with is_public false, boards get, boards update, and both
      boards lists writes all answer 404 "Board not found" with exit 4. The
      visibility check runs before the update is applied, so --is-public
      cannot switch it back on, and no endpoint deletes a board. The board
      does keep existing, and the web app still shows it to its members — the
      board becomes unreachable for this CLI, not for people. Leave the flag
      off unless the user asked for a board no agent should touch again.

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

      Passing =false is one-way: this same command answers 404 afterwards,
      because the visibility check runs before the update. See boards create
      --is-public for what stays reachable and what does not.

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
// boards delete
// --------------------------------------------------------------------------

var boardDeleteTenant string

var boardsDeleteCmd = &cobra.Command{
	Use:   "delete <board-id>",
	Short: "Delete a board",
	Long: `Retire a board.

PURPOSE
  Take a board off the workspace for good. It leaves every read, and a repeat
  answers exit 4.

  This is not a way to retire the work on the board. Its lists and the tasks in
  them stay live rows: the lists become unreachable, because a list is addressed
  inside its board, but the tasks stay readable through tasks get and tasks
  list. To close the work instead, archive each list — boards lists update
  --is-archived archives the tasks in that list.

  Nothing brings a deleted board back. There is no un-delete here, and no
  restore anywhere in the API.

USAGE
  capigo boards delete <board-id> --tenant <code>

FLAGS
  <board-id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

        capigo boards delete 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          --tenant acme

OUTPUT
  The id the board was retired under is at .data:

      {
        "data": { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-09-28T09:00:00Z" }
      }

  The row itself is not returned: every read answers 404 for the board from here
  on, so exit 0 and .data.id are the whole answer.

  Confirming it is a read: after this, boards list no longer shows it and
  boards get answers 404.

  Exit 3 when the caller may not edit the board (board owner or tenant owner
  only). Exit 4 when the board is unknown, private, or already retired — a
  private board cannot be deleted through the API at all.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardDeleteTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards delete")

		req := api.DeleteBoardRequest{TenantCode: *tenant}

		resp, err := client.DeleteBoard(ctx, args[0], req, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, boardDeleteTenant, envelope.Meta)
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
	boardListsCreateWIPLimit int
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
                             [--wip-limit <n>] [--from-json <path|->]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      List name, at most 200 characters. Required unless --from-json is used.

  --wip-limit <n>
      Work-in-progress cap for the list — how many tasks the list is meant to
      hold at once. Omit it for no cap. This is not pagination, and not the
      --limit of boards list.

      Write-only: no board or list read returns limit, so the value sent
      cannot be read back or confirmed later. Report it as requested, never
      as verified.

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

  That is the whole record the API returns: limit is not echoed back, so the
  response confirms the list, not the cap.

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
			if cmd.Flags().Changed("wip-limit") {
				req.Limit = &boardListsCreateWIPLimit
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
	boardListsUpdateWIPLimit   int
	boardListsUpdateIsArchived bool
	boardListsUpdateAfterList  string
	boardListsUpdateBeforeList string
	boardListsUpdateFromJSON   string
)

var boardListsUpdateCmd = &cobra.Command{
	Use:   "update <board-id> <list-id>",
	Short: "Update or reorder a list in a board",
	Long: `Change some fields of a board list, or move it among the board's lists.

PURPOSE
  Rename a list, change its WIP limit, archive/unarchive it — or move it to sit
  directly behind or directly in front of another list on the same board.

USAGE
  capigo boards lists update <board-id> <list-id> --tenant <code>
                              [--name <text>] [--wip-limit <n>]
                              [--is-archived[=false]]
                              [--after-list-id <uuid> | --before-list-id <uuid>]
                              [--from-json <path|->]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  <list-id>
      List UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --name <text>
      New list name.

  --wip-limit <n>
      New work-in-progress cap. This is not pagination, and not the --limit of
      boards list. Use --from-json with "limit": null to clear it.

      Write-only: no read returns limit, so a change to it cannot be confirmed
      afterwards, and a call that changed nothing looks like one that did.

  --is-archived
      Archive (true) or unarchive (false) the list.

      Archiving hides the list from every read: boards get then omits it from
      .lists and drops meta.list_count, and no read exposes archived lists.
      This command still reaches it, so keep the list id — unarchiving needs
      an id nothing else will give you back. Tasks in the list are neither
      moved nor deleted; they keep pointing at a list no read reports.

  --after-list-id <uuid>
      Move the list to sit directly behind this list on the same board.

        capigo boards lists update 7c1f2e88-... 9ab2c744-... --tenant acme \
          --after-list-id 4d9a1c07-...

  --before-list-id <uuid>
      Move the list to sit directly in front of this list on the same board.

  --from-json <path|->
      A JSON object for the update body, where - reads stdin. --tenant
      overrides any tenant_code in the file.

      The file is the whole body, so it cannot be combined with --name,
      --wip-limit, --is-archived, --after-list-id or --before-list-id: that
      is exit 5, where the flag beside it used to be dropped silently.

  One anchor only, and never together with --name, --wip-limit or
  --is-archived: the API takes a reorder or an update, not both, and refusing
  the pair here says so before a request that would be rejected anyway.

  The anchor must be another list on the same board that still appears in
  boards get. An anchor on another board, or an archived one, exits 4 — a list
  no read reports is not a place to sit behind.

  At least one of --name, --wip-limit, --is-archived, --after-list-id or
  --before-list-id is required, unless --from-json supplies the body.

OUTPUT
  The list as it now stands is at .data, and after a move its position is the
  one the server computed:

      {
        "data": { "id": "9ab2c744-...", "name": "Backlog", "position": 1500 },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-09-28T09:00:00Z" }
      }

  A list already where it was asked to go is a success: exit 0, and its
  position may be unchanged. Nothing here reports "moved" — a reorder is a
  write like any other, and .data.position is what the server now holds.

  The API returns neither limit nor is_archived, so a --wip-limit or
  --is-archived call answers with a record identical to the one before it.
  Exit 0 says the request was accepted; the response cannot say what changed.

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
			// A file is the whole body: a flag alongside it would be dropped
			// silently, and the caller would believe it applied.
			for _, flag := range []string{"name", "wip-limit", "is-archived", "after-list-id", "before-list-id"} {
				if cmd.Flags().Changed(flag) {
					failValidation("--from-json carries the whole body: drop --%s, or drop --from-json", flag)
				}
			}
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
			// One intent per request. A reorder names one anchor and no field —
			// the API refuses the pair, and refusing it here says so before a
			// request that would be rejected anyway.
			hasAfter := cmd.Flags().Changed("after-list-id")
			hasBefore := cmd.Flags().Changed("before-list-id")
			hasField := cmd.Flags().Changed("name") ||
				cmd.Flags().Changed("wip-limit") ||
				cmd.Flags().Changed("is-archived")

			if hasAfter && hasBefore {
				failValidation("--after-list-id and --before-list-id are two different moves: pass exactly one")
			}
			if (hasAfter || hasBefore) && hasField {
				failValidation("a reorder is one request: --after-list-id/--before-list-id cannot be combined with --name, --wip-limit or --is-archived")
			}

			req := api.UpdateBoardListRequest{TenantCode: *tenant}
			if cmd.Flags().Changed("name") {
				req.Name = &boardListsUpdateName
			}
			if cmd.Flags().Changed("wip-limit") {
				req.Limit = &boardListsUpdateWIPLimit
			}
			if cmd.Flags().Changed("is-archived") {
				req.IsArchived = &boardListsUpdateIsArchived
			}
			if hasAfter {
				req.AfterListID = &boardListsUpdateAfterList
			}
			if hasBefore {
				req.BeforeListID = &boardListsUpdateBeforeList
			}
			if req.Name == nil && req.Limit == nil && req.IsArchived == nil &&
				req.AfterListID == nil && req.BeforeListID == nil {
				failValidation("at least one of --name, --wip-limit, --is-archived, --after-list-id, --before-list-id is required (or use --from-json)")
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

var boardListsDeleteTenant string

var boardListsDeleteCmd = &cobra.Command{
	Use:   "delete <board-id> <list-id>",
	Short: "Delete a list from a board",
	Long: `Retire a list from a board.

PURPOSE
  Take a list off a board for good. This is not archiving: boards lists update
  --is-archived hides a list and archives the tasks in it, while this retires
  the row and leaves those tasks exactly where they are — still active, still
  filed under a list no read reports again.

  Nothing brings a deleted list back. There is no un-delete here, and no
  restore anywhere in the API.

USAGE
  capigo boards lists delete <board-id> <list-id> --tenant <code>

FLAGS
  <board-id>
      Board UUID. Positional, required.

  <list-id>
      List UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

        capigo boards lists delete 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          9ab2c744-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The id the list was retired under is at .data:

      {
        "data": { "id": "9ab2c744-2b6e-4f83-a5d1-8c07e2f419bb" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-09-28T09:00:00Z" }
      }

  The row itself is not returned: every read answers 404 for the list from here
  on, so exit 0 and .data.id are the whole answer. A repeat is exit 4, and so is
  a list that belongs to another board — the address names both.

  Confirming it is a read: after this, boards get no longer lists it and its
  meta.list_count has dropped.

  The address reaches an archived list too, exactly as boards lists update
  does: a caller holding an id may retire what they hid. The list's tasks are
  left alone either way — this is not the archive, which retires them.

  Exit 3 when the caller may not edit the board (board owner or tenant owner
  only). Exit 4 when the board is unknown, private or in a tenant this key
  cannot see, or when the list is unknown, belongs to another board, or has
  already been retired.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardListsDeleteTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards lists delete")

		req := api.DeleteBoardListRequest{TenantCode: *tenant}

		resp, err := client.DeleteBoardList(ctx, args[0], args[1], req, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, boardListsDeleteTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// --------------------------------------------------------------------------
// boards members (group)
// --------------------------------------------------------------------------

var boardMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage who belongs to a board",
	Long: `The people on a board, and their board role.

A board's members are a subset of the workspace's members: being in the
workspace does not put anyone on the board. These commands read that subset and
change it, and they are the only way to do so without the board settings screen.

USAGE
  capigo boards members <command> [--tenant <code>] [<args>]

--tenant is optional on list only; add, update and remove address one workspace's
board and need it.`,
}

var (
	boardMembersListTenant string
	boardMembersListPage   int
	boardMembersListLimit  int
)

var boardMembersListCmd = &cobra.Command{
	Use:   "list <board-id>",
	Short: "List the members of a board",
	Long: `List the members of a board and their board roles.

PURPOSE
  Read who is on a board — most often to turn a name into the user id that
  boards members add and boards members remove take.

USAGE
  capigo boards members list <board-id> [--tenant <code>] [--page <n>]
                                    [--limit <n>]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant to look the board up in. Optional — omit it and the board is
      searched across every tenant this key can reach, as boards get does.
      See capigo help tenancy.

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page, 1 to 50. Defaults to 20.

        capigo boards members list 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          --tenant acme --limit 50

OUTPUT
  The members are at .data[]:

      {
        "data": [
          { "user_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
            "email": "tram@acme.vn", "display_name": "Tram Nguyen",
            "role": "member", "avatar_url": null }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false }
      }

  user_id is the id every other boards members command takes; it is not the
  workspace member id that members list reports, and the two are not
  interchangeable. role is the board-level role, owner or member. Read
  meta.total rather than counting .data[]: a page never holds more than
  --limit.

  Requires membership of the board itself (any role) or ownership of the
  tenant: a caller who is in the workspace but not on the board is refused
  with exit 3, and a board in a tenant this key cannot see is exit 4.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardMembersListTenant, activeProfileOrEmpty(cfg))

		resp, err := client.ListBoardMembers(ctx, args[0], tenant, boardMembersListPage, boardMembersListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := listMeta(tenant, boardMembersListTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawList(envelope.Data), meta)
	},
}

var (
	boardMembersAddTenant  string
	boardMembersAddUserIDs []string
	boardMembersAddRole    string
)

var boardMembersAddCmd = &cobra.Command{
	Use:   "add <board-id>",
	Short: "Add members to a board",
	Long: `Add one or more workspace members to a board.

PURPOSE
  Put people on a board in a single request — the batch the API exposes, rather
  than a loop of single adds. A member the board already has is skipped instead
  of refused, so running the same call twice leaves the board as it was.

USAGE
  capigo boards members add <board-id> --tenant <code> --user-id <uuid>
                                   [--user-id <uuid> ...] [--role <role>]

FLAGS
  <board-id>
      Board UUID. Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --user-id <uuid>
      Auth user id of a workspace member to add. Repeatable, and at least one
      is required; up to 50 per call. This is the user_id that boards members
      list reports, not the workspace member id that members list reports.

        capigo boards members add 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          --tenant acme --user-id 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
          --user-id 2b7e4f83-a5d1-4c07-e2f4-19bb4d9a1c07

  --role <role>
      Board role for every member in this call: owner or member. Defaults to
      member. The API takes one role per request, so adding a member and
      promoting another is two calls; the role of an existing member is
      changed with boards members update.

OUTPUT
  One entry per requested member is at .data.results[], in the order the
  --user-id flags were given:

      {
        "data": {
          "results": [
            { "user_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
              "status": "added" },
            { "user_id": "2b7e4f83-a5d1-4c07-e2f4-19bb4d9a1c07",
              "status": "skipped", "reason": "already_member" }
          ],
          "added_count": 1, "skipped_count": 1
        },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-09-28T09:00:00Z" }
      }

  added means this call wrote the membership; skipped means the board already
  had it. Read added_count for what changed: a repeat of the same call reports
  0 with every entry skipped, which is also what makes the call safe to retry.

  Exit 3 when the caller may not edit the board (board owner or tenant owner
  only), exit 4 when the board is missing or a user is not an active member of
  the tenant, exit 5 when a --user-id is not a UUID or --role is neither owner
  nor member.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardMembersAddTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards members add")

		if len(boardMembersAddUserIDs) == 0 {
			failValidation("--user-id is required at least once")
		}

		req := api.AddBoardMembersRequest{
			TenantCode: *tenant,
			UserIDs:    boardMembersAddUserIDs,
		}
		if boardMembersAddRole != "" {
			if boardMembersAddRole != "owner" && boardMembersAddRole != "member" {
				failValidation("--role must be owner or member (got %q)", boardMembersAddRole)
			}
			req.Role = &boardMembersAddRole
		}

		resp, err := client.AddBoardMembers(ctx, args[0], req, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, boardMembersAddTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

var (
	boardMembersUpdateTenant string
	boardMembersUpdateRole   string
)

var boardMembersUpdateCmd = &cobra.Command{
	Use:   "update <board-id> <user-id>",
	Short: "Change a member's board role",
	Long: `Change one member's role on a board.

PURPOSE
  Promote a member to owner, who can then edit the board and manage its
  members, or demote an owner back to member. The API changes the role and
  nothing else.

USAGE
  capigo boards members update <board-id> <user-id> --tenant <code>
                                      --role <owner|member>

FLAGS
  <board-id>
      Board UUID. Positional, required.

  <user-id>
      Auth user id of the member — the user_id boards members list reports.
      Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

  --role <owner|member>
      The new board role. Required: there is no partial update here, and the
      API refuses a request without one.

        capigo boards members update 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme --role owner

OUTPUT
  The updated member is at .data:

      {
        "data": { "user_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "email": "tram@acme.vn", "display_name": "Tram Nguyen",
                  "role": "owner", "avatar_url": null },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-09-28T09:00:00Z" }
      }

  Read .data.role: it is what the server now holds, and a demotion that was
  refused changed nothing.

  Exit 3 when the caller may not edit the board, exit 4 when the board or the
  member is not there, exit 8 when the change would leave the board with no
  active owner — a board always keeps one.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardMembersUpdateTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards members update")

		if boardMembersUpdateRole != "owner" && boardMembersUpdateRole != "member" {
			failValidation("--role must be owner or member (got %q)", boardMembersUpdateRole)
		}

		req := api.UpdateBoardMemberRequest{
			TenantCode: *tenant,
			Role:       boardMembersUpdateRole,
		}

		resp, err := client.UpdateBoardMember(ctx, args[0], args[1], req, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, boardMembersUpdateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

var boardMembersRemoveTenant string

var boardMembersRemoveCmd = &cobra.Command{
	Use:   "remove <board-id> <user-id>",
	Short: "Remove a member from a board",
	Long: `Remove one member from a board.

PURPOSE
  Take someone off a board without touching the workspace: the person stays a
  workspace member and keeps every other board they are on. This removes them
  from this board only.

USAGE
  capigo boards members remove <board-id> <user-id> --tenant <code>

FLAGS
  <board-id>
      Board UUID. Positional, required.

  <user-id>
      Auth user id of the member — the user_id boards members list reports.
      Positional, required.

  --tenant <code>
      Tenant the board belongs to. Required.

        capigo boards members remove 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 \
          4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  Nothing is printed on success: the API answers 204 with no body, and this
  CLI never invents a payload the server did not send. Exit 0 is the whole
  confirmation, and the member is gone from boards members list.

  Exit 3 when the caller may not edit the board, exit 4 when the board or the
  member is not there, exit 8 when the member is the board's last active owner
  — a board always keeps one.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(boardMembersRemoveTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "boards members remove")

		req := api.RemoveBoardMemberRequest{TenantCode: *tenant}

		if _, err := client.RemoveBoardMember(ctx, args[0], args[1], req, tenant); err != nil {
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

	boardsDeleteCmd.Flags().StringVar(&boardDeleteTenant, "tenant", "", "tenant code (required)")

	boardListsCreateCmd.Flags().StringVar(&boardListsCreateTenant, "tenant", "", "tenant code (required)")
	boardListsCreateCmd.Flags().StringVar(&boardListsCreateName, "name", "", "list name (required unless --from-json is used)")
	boardListsCreateCmd.Flags().IntVar(&boardListsCreateWIPLimit, "wip-limit", 0, "work-in-progress cap for the list (not pagination)")
	boardListsCreateCmd.Flags().StringVar(&boardListsCreateFromJSON, "from-json", "", "path to a JSON object with the full request body (use - for stdin)")

	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateTenant, "tenant", "", "tenant code (required)")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateName, "name", "", "new list name")
	boardListsUpdateCmd.Flags().IntVar(&boardListsUpdateWIPLimit, "wip-limit", 0, "new work-in-progress cap (not pagination)")
	boardListsUpdateCmd.Flags().BoolVar(&boardListsUpdateIsArchived, "is-archived", false, "archive (true) or unarchive (false)")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateAfterList, "after-list-id", "", "move the list directly behind this list")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateBeforeList, "before-list-id", "", "move the list directly in front of this list")
	boardListsUpdateCmd.Flags().StringVar(&boardListsUpdateFromJSON, "from-json", "", "path to a JSON object with the update body (use - for stdin)")

	boardListsDeleteCmd.Flags().StringVar(&boardListsDeleteTenant, "tenant", "", "tenant code (required)")

	boardListsCmd.AddCommand(boardListsCreateCmd, boardListsUpdateCmd, boardListsDeleteCmd)
	boardMembersListCmd.Flags().StringVar(&boardMembersListTenant, "tenant", "", "tenant code (required)")
	boardMembersListCmd.Flags().IntVar(&boardMembersListPage, "page", 0, "page number")
	boardMembersListCmd.Flags().IntVar(&boardMembersListLimit, "limit", 20, "items per page")

	boardMembersAddCmd.Flags().StringVar(&boardMembersAddTenant, "tenant", "", "tenant code (required)")
	boardMembersAddCmd.Flags().StringArrayVar(&boardMembersAddUserIDs, "user-id", nil, "auth user id to add (repeatable)")
	boardMembersAddCmd.Flags().StringVar(&boardMembersAddRole, "role", "", "board role for the whole request: owner or member")

	boardMembersUpdateCmd.Flags().StringVar(&boardMembersUpdateTenant, "tenant", "", "tenant code (required)")
	boardMembersUpdateCmd.Flags().StringVar(&boardMembersUpdateRole, "role", "", "new board role: owner or member (required)")

	boardMembersRemoveCmd.Flags().StringVar(&boardMembersRemoveTenant, "tenant", "", "tenant code (required)")

	boardMembersCmd.AddCommand(boardMembersListCmd, boardMembersAddCmd, boardMembersUpdateCmd, boardMembersRemoveCmd)

	boardCmd.AddCommand(boardsListCmd, boardsGetCmd, boardsCreateCmd, boardsUpdateCmd, boardsDeleteCmd, boardListsCmd, boardMembersCmd)
	rootCmd.AddCommand(boardCmd)
}
