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

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
	Long: `Tasks are the work items in Capigo Mission, Capigo's project and
task-management module.

--tenant is optional on list, get, comments and update — each task is
addressed by its own id, and omitting --tenant on list or get searches every
tenant this key can reach. create and subtasks act on a tenant's board and
member list, so --tenant is required on those two.
  capigo help tenancy

USAGE
  capigo tasks <command> [--tenant <code>] [<args>]`,
}

// tasks list flags
var (
	taskListTenant        string
	taskListQuery         string
	taskListStatus        string
	taskListPriority      string
	taskListAssigneeID    string
	taskListOwnerID       string
	taskListBoardID       string
	taskListBoardListID   string
	taskListDueAfter      string
	taskListDueBefore     string
	taskListCreatedAfter  string
	taskListCreatedBefore string
	taskListParentTaskID  string
	taskListPage          int
	taskListLimit         int
)

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks (search, filter, paginate)",
	Long: `List tasks, optionally filtered.

PURPOSE
  Find tasks across boards, and across tenants. Look one up by title with
  --query, then read it in full with tasks get. Omitting --tenant searches
  every tenant this key can reach, and the result then names no tenant at
  all — neither in meta nor on the tasks themselves.

USAGE
  capigo tasks list [--tenant <code>] [-q <term>] [--status <text>]
                     [--priority <text>] [--assignee-id <uuid>]
                     [--owner-id <uuid>] [--board-id <uuid>]
                     [--board-list-id <uuid>] [--due-after <date>]
                     [--due-before <date>] [--created-after <ts>]
                     [--created-before <ts>] [--parent-task-id <uuid>|null]
                     [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to search. Optional — omit it to span every tenant this key can
      reach.

        capigo tasks list --tenant acme

  -q, --query <term>
      Search by task title.

        capigo tasks list --tenant acme -q "Fix login"

  --status <text>
      Filter by status: Pending, To-Do, Doing, Done, Closed, or Cancelled.

  --priority <text>
      Filter by priority, e.g. low, medium, high.

  --assignee-id <uuid>
      Only tasks assigned to this user.

  --owner-id <uuid>
      Only tasks owned by this user.

  --board-id <uuid>
      Only tasks on this board.

  --board-list-id <uuid>
      Only tasks in this board list.

  --due-after <date>
      ISO 8601 date. Only tasks due on or after it.

  --due-before <date>
      ISO 8601 date. Only tasks due on or before it.

  --created-after <ts>
      ISO 8601 timestamp. Only tasks created on or after it.

  --created-before <ts>
      ISO 8601 timestamp. Only tasks created on or before it.

  --parent-task-id <uuid>|null
      Only the children of one task. Pass the literal string null to return
      only top-level tasks; omit the flag to get both mixed together. Any
      other value exits 5.

        capigo tasks list --tenant acme --parent-task-id null

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page, 1 to 50. The default, 0, sends no limit parameter; the
      server then applies its own default of 20. Above 50 the server rejects
      the call with exit 5 — mission endpoints cap at 50, not at the 100 the
      PCMS lists allow.

        capigo tasks list --tenant acme --page 2 --limit 50

OUTPUT
  The tasks are at .data[]:

      {
        "data": [
          {
            "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
            "code": "TASK-104", "title": "Fix login bug", "description": "...",
            "status": "To-Do", "priority": "high", "assignee": {...},
            "owner": {...}, "board_id": "...", "board_list_id": "...",
            "due_date": "...", "parent": null, "has_subtasks": false,
            "attachments": [...], "created_at": "...", "updated_at": "..."
          }
        ],
        "meta": {
          "tenant": "acme", "tenant_source": "flag",
          "page": 1, "limit": 20, "total": 42, "has_more": true
        }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic.

  With --tenant omitted, meta.tenant and meta.tenant_source are absent: the
  search spanned every tenant this key can reach, and there is no single one
  to name. Neither does a task name its own — the API's task object carries no
  tenant field. A cross-tenant list therefore tells you which tasks exist, not
  which tenant each belongs to. Pass --tenant when that matters.

  Exit 5 if --parent-task-id is set to anything other than null or a task
  UUID.`,
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

		tenant := resolveTenant(taskListTenant, profile)

		path := tasksListPath(taskListFilters{
			query:         taskListQuery,
			status:        taskListStatus,
			priority:      taskListPriority,
			assigneeID:    taskListAssigneeID,
			ownerID:       taskListOwnerID,
			boardID:       taskListBoardID,
			boardListID:   taskListBoardListID,
			dueAfter:      taskListDueAfter,
			dueBefore:     taskListDueBefore,
			createdAfter:  taskListCreatedAfter,
			createdBefore: taskListCreatedBefore,
			parentTaskID:  taskListParentTaskID,
			page:          taskListPage,
			limit:         taskListLimit,
		})

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, taskListTenant, envelope.Meta))
	},
}

var (
	taskGetTenant string
	taskGetCode   string
)

var tasksGetCmd = &cobra.Command{
	Use:   "get [<id>]",
	Short: "Get a task by id or by code",
	Long: `Get one task, addressed by id or by code.

PURPOSE
  Read a single task in full. This is the authoritative source for a task's
  current status and assignee. tasks comments records how it got there, but
  activity entries are written asynchronously and can lag.

USAGE
  capigo tasks get (<id> | --code <code>) [--tenant <code>]

FLAGS
  <id>
      Task UUID. Positional. Give this or --code, never both; giving neither
      exits 5.

  --code <code>
      Address the task by its code — the key a person quotes, like ACMEC-68.
      A code is unique within a tenant, not across them, so --code needs a
      tenant: pass --tenant, or set a default. Give an id or --code, never
      both; a bare argument is never guessed at.

        capigo tasks get --code ACMEC-68 --tenant acme

  --tenant <code>
      Tenant to scope the lookup to. Optional with an id — the id alone finds
      the task regardless of tenant. Required with --code.

        capigo tasks get 7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10 --tenant acme

OUTPUT
  The task is at .data:

      {
        "data": {
          "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10",
          "code": "TASK-104", "title": "Fix login bug", "description": "...",
          "status": "To-Do", "priority": "high", "assignee": {...},
          "owner": {...}, "board_id": "...", "board_list_id": "...",
          "due_date": "...", "parent": null, "has_subtasks": false,
          "attachments": [...], "created_at": "...", "updated_at": "..."
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  data.attachments[] carries metadata only — id, file_name, mime_type,
  size_bytes. It never carries a download URL; tasks attachments download
  fetches one.

  meta.tenant and meta.tenant_source are absent when --tenant was omitted:
  the id alone found the task, independent of any one tenant.

  Exit 4 when no such task is reachable.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		var id string
		if len(args) == 1 {
			id = args[0]
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskGetTenant, profile)
		requireOneTaskAddress(id, taskGetCode, tenant)

		resp, err := client.Do(ctx, "GET", taskPath(id, taskGetCode), nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, taskGetTenant))
	},
}

// tasks comments flags
var (
	taskCommentsTenant string
	taskCommentsCode   string
	taskCommentsType   string
	taskCommentsSort   string
	taskCommentsPage   int
	taskCommentsLimit  int
)

var tasksCommentsCmd = &cobra.Command{
	Use:   "comments [<id>]",
	Short: "List a task's comments and activity timeline",
	Long: `Read a task's discussion and activity timeline.

PURPOSE
  See what people said and how the work progressed. Human comments are
  interleaved with system activity, newest first by default. For a task's
  CURRENT status or assignee, read tasks get instead — activity entries here
  are written asynchronously and can lag, so this command is the history, not
  the source of truth for live state.

USAGE
  capigo tasks comments (<id> | --code <code>) [--tenant <code>]
                        [--type comment|activity] [--sort asc|desc]
                        [--page <n>] [--limit <n>]

FLAGS
  <id>
      Task UUID. Positional. Give this or --code, never both.

  --code <code>
      Address the task by its code — the key a person quotes, like ACMEC-68.
      A code is unique within a tenant, not across them, so --code needs a
      tenant: pass --tenant, or set a default. Give an id or --code, never
      both; a bare argument is never guessed at.

        capigo tasks comments --code ACMEC-68 --tenant acme

  --tenant <code>
      Tenant to scope the lookup to. Optional with an id; required with
      --code.

  --type comment|activity
      Return only one kind. Omit to return both.

  --sort asc|desc
      Order by created_at. Defaults to desc, newest first.

  --page <n>
      Page to fetch, 1-based. The default, 0, sends no page parameter and
      lets the server choose.

  --limit <n>
      Items per page, at most 50. Values above 50 exit 5; the server rejects
      them rather than clamping.

        capigo tasks comments <uuid> --type comment --sort asc --limit 50

OUTPUT
  The entries are at .data[]:

      {
        "data": [
          { "id": "...",
            "author": { "id": "...", "name": "Minh", "type": "user" },
            "kind": "comment", "content": "Reproduced on staging.",
            "ui_data": null, "attachments": [...], "parent_id": null,
            "created_at": "2026-07-08T09:12:00Z" }
        ],
        "meta": { "page": 1, "limit": 20, "total": 6, "has_more": false }
      }

  kind is one of comment, activity, card, or artifact:

      comment    text a person or an agent typed; it is in content
      activity   a system event. content is a ready-made sentence, and
                 ui_data carries the structured before and after
      card       a card-shaped entry (structure not further specified here)
      artifact   an artifact-shaped entry (structure not further specified
                 here)

  author.name may read System when the original actor can no longer be
  resolved — a removed member, for instance. That is a graceful fallback, not
  an error.

  attachments[] carries metadata only, never a download URL.

  meta.tenant and meta.tenant_source are absent: a comments read is scoped to
  one task by id, not to a tenant.

  A task nobody has commented on returns an empty list and exit 0.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		var id string
		if len(args) == 1 {
			id = args[0]
		}

		// Validate flag values client-side so we fail fast (exit 5) before any
		// network call.
		if e := validateCommentParams(taskCommentsType, taskCommentsSort, taskCommentsLimit); e != nil {
			return handleErr(e)
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskCommentsTenant, profile)
		requireOneTaskAddress(id, taskCommentsCode, tenant)

		path := commentsPath(taskPath(id, taskCommentsCode), taskCommentsType, taskCommentsSort, taskCommentsPage, taskCommentsLimit)

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		// Comments are scoped to a single task, so there is no tenant in meta
		// even when a tenant was resolved implicitly.
		meta := output.Meta{
			Page:    output.Ptr(envelope.Meta.Page),
			Limit:   output.Ptr(envelope.Meta.Limit),
			Total:   output.Ptr(envelope.Meta.Total),
			HasMore: output.Ptr(envelope.Meta.HasMore),
		}
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// tasks update flags
var (
	taskUpdateTenant      string
	taskUpdateTitle       string
	taskUpdateDescription string
	taskUpdateStatus      string
	taskUpdateAssignee    string
	taskUpdateBoard       string
	taskUpdateList        string
	taskUpdateFollowerIDs []string
)

var tasksUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change a task's title, status, assignee, board or followers",
	Long: `Change some fields of a task. Fields you do not send are left unchanged.

PURPOSE
  Move a task forward: reassign it, change its status, place it on a board,
  or add followers. Read tasks get first if you need the current values
  before changing them.

USAGE
  capigo tasks update <id> [--tenant <code>] [--title <text>]
                           [--description <text>] [--status <text>]
                           [--assignee <uuid>] [--board <uuid> --list <uuid>]
                           [--follower-id <uuid>]...

FLAGS
  <id>
      Task UUID. Positional, required.

  --tenant <code>
      Tenant to scope the lookup to. Optional; the id alone finds the task
      regardless of tenant.

  --title <text>
      New title.

  --description <text>
      New description. An empty string clears it.

  --status <text>
      New status: Pending, To-Do, Doing, Done, Closed, or Cancelled.

        capigo tasks update <uuid> --status Done

  --assignee <uuid>
      New assignee. An empty string unassigns.

        capigo tasks update <uuid> --assignee ""

  --board <uuid>
      Board id. Always sent together with --list: setting either one sends
      both. Passing --board "" --list "" removes the task from its board.

  --list <uuid>
      Board list id. See --board.

        capigo tasks update <uuid> --board "" --list ""

  --follower-id <uuid>
      Add a follower. Repeatable. Additive and idempotent — this endpoint
      cannot remove a follower.

  At least one field flag is required; sending none exits 5.

OUTPUT
  The task as it now stands is at .data, the same shape as tasks get:

      {
        "data": { "id": "...", "code": "TASK-104", "title": "...", ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in, when --tenant was given or
  resolved from CAPIGO_TENANT/config; it is absent when the id alone found
  the task with no tenant resolved. A write into the wrong tenant looks
  exactly like a write that succeeded — read meta.tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id := args[0]

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskUpdateTenant, profile)

		// Build the PATCH body as a map so we can express the tri-state the
		// API needs: a field is absent (omitted), set to a value, or explicitly
		// null. nullableID maps an empty flag value to JSON null (unassign /
		// remove from board); a non-empty value is sent as-is.
		body := map[string]any{}
		nullableID := func(s string) any {
			if s == "" {
				return nil
			}
			return s
		}

		if cmd.Flags().Changed("title") {
			body["title"] = taskUpdateTitle
		}
		if cmd.Flags().Changed("description") {
			// The server schema accepts both "" and null for description; send the
			// literal value the caller passed (--description "" clears it to empty).
			body["description"] = taskUpdateDescription
		}
		if cmd.Flags().Changed("status") {
			body["status"] = taskUpdateStatus
		}
		if cmd.Flags().Changed("assignee") {
			body["assignee_id"] = nullableID(taskUpdateAssignee)
		}
		// board_id and board_list_id must be sent together; setting either flag
		// sends both. Empty string on either becomes null (remove from board).
		if cmd.Flags().Changed("board") || cmd.Flags().Changed("list") {
			body["board_id"] = nullableID(taskUpdateBoard)
			body["board_list_id"] = nullableID(taskUpdateList)
		}
		if len(taskUpdateFollowerIDs) > 0 {
			body["follower_ids"] = taskUpdateFollowerIDs
		}

		if len(body) == 0 {
			failValidation("at least one field must be provided for update")
		}

		resp, err := client.Do(ctx, "PATCH", "/mission/tasks/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, taskUpdateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// tasks create flags
var (
	taskCreateTenant       string
	taskCreateTitle        string
	taskCreateDescription  string
	taskCreatePriority     string
	taskCreateStatus       string
	taskCreateDueDate      string
	taskCreateAssignee     string
	taskCreateBoard        string
	taskCreateList         string
	taskCreateFollowerIDs  []string
	taskCreateSubtasksJSON string
)

// tasks subtasks flags
var (
	taskSubtasksTenant      string
	taskSubtasksCode        string
	taskSubtasksFromJSON    string
	taskSubtasksTitle       string
	taskSubtasksDescription string
	taskSubtasksAssignee    string
	taskSubtasksDueDate     string
	taskSubtasksPriority    string
	taskSubtasksStatus      string
)

var tasksCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a task, optionally with subtasks",
	Long: `Create a task, optionally with its subtasks in the same call.

PURPOSE
  Add a task to a tenant's board. With --subtasks-json the parent and its
  children are created atomically: if any subtask is invalid, nothing at all
  is created. To add subtasks to a task that already exists, use tasks
  subtasks instead.

USAGE
  capigo tasks create --tenant <code> --title <text> [--description <text>]
                       [--priority <text>] [--status <text>]
                       [--due-date <ts>] [--assignee <uuid>]
                       [--board <uuid> --list <uuid>]
                       [--follower-id <uuid>]... [--subtasks-json <path|->]

FLAGS
  --tenant <code>
      Tenant to create the task in. Required.

  --title <text>
      Task title. Required.

  --description <text>
      Task description.

  --priority <text>
      Priority, e.g. low, medium, high.

  --status <text>
      Initial status.

  --due-date <ts>
      RFC3339 timestamp. Note this differs from a subtask's due_date (see
      --subtasks-json), which is a calendar date.

  --assignee <uuid>
      Assignee user id.

  --board <uuid>
      Board id. Always sent together with --list.

  --list <uuid>
      Board list id. See --board.

  --follower-id <uuid>
      Follower user id. Repeatable.

  --subtasks-json <path|->
      A JSON array of subtask items, at most 25, creating the parent and its
      children atomically: if any item is invalid, nothing is created. - reads
      stdin. Only title is required on each item:

          { "title": "Design", "description": "...", "assignee_id": "<uuid>",
            "due_date": "2026-07-31", "priority": "Low|Normal|High|Urgent",
            "status": "Pending|To-Do|Doing|Done|Closed|Cancelled" }

        capigo tasks create --tenant acme --title "Fix login bug" \
            --priority high

        echo '[{"title":"Design"},{"title":"Build","priority":"High"}]' \
          | capigo tasks create --tenant acme --title "Epic X" --subtasks-json -

OUTPUT
  The shape of .data depends on whether subtasks were sent.

  Without --subtasks-json, .data is the bare created task, the same shape as
  tasks get:

      {
        "data": { "id": "...", "code": "TASK-104", "title": "...", ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  With --subtasks-json, .data carries both the parent and the children it was
  created with:

      { "data": { "task": { ... }, "subtasks": [ { ... } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" } }

  meta.tenant is the tenant the task was written to. Read it: a write that
  landed in the wrong tenant looks exactly like a write that succeeded.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if taskCreateTitle == "" {
			failValidation("--title is required")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskCreateTenant, profile)
		requireTenant(tenant, "tasks create")

		// --subtasks-json routes to the atomic parent+subtasks endpoint. The
		// parent task is built from the same create flags; the JSON payload is
		// the subtasks array. All-or-nothing: nothing is created if any part fails.
		if taskCreateSubtasksJSON != "" {
			raw, err := readJSONInput(taskCreateSubtasksJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --subtasks-json: %w", err))
			}
			var subtasks []api.SubtaskItem
			if err := json.Unmarshal(raw, &subtasks); err != nil {
				return handleErr(fmt.Errorf("--subtasks-json must be a JSON array of subtask items: %w", err))
			}

			task := api.CreateTaskWithSubtasksTask{Title: taskCreateTitle}
			if taskCreateDescription != "" {
				task.Description = &taskCreateDescription
			}
			if taskCreatePriority != "" {
				task.Priority = &taskCreatePriority
			}
			if taskCreateStatus != "" {
				task.Status = &taskCreateStatus
			}
			if taskCreateDueDate != "" {
				task.DueDate = &taskCreateDueDate
			}
			if taskCreateAssignee != "" {
				task.AssigneeID = &taskCreateAssignee
			}
			if taskCreateBoard != "" {
				task.BoardID = &taskCreateBoard
			}
			if taskCreateList != "" {
				task.BoardListID = &taskCreateList
			}
			if len(taskCreateFollowerIDs) > 0 {
				task.FollowerIDs = taskCreateFollowerIDs
			}

			resp, err := client.Do(ctx, "POST", "/mission/tasks/with-subtasks", api.CreateTaskWithSubtasksRequest{
				TenantCode: *tenant,
				Task:       task,
				Subtasks:   subtasks,
			}, tenant)
			if err != nil {
				return handleErr(err)
			}

			var envelope api.RawEnvelope
			if err := json.Unmarshal(resp.Body, &envelope); err != nil {
				return handleErr(fmt.Errorf("decode response: %w", err))
			}

			meta := itemMeta(tenant, taskCreateTenant)
			meta.ServerTime = resp.ServerTime
			return output.Write(os.Stdout, rawItem(envelope.Data), meta)
		}

		body := api.CreateTaskRequest{
			TenantCode: *tenant,
			Title:      taskCreateTitle,
		}
		if taskCreateDescription != "" {
			body.Description = &taskCreateDescription
		}
		if taskCreatePriority != "" {
			body.Priority = &taskCreatePriority
		}
		if taskCreateStatus != "" {
			body.Status = &taskCreateStatus
		}
		if taskCreateDueDate != "" {
			body.DueDate = &taskCreateDueDate
		}
		if taskCreateAssignee != "" {
			body.AssigneeID = &taskCreateAssignee
		}
		if taskCreateBoard != "" {
			body.BoardID = &taskCreateBoard
		}
		if taskCreateList != "" {
			body.BoardListID = &taskCreateList
		}
		if len(taskCreateFollowerIDs) > 0 {
			body.FollowerIDs = taskCreateFollowerIDs
		}

		// POST /mission/tasks: tenant_code is in the body; also send X-Tenant-Code header for consistency.
		resp, err := client.Do(ctx, "POST", "/mission/tasks", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, taskCreateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

var tasksSubtasksCmd = &cobra.Command{
	Use:   "subtasks [<parent-task-id>]",
	Short: "Add subtasks to an existing task",
	Long: `Add subtasks to a task that already exists.

PURPOSE
  Create children under a parent task that was created earlier. To create a
  parent and its subtasks together in one call, use tasks create
  --subtasks-json instead. Validation is all-or-nothing: if any item is
  invalid, nothing is created.

USAGE
  capigo tasks subtasks (<parent-id> | --code <code>) --tenant <code>
                         [--title <text> [--description <text>]
                          [--assignee <uuid>] [--due-date <date>]
                          [--priority <text>] [--status <text>]
                          | --from-json <path|->]

FLAGS
  <parent-id>
      Parent task UUID. Positional. Give this or --code, never both.

  --code <code>
      Address the parent task by its code — the key a person quotes, like
      ACMEC-68 — instead of by id. A bare argument is never guessed at.

        capigo tasks subtasks --code ACMEC-68 --tenant acme --title Design

  --tenant <code>
      Tenant the parent task belongs to. Required either way — a code is
      unique within a tenant, not across them.

  --title <text>
      Subtask title. Required unless --from-json is used.

        capigo tasks subtasks <parent-uuid> --tenant acme --title Design

  --description <text>
      Subtask description.

  --assignee <uuid>
      Assignee user id.

  --due-date <date>
      Calendar date, YYYY-MM-DD. This differs from a task's own --due-date
      (see tasks create), which is an RFC3339 timestamp.

  --priority <text>
      Priority: Low, Normal, High, or Urgent.

  --status <text>
      Status: Pending, To-Do, Doing, Done, Closed, or Cancelled.

  --from-json <path|->
      A JSON array of subtask items, at most 25, where - reads stdin. Only
      title is required on each item. Mutually exclusive with the single-item
      flags above.

        echo '[{"title":"Design"},{"title":"Build","priority":"High"}]' \
          | capigo tasks subtasks <parent-uuid> --tenant acme --from-json -

OUTPUT
  .data names the parent and the children it gained. It is neither a bare
  task nor a list envelope, and parent_task is trimmed — id, code, and title
  only, not the full task shape tasks get returns:

      {
        "data": {
          "parent_task": { "id": "...", "code": "TASK-104", "title": "..." },
          "subtasks": [ { "id": "...", "code": "TASK-105", "title": "Design",
                          ... } ]
        },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the subtasks were written to. Read it: a write
  that landed in the wrong tenant looks exactly like a write that succeeded.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		var id string
		if len(args) == 1 {
			id = args[0]
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskSubtasksTenant, profile)
		requireTenant(tenant, "tasks subtasks")
		requireOneTaskAddress(id, taskSubtasksCode, tenant)

		var subtasks []api.SubtaskItem
		if taskSubtasksFromJSON != "" {
			raw, err := readJSONInput(taskSubtasksFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			if err := json.Unmarshal(raw, &subtasks); err != nil {
				return handleErr(fmt.Errorf("--from-json must be a JSON array of subtask items: %w", err))
			}
		} else {
			if taskSubtasksTitle == "" {
				failValidation("--title is required (or use --from-json for a batch)")
			}
			item := api.SubtaskItem{Title: taskSubtasksTitle}
			if taskSubtasksDescription != "" {
				item.Description = &taskSubtasksDescription
			}
			if taskSubtasksAssignee != "" {
				item.AssigneeID = &taskSubtasksAssignee
			}
			if taskSubtasksDueDate != "" {
				item.DueDate = &taskSubtasksDueDate
			}
			if taskSubtasksPriority != "" {
				item.Priority = &taskSubtasksPriority
			}
			if taskSubtasksStatus != "" {
				item.Status = &taskSubtasksStatus
			}
			subtasks = []api.SubtaskItem{item}
		}

		resp, err := client.Do(ctx, "POST", taskPath(id, taskSubtasksCode)+"/subtasks", api.CreateSubtasksRequest{
			TenantCode: *tenant,
			Subtasks:   subtasks,
		}, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, taskSubtasksTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	// tasks list flags
	tasksListCmd.Flags().StringVar(&taskListTenant, "tenant", "", "scope to this tenant code")
	tasksListCmd.Flags().StringVarP(&taskListQuery, "query", "q", "", "search by task title")
	tasksListCmd.Flags().StringVar(&taskListStatus, "status", "", "filter by status")
	tasksListCmd.Flags().StringVar(&taskListPriority, "priority", "", "filter by priority (e.g. low, medium, high)")
	tasksListCmd.Flags().StringVar(&taskListAssigneeID, "assignee-id", "", "filter by assignee user UUID")
	tasksListCmd.Flags().StringVar(&taskListOwnerID, "owner-id", "", "filter by owner user UUID")
	tasksListCmd.Flags().StringVar(&taskListBoardID, "board-id", "", "filter by board UUID")
	tasksListCmd.Flags().StringVar(&taskListBoardListID, "board-list-id", "", "filter by board list UUID")
	tasksListCmd.Flags().StringVar(&taskListDueAfter, "due-after", "", "filter to tasks due on/after this ISO 8601 date")
	tasksListCmd.Flags().StringVar(&taskListDueBefore, "due-before", "", "filter to tasks due on/before this ISO 8601 date")
	tasksListCmd.Flags().StringVar(&taskListCreatedAfter, "created-after", "", "filter to tasks created on/after this ISO 8601 timestamp")
	tasksListCmd.Flags().StringVar(&taskListCreatedBefore, "created-before", "", "filter to tasks created on/before this ISO 8601 timestamp")
	tasksListCmd.Flags().StringVar(&taskListParentTaskID, "parent-task-id", "", "filter by parent task ID (use 'null' for top-level only)")
	tasksListCmd.Flags().IntVar(&taskListPage, "page", 0, "page number")
	tasksListCmd.Flags().IntVar(&taskListLimit, "limit", 0, "items per page")

	// tasks get flags
	tasksGetCmd.Flags().StringVar(&taskGetTenant, "tenant", "", "scope to this tenant code")
	tasksGetCmd.Flags().StringVar(&taskGetCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")

	// tasks comments flags
	tasksCommentsCmd.Flags().StringVar(&taskCommentsTenant, "tenant", "", "scope to this tenant code")
	tasksCommentsCmd.Flags().StringVar(&taskCommentsCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")
	tasksCommentsCmd.Flags().StringVar(&taskCommentsType, "type", "", "filter by kind: comment | activity (default: both)")
	tasksCommentsCmd.Flags().StringVar(&taskCommentsSort, "sort", "", "order by created_at: asc | desc (default: desc — newest first)")
	tasksCommentsCmd.Flags().IntVar(&taskCommentsPage, "page", 0, "page number (1-based)")
	tasksCommentsCmd.Flags().IntVar(&taskCommentsLimit, "limit", 0, "items per page (max 50)")

	// tasks update flags
	tasksUpdateCmd.Flags().StringVar(&taskUpdateTenant, "tenant", "", "scope to this tenant code")
	tasksUpdateCmd.Flags().StringVar(&taskUpdateTitle, "title", "", "new task title")
	tasksUpdateCmd.Flags().StringVar(&taskUpdateDescription, "description", "", "new task description (set to empty string to clear)")
	tasksUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "new status (Pending, To-Do, Doing, Done, Closed, Cancelled)")
	tasksUpdateCmd.Flags().StringVar(&taskUpdateAssignee, "assignee", "", "assignee user UUID (set to empty string to unassign)")
	tasksUpdateCmd.Flags().StringVar(&taskUpdateBoard, "board", "", `board UUID; sent together with --list (pass --board "" --list "" to remove from board)`)
	tasksUpdateCmd.Flags().StringVar(&taskUpdateList, "list", "", "board list UUID; sent together with --board")
	tasksUpdateCmd.Flags().StringArrayVar(&taskUpdateFollowerIDs, "follower-id", nil, "follower user UUID (repeatable: --follower-id <uuid>); additive — removes are not supported")

	// tasks create flags
	tasksCreateCmd.Flags().StringVar(&taskCreateTenant, "tenant", "", "tenant code (required)")
	tasksCreateCmd.Flags().StringVar(&taskCreateTitle, "title", "", "task title (required)")
	tasksCreateCmd.Flags().StringVar(&taskCreateDescription, "description", "", "task description")
	tasksCreateCmd.Flags().StringVar(&taskCreatePriority, "priority", "", "priority (e.g. low, medium, high)")
	tasksCreateCmd.Flags().StringVar(&taskCreateStatus, "status", "", "initial status")
	tasksCreateCmd.Flags().StringVar(&taskCreateDueDate, "due-date", "", "due date (RFC3339)")
	tasksCreateCmd.Flags().StringVar(&taskCreateAssignee, "assignee", "", "assignee user ID")
	tasksCreateCmd.Flags().StringVar(&taskCreateBoard, "board", "", "board ID")
	tasksCreateCmd.Flags().StringVar(&taskCreateList, "list", "", "board list ID")
	tasksCreateCmd.Flags().StringArrayVar(&taskCreateFollowerIDs, "follower-id", nil, "follower user ID (repeatable: --follower-id <uuid> --follower-id <uuid>)")
	tasksCreateCmd.Flags().StringVar(&taskCreateSubtasksJSON, "subtasks-json", "", "path to a JSON array of subtask items (use - for stdin); creates the task and its subtasks atomically via POST /mission/tasks/with-subtasks")

	// tasks subtasks flags
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksTenant, "tenant", "", "tenant code (required)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksCode, "code", "", "address the parent task by its code (e.g. ACMEC-68) instead of by id")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksFromJSON, "from-json", "", "path to a JSON array of subtask items (use - for stdin); mutually exclusive with the single-item flags")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksTitle, "title", "", "subtask title (required unless --from-json is used)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksDescription, "description", "", "subtask description")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksAssignee, "assignee", "", "assignee user UUID")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksDueDate, "due-date", "", "due date (YYYY-MM-DD)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksPriority, "priority", "", "priority (Low, Normal, High, Urgent)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksStatus, "status", "", "status (Pending, To-Do, Doing, Done, Closed, Cancelled)")

	// Registration order is display order (cobra.EnableCommandSorting is off).
	// tasksAttachmentsCmd is defined in task_attachments.go, whose init() runs
	// first — it is registered here so it lands after the verbs, not above them.
	taskCmd.AddCommand(tasksListCmd, tasksGetCmd, tasksCreateCmd, tasksUpdateCmd, tasksCommentsCmd, tasksSubtasksCmd, tasksAttachmentsCmd)
	rootCmd.AddCommand(taskCmd)
}

// commentMaxLimit mirrors the server's pagination cap for this endpoint
// (parsePagination rejects limit > 50); validated client-side for fast feedback.
const commentMaxLimit = 50

// validateCommentParams checks the enum and pagination flags for
// `tasks comments`. Returns nil when valid, or a VALIDATION_ERROR APIError
// (HTTP 400 → exit 5) describing the first offending flag.
func validateCommentParams(typeFlag, sortFlag string, limit int) *api.APIError {
	if typeFlag != "" && typeFlag != "comment" && typeFlag != "activity" {
		return &api.APIError{
			Code:       "VALIDATION_ERROR",
			Message:    "--type must be 'comment' or 'activity'",
			HTTPStatus: 400,
		}
	}
	if sortFlag != "" && sortFlag != "asc" && sortFlag != "desc" {
		return &api.APIError{
			Code:       "VALIDATION_ERROR",
			Message:    "--sort must be 'asc' or 'desc'",
			HTTPStatus: 400,
		}
	}
	if limit > commentMaxLimit {
		return &api.APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("--limit must be at most %d (got %d)", commentMaxLimit, limit),
			HTTPStatus: 400,
		}
	}
	return nil
}

// taskListFilters holds the `tasks list` flag values used to build the
// request query string. See apps/platform/src/lib/api/query-parser.ts
// ALLOWED_FILTER_COLUMNS on the capigo API for the full set of columns this
// endpoint accepts.
type taskListFilters struct {
	query         string
	status        string
	priority      string
	assigneeID    string
	ownerID       string
	boardID       string
	boardListID   string
	dueAfter      string
	dueBefore     string
	createdAfter  string
	createdBefore string
	parentTaskID  string
	page          int
	limit         int
}

// tasksListPath builds the request path + query string for `tasks list`.
// Empty/zero flag values are omitted so the server applies its own defaults.
func tasksListPath(f taskListFilters) string {
	params := url.Values{}
	if f.query != "" {
		params.Set("q", f.query)
	}
	if f.status != "" {
		params.Set("filters[status][$eq]", f.status)
	}
	if f.priority != "" {
		params.Set("filters[priority][$eq]", f.priority)
	}
	if f.assigneeID != "" {
		params.Set("filters[assignee_id][$eq]", f.assigneeID)
	}
	if f.ownerID != "" {
		params.Set("filters[owner_id][$eq]", f.ownerID)
	}
	if f.boardID != "" {
		params.Set("filters[board_id][$eq]", f.boardID)
	}
	if f.boardListID != "" {
		params.Set("filters[board_list_id][$eq]", f.boardListID)
	}
	if f.dueAfter != "" {
		params.Set("filters[due_date][$gte]", f.dueAfter)
	}
	if f.dueBefore != "" {
		params.Set("filters[due_date][$lte]", f.dueBefore)
	}
	if f.createdAfter != "" {
		params.Set("filters[created_at][$gte]", f.createdAfter)
	}
	if f.createdBefore != "" {
		params.Set("filters[created_at][$lte]", f.createdBefore)
	}
	if f.parentTaskID != "" {
		params.Set("parent_task_id", f.parentTaskID)
	}
	if f.page > 0 {
		params.Set("page", strconv.Itoa(f.page))
	}
	if f.limit > 0 {
		params.Set("limit", strconv.Itoa(f.limit))
	}

	path := "/mission/tasks"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return path
}

// commentsPath builds the request path + query string for `tasks comments`.
// Empty/zero flag values are omitted so the server applies its own defaults.
func commentsPath(base, typeFlag, sortFlag string, page, limit int) string {
	params := url.Values{}
	if typeFlag != "" {
		params.Set("type", typeFlag)
	}
	if sortFlag != "" {
		params.Set("sort", sortFlag)
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := base + "/comments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return path
}
