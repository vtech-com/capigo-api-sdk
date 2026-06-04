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
}

// tasks list flags
var (
	taskListTenant       string
	taskListQuery        string
	taskListStatus       string
	taskListParentTaskID string
	taskListPage         int
	taskListLimit        int
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

		params := url.Values{}
		if taskListQuery != "" {
			params.Set("q", taskListQuery)
		}
		if taskListStatus != "" {
			params.Set("filters[status][$eq]", taskListStatus)
		}
		if taskListParentTaskID != "" {
			params.Set("parent_task_id", taskListParentTaskID)
		}
		if taskListPage > 0 {
			params.Set("page", strconv.Itoa(taskListPage))
		}
		if taskListLimit > 0 {
			params.Set("limit", strconv.Itoa(taskListLimit))
		}

		path := "/mission/tasks"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

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

		if outputMode == "table" && envelope.Meta.HasMore {
			fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate.\n",
				len(envelope.Data), envelope.Meta.Total)
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

// tasks create flags
var (
	taskCreateTenant      string
	taskCreateTitle       string
	taskCreateDescription string
	taskCreatePriority    string
	taskCreateStatus      string
	taskCreateDueDate     string
	taskCreateAssignee    string
	taskCreateBoard       string
	taskCreateList        string
	taskCreateFollowerIDs []string
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

func init() {
	// tasks list flags
	tasksListCmd.Flags().StringVar(&taskListTenant, "tenant", "", "scope to this tenant code")
	tasksListCmd.Flags().StringVarP(&taskListQuery, "query", "q", "", "search by task title")
	tasksListCmd.Flags().StringVar(&taskListStatus, "status", "", "filter by status")
	tasksListCmd.Flags().StringVar(&taskListParentTaskID, "parent-task-id", "", "filter by parent task ID (use 'null' for top-level only)")
	tasksListCmd.Flags().IntVar(&taskListPage, "page", 0, "page number")
	tasksListCmd.Flags().IntVar(&taskListLimit, "limit", 0, "items per page")

	// tasks get flags
	tasksGetCmd.Flags().StringVar(&taskGetTenant, "tenant", "", "scope to this tenant code")

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

	taskCmd.AddCommand(tasksListCmd, tasksGetCmd, tasksCreateCmd)
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
