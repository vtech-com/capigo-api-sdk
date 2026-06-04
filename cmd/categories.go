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

var categoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Manage PCMS categories",
}

// --------------------------------------------------------------------------
// categories list
// --------------------------------------------------------------------------

var (
	categoryListTenant string
	categoryListQuery  string
	categoryListPage   int
	categoryListLimit  int
)

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List categories",
	Long: `List categories from the PCMS catalog. Tenant is required.

Use --query / -q for a name-contains search (case-insensitive, max 200 chars).
Each category in the response has: id, name, parent_id (uuid or null for root).`,
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

		tenant := resolveTenant(categoryListTenant, profile)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "categories commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		validatePCMSLimit(categoryListLimit)

		resp, err := client.ListCategories(ctx, tenant, categoryListQuery, categoryListPage, categoryListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Category]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Category{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Category, len(envelope.Data))
		for i, c := range envelope.Data {
			items[i] = output.Category{
				ID:       c.ID,
				Name:     c.Name,
				ParentID: c.ParentID,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "category",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			if envelope.Meta.HasMore {
				fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate.\n",
					len(envelope.Data), envelope.Meta.Total)
			}
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// categories create
// --------------------------------------------------------------------------

var (
	categoryCreateTenant   string
	categoryCreateName     string
	categoryCreateParentID string
	categoryCreateFromJSON string
)

var categoriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new category",
	Long: `Create a new category in PCMS. Tenant is required.

Provide --name and optional --parent-id, or supply the full request body
with --from-json <file> (use - to read from stdin). When --from-json is
set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Electronics" }
  { "name": "Smartphones", "parent_id": "uuid-of-parent" }
  { "name": "Root Category", "parent_id": null }

Response: { "data": { "id": "uuid", "name": "string", "parent_id": "uuid|null" } }`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(categoryCreateTenant, profile)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "categories commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if categoryCreateFromJSON != "" {
			for _, f := range []string{"name", "parent-id"} {
				if cmd.Flags().Changed(f) {
					e := &api.APIError{
						Code:       "VALIDATION_ERROR",
						Message:    fmt.Sprintf("--from-json and --%s are mutually exclusive", f),
						HTTPStatus: 400,
					}
					output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
					os.Exit(api.ExitCodeFor(e))
				}
			}
			raw, err := readJSONInput(categoryCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if categoryCreateName == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--name is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			req := api.CreateCategoryRequest{Name: categoryCreateName}
			if categoryCreateParentID != "" {
				req.ParentID = &categoryCreateParentID
			}
			body = req
		}

		resp, err := client.Do(ctx, "POST", "/pcms/categories", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Category `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Category{
			ID:       envelope.Data.ID,
			Name:     envelope.Data.Name,
			ParentID: envelope.Data.ParentID,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "category"})
	},
}

// --------------------------------------------------------------------------
// categories update
// --------------------------------------------------------------------------

var (
	categoryUpdateTenant      string
	categoryUpdateName        string
	categoryUpdateParentID    string
	categoryUpdateClearParent bool
	categoryUpdateFromJSON    string
)

var categoriesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Partial update of an existing category (PATCH)",
	Long: `Partial update (PATCH) of an existing category in PCMS. Tenant is required.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged. Use --clear-parent to promote a category to root (sets
parent_id to null). Omitting --parent-id leaves the current parent unchanged.

Use --from-json to supply the full update body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "New Name" }
  { "parent_id": "uuid-of-new-parent" }
  { "parent_id": null }

Response: { "data": { "id": "uuid", "name": "string", "parent_id": "uuid|null" } }`,
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

		tenant := resolveTenant(categoryUpdateTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "categories commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if categoryUpdateFromJSON != "" {
			for _, f := range []string{"name", "parent-id", "clear-parent"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(categoryUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			m := map[string]any{}
			if categoryUpdateName != "" {
				m["name"] = categoryUpdateName
			}
			if categoryUpdateClearParent {
				m["parent_id"] = nil
			} else if categoryUpdateParentID != "" {
				m["parent_id"] = categoryUpdateParentID
			}
			if len(m) == 0 {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "at least one field must be provided for update", "")
				os.Exit(5)
			}
			body = m
		}

		resp, err := client.Do(ctx, "PATCH", "/pcms/categories/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Category `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Category{
			ID:       envelope.Data.ID,
			Name:     envelope.Data.Name,
			ParentID: envelope.Data.ParentID,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "category"})
	},
}

// --------------------------------------------------------------------------
// categories get
// --------------------------------------------------------------------------

var categoryGetTenant string

var categoriesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a category by ID",
	Long: `Get a single category by ID from PCMS. Tenant is required.

Returns 404 for both not-found and cross-tenant resources (no info leakage).
Response: { "data": { "id": "uuid", "name": "string", "parent_id": "uuid|null" } }`,
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

		tenant := resolveTenant(categoryGetTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "categories commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		resp, err := client.Do(ctx, "GET", "/pcms/categories/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Category `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Category{
			ID:       envelope.Data.ID,
			Name:     envelope.Data.Name,
			ParentID: envelope.Data.ParentID,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "category"})
	},
}

// --------------------------------------------------------------------------
// categories replace
// --------------------------------------------------------------------------

var (
	categoryReplaceTenant   string
	categoryReplaceName     string
	categoryReplaceParentID string
	categoryReplaceRoot     bool
	categoryReplaceFromJSON string
)

var categoriesReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Full replace of a category (PUT)",
	Long: `Full replace (PUT) of an existing category in PCMS. Tenant is required.

All fields are required by the server. You must provide either --parent-id <uuid>
or --root (to set parent_id to null, promoting the category to root);
these flags are mutually exclusive.

Use --from-json to supply the full request body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Electronics", "parent_id": null }
  { "name": "Smartphones", "parent_id": "uuid-of-parent" }

Response: { "data": { "id": "uuid", "name": "string", "parent_id": "uuid|null" } }`,
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

		tenant := resolveTenant(categoryReplaceTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "categories commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if categoryReplaceFromJSON != "" {
			for _, f := range []string{"name", "parent-id", "root"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(categoryReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if categoryReplaceName == "" {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--name is required for replace", "")
				os.Exit(5)
			}
			parentIDSet := cmd.Flags().Changed("parent-id")
			if parentIDSet && categoryReplaceRoot {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--parent-id and --root are mutually exclusive", "")
				os.Exit(5)
			}
			if !parentIDSet && !categoryReplaceRoot {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "one of --parent-id or --root is required for replace", "")
				os.Exit(5)
			}
			req := api.ReplaceCategoryRequest{Name: categoryReplaceName}
			if !categoryReplaceRoot {
				req.ParentID = &categoryReplaceParentID
			}
			body = req
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/categories/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Category `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Category{
			ID:       envelope.Data.ID,
			Name:     envelope.Data.Name,
			ParentID: envelope.Data.ParentID,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "category"})
	},
}

func init() {
	categoriesListCmd.Flags().StringVar(&categoryListTenant, "tenant", "", "tenant code (required)")
	categoriesListCmd.Flags().StringVarP(&categoryListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	categoriesListCmd.Flags().IntVar(&categoryListPage, "page", 0, "page number")
	categoriesListCmd.Flags().IntVar(&categoryListLimit, "limit", 20, "items per page (1-100)")

	categoriesCreateCmd.Flags().StringVar(&categoryCreateTenant, "tenant", "", "tenant code (required)")
	categoriesCreateCmd.Flags().StringVar(&categoryCreateName, "name", "", "category name (required unless --from-json is used)")
	categoriesCreateCmd.Flags().StringVar(&categoryCreateParentID, "parent-id", "", "parent category UUID (omit for root)")
	categoriesCreateCmd.Flags().StringVar(&categoryCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateTenant, "tenant", "", "tenant code (required)")
	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateName, "name", "", "new category name")
	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateParentID, "parent-id", "", "new parent category UUID")
	categoriesUpdateCmd.Flags().BoolVar(&categoryUpdateClearParent, "clear-parent", false, "set parent_id to null (promote category to root)")
	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	categoriesGetCmd.Flags().StringVar(&categoryGetTenant, "tenant", "", "tenant code (required)")

	categoriesReplaceCmd.Flags().StringVar(&categoryReplaceTenant, "tenant", "", "tenant code (required)")
	categoriesReplaceCmd.Flags().StringVar(&categoryReplaceName, "name", "", "category name (required)")
	categoriesReplaceCmd.Flags().StringVar(&categoryReplaceParentID, "parent-id", "", "parent category UUID (mutually exclusive with --root)")
	categoriesReplaceCmd.Flags().BoolVar(&categoryReplaceRoot, "root", false, "set parent_id to null (promote to root; mutually exclusive with --parent-id)")
	categoriesReplaceCmd.Flags().StringVar(&categoryReplaceFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin); mutually exclusive with individual field flags")

	categoriesCmd.AddCommand(categoriesListCmd, categoriesCreateCmd, categoriesUpdateCmd, categoriesGetCmd, categoriesReplaceCmd)
	rootCmd.AddCommand(categoriesCmd)
}
