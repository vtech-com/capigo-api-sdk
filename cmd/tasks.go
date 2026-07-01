package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var taskCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
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
	Short: "List tasks",
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

		var envelope api.Envelope[[]api.Task]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Task{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Task, len(envelope.Data))
		for i, t := range envelope.Data {
			items[i] = toOutputTask(t)
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "task",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, taskListTenant),
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
	taskGetTenant string
)

var tasksGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a task by ID",
	Args:  cobra.ExactArgs(1),
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

		tenant := resolveTenant(taskGetTenant, profile)

		resp, err := client.Do(ctx, "GET", "/mission/tasks/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Task `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputTask(envelope.Data), output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "task",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

// tasks comments flags
var (
	taskCommentsTenant string
	taskCommentsType   string
	taskCommentsSort   string
	taskCommentsPage   int
	taskCommentsLimit  int
)

var tasksCommentsCmd = &cobra.Command{
	Use:   "comments <id>",
	Short: "List a task's comments and activity timeline",
	Long: `List the conversation and activity timeline of a task: human comments
interleaved with system activity (status, assignment, title, description,
due-date and create events).

Each entry has a "kind":
  comment   a message typed by a person or an agent
  activity  a system event (e.g. "X changed status from Doing to Done")

Use --type comment or --type activity to return only one kind. Order is newest
first by default; pass --sort asc for oldest first.

A task that nobody has commented on yet returns an empty list (exit 0), not an
error. The authoritative current status of a task lives on the task itself
(tasks get) — this command provides the history/narrative.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

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

		path := commentsPath(args[0], taskCommentsType, taskCommentsSort, taskCommentsPage, taskCommentsLimit)

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.TaskComment]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.TaskComment{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.TaskComment, len(envelope.Data))
		for i, c := range envelope.Data {
			items[i] = toOutputComment(c)
		}

		// Comments are scoped to a single task, so there is no tenant column even
		// when the tenant was resolved implicitly.
		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "task_comment",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, taskCommentsTenant),
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
	Short: "Partial update of a task (PATCH)",
	Long: `Partial update (PATCH) of an existing task. All fields are optional;
at least one must be provided. Fields not specified are left unchanged.

Clearing semantics:
  --assignee ""              unassign the task (sends assignee_id: null)
  --board "" --list ""       remove the task from its board (both null)

--board and --list are sent together: setting either flag sends both. To move
a task onto a board pass --board <uuid> --list <uuid>; to remove it pass both
as empty strings. (The API requires board_id and board_list_id together.)

follower_ids is additive: listed users are added as followers (idempotent);
removing followers is not supported by this endpoint.`,
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
		defer echoTenant(tenant, taskUpdateTenant)

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
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "at least one field must be provided for update",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.Do(ctx, "PATCH", "/mission/tasks/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Task `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputTask(envelope.Data), output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "task",
		}); err != nil {
			return handleErr(err)
		}

		return nil
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
	Short: "Create a new task",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		if taskCreateTitle == "" {
			err := &api.APIError{Code: "VALIDATION_ERROR", Message: "--title is required", HTTPStatus: 400}
			output.RenderError(os.Stderr, outputMode, err.Code, err.Message, "")
			os.Exit(api.ExitCodeFor(err))
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
		defer echoTenant(tenant, taskCreateTenant)

		// POST /mission/tasks requires a tenant; reject if nil.
		_ = api.CreateTaskUsesBodyField
		if tenant == nil {
			err := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "tasks create requires a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, err.Code, err.Message, "")
			os.Exit(api.ExitCodeFor(err))
		}

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

			var envelope struct {
				Data struct {
					Task     api.Task   `json:"task"`
					Subtasks []api.Task `json:"subtasks"`
				} `json:"data"`
			}
			if err := json.Unmarshal(resp.Body, &envelope); err != nil {
				return handleErr(fmt.Errorf("decode response: %w", err))
			}

			if outputMode == "json" {
				return output.WriteJSONObject(os.Stdout, envelope.Data)
			}
			return renderTaskList(append([]api.Task{envelope.Data.Task}, envelope.Data.Subtasks...), tenant)
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

		var envelope struct {
			Data api.Task `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, toOutputTask(envelope.Data), output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "task",
		}); err != nil {
			return handleErr(err)
		}

		return nil
	},
}

var tasksSubtasksCmd = &cobra.Command{
	Use:   "subtasks <parent-task-id>",
	Short: "Create subtasks under an existing task",
	Long: `Batch-create subtasks under an existing parent task (1–25 per request).

Validation is all-or-nothing: if any subtask is invalid, nothing is created.

Single subtask via flags:

  capigo tasks subtasks <parent-id> --tenant acme --title "Design mockups"

Batch via JSON (an array of subtask items; use - for stdin):

  echo '[{"title":"A"},{"title":"B","priority":"High","assignee_id":"<uuid>"}]' \
    | capigo tasks subtasks <parent-id> --tenant acme --from-json -

Each subtask item: title (required), description, assignee_id, due_date
(YYYY-MM-DD), priority (Low/Normal/High/Urgent), status (Pending/To-Do/Doing/
Done/Closed/Cancelled). When --from-json is set, the single-item flags are ignored.`,
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

		tenant := resolveTenant(taskSubtasksTenant, profile)
		defer echoTenant(tenant, taskSubtasksTenant)

		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "tasks subtasks requires a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

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
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--title is required (or use --from-json for a batch)", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
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

		resp, err := client.Do(ctx, "POST", "/mission/tasks/"+args[0]+"/subtasks", api.CreateSubtasksRequest{
			TenantCode: *tenant,
			Subtasks:   subtasks,
		}, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data struct {
				ParentTask struct {
					ID    string `json:"id"`
					Code  string `json:"code"`
					Title string `json:"title"`
				} `json:"parent_task"`
				Subtasks []api.Task `json:"subtasks"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}
		return renderTaskList(envelope.Data.Subtasks, tenant)
	},
}

// renderTaskList renders a slice of tasks as the standard task table.
func renderTaskList(tasks []api.Task, tenant *string) error {
	items := make([]output.Task, len(tasks))
	for i, t := range tasks {
		items[i] = toOutputTask(t)
	}
	if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
		GlobalMode:   tenant == nil,
		ResourceKind: "task",
	}); err != nil {
		return handleErr(err)
	}
	return nil
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

	// tasks comments flags
	tasksCommentsCmd.Flags().StringVar(&taskCommentsTenant, "tenant", "", "scope to this tenant code")
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
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksFromJSON, "from-json", "", "path to a JSON array of subtask items (use - for stdin); mutually exclusive with the single-item flags")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksTitle, "title", "", "subtask title (required unless --from-json is used)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksDescription, "description", "", "subtask description")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksAssignee, "assignee", "", "assignee user UUID")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksDueDate, "due-date", "", "due date (YYYY-MM-DD)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksPriority, "priority", "", "priority (Low, Normal, High, Urgent)")
	tasksSubtasksCmd.Flags().StringVar(&taskSubtasksStatus, "status", "", "status (Pending, To-Do, Doing, Done, Closed, Cancelled)")

	taskCmd.AddCommand(tasksListCmd, tasksGetCmd, tasksCommentsCmd, tasksUpdateCmd, tasksCreateCmd, tasksSubtasksCmd)
	rootCmd.AddCommand(taskCmd)
}

// toOutputTask converts an api.Task to an output.Task for rendering.
func toOutputTask(t api.Task) output.Task {
	assignee := ""
	if t.Assignee != nil {
		assignee = t.Assignee.DisplayName
	}
	return output.Task{
		ID:       t.ID,
		Code:     t.Code,
		Title:    t.Title,
		Status:   t.Status,
		Assignee: assignee,
	}
}

// toOutputComment converts an api.TaskComment to an output.TaskComment for
// table/quiet rendering. The full, unmodified content and structured ui_data are
// only available in json mode; the table content is flattened to one line and
// truncated for readability.
func toOutputComment(c api.TaskComment) output.TaskComment {
	content := ""
	if c.Content != nil {
		content = flattenForTable(*c.Content, 100)
	}
	return output.TaskComment{
		ID:          c.ID,
		Created:     c.CreatedAt,
		Author:      c.Author.Name,
		Kind:        c.Kind,
		Content:     content,
		Attachments: len(c.Attachments),
	}
}

// flattenForTable collapses whitespace runs (newlines/tabs) into single spaces
// and truncates to max runes with an ellipsis, so free-form comment text does
// not break table layout. Display-only: json mode returns the raw content.
func flattenForTable(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
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
func commentsPath(id, typeFlag, sortFlag string, page, limit int) string {
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

	path := "/mission/tasks/" + id + "/comments"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return path
}
