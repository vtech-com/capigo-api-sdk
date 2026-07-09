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
	Long: `Categories organize products into a tree via parent_id, in the Capigo
Product Catalog Management System (PCMS).

Categories are tenant-scoped reference data. Every command here requires a
tenant.
  capigo help tenancy

USAGE
  capigo categories <command> --tenant <code> [<args>]`,
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
	Long: `List categories.

PURPOSE
  Read the categories defined for a tenant, optionally narrowed by name. To
  read a single category whose id you already have, use categories get.

USAGE
  capigo categories list --tenant <code> [-q <term>] [--page <n>]
                         [--limit <n>] [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to read from. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

        capigo categories list --tenant acme

  -q, --query <term>
      Name-contains filter, case-insensitive, up to 200 characters.

        capigo categories list --tenant acme -q "phu kien"

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

        capigo categories list --tenant acme --page 2 --limit 100

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of categories, then a summary line. A root category has no
  ParentID.

      ┌──────────┬────────────┬──────────┐
      │ ID       │ Name       │ ParentID │
      ├──────────┼────────────┼──────────┤
      │ 7c1f2e88 │ Phu kien   │          │
      │ 9ab2c744 │ Op lung dt │ 7c1f2e88 │
      └──────────┴────────────┴──────────┘
      Tenant: acme · Total: 2 (all rows shown)

  Ids are shortened here to fit the page; the command prints them in full.

  -o json emits the list envelope; the categories are at .data[]:

      {
        "data": [
          { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10", "name": "Phu kien",
            "parent_id": null },
          { "id": "9ab2c744-1e3a-4b8c-9f10-5c1e2a4d9f10", "name": "Op lung dt",
            "parent_id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10" }
        ],
        "meta": { "page": 1, "limit": 20, "total": 2, "has_more": false }
      }`,
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
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, categoryListTenant),
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

// --------------------------------------------------------------------------
// categories get
// --------------------------------------------------------------------------

var categoryGetTenant string

var categoriesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a category by id",
	Long: `Get one category by id.

PURPOSE
  Read a single category, addressed by id only. To find that id from a name,
  use categories list --query.

USAGE
  capigo categories get <id> --tenant <code> [-o table|json|quiet]

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required. Exits 4 if the category is
      not in it.

        capigo categories get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table:

      ┌──────────┬──────────┬──────────┐
      │ ID       │ Name     │ ParentID │
      ├──────────┼──────────┼──────────┤
      │ 4d9a1c07 │ Phu kien │          │
      └──────────┴──────────┴──────────┘

  Ids are shortened here to fit the page; the command prints them in full.

  -o json emits the bare object. A get is not a list, so there is no envelope
  and no .data to reach for:

      {
        "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
        "name": "Phu kien",
        "parent_id": null
      }

  Exit 4 when no such category exists in the resolved tenant.`,
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
	Long: `Create a category.

PURPOSE
  Add a category to this tenant's reference data. Omit --parent-id to create
  a root category.

USAGE
  capigo categories create --tenant <code>
                           (--name <text> [--parent-id <uuid>]
                            | --from-json <path|->)
                           [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to create in. Required.

        capigo categories create --tenant acme --name "Phu kien"

  --name <text>
      Category name, 1 to 500 characters. Required, unless --from-json is
      used. A name that duplicates an existing category's name in this
      tenant exits 8.

  --parent-id <uuid>
      The parent category. Omit it to create a root category. An id that
      does not exist, or belongs to another tenant, exits 5.

        capigo categories create --tenant acme --name Pin --parent-id 8f2a-...

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. The
      individual field flags above are ignored when this is set.

      Body:

          { "name": "Pin", "parent_id": "8f2a-..." }
          { "name": "Phu kien" }

        echo '{"name":"Pin","parent_id":"8f2a-..."}' \
          | capigo categories create --tenant acme --from-json -

  -o, --output table|json|quiet
      Print the created row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare created category:

      { "id": "8f2a-...", "name": "Pin", "parent_id": "8f2a-..." }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output`,
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
		defer echoTenant(tenant, categoryCreateTenant)
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

		emitServerTime(resp.ServerTime, "")

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
	Short: "Change some fields of a category",
	Long: `Update a category. Fields you do not send are left unchanged.

PURPOSE
  Change part of a category: its name, or its parent. To overwrite every
  field at once, use categories replace <id>.

USAGE
  capigo categories update <id> --tenant <code>
                           ([--name <text>]
                            [--parent-id <uuid> | --clear-parent]
                            | --from-json <path|->)
                           [-o table|json|quiet]

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required.

  --name <text>
      A new name, 1 to 500 characters. A name that duplicates another
      category's name in this tenant exits 8.

        capigo categories update <uuid> --tenant acme --name "Phu kien dt"

  --parent-id <uuid>
      A new parent category. Mutually exclusive with --clear-parent. An id
      that does not exist, creates a cycle, or belongs to another tenant,
      exits 5.

  --clear-parent
      Set parent_id to null, promoting the category to root. Mutually
      exclusive with --parent-id.

        capigo categories update <uuid> --tenant acme --clear-parent

  At least one of --name, --parent-id, --clear-parent is required; sending
  none exits 5.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with the individual field flags above: passing both exits 5.

        echo '{"name":"Phu kien dt"}' \
          | capigo categories update <uuid> --tenant acme --from-json -

  -o, --output table|json|quiet
      Print the updated row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare updated category:

      { "id": "...", "name": "Phu kien dt", "parent_id": null }

  Output modes and the JSON contract: capigo help output`,
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
		defer echoTenant(tenant, categoryUpdateTenant)
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

		emitServerTime(resp.ServerTime, "")

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
	Short: "Overwrite every field of a category",
	Long: `Replace a category's name and parent in one call.

PURPOSE
  Rewrite a category's name and its place in the tree together in one call.
  The API does not clear a field you omit on PUT — like PATCH, it changes
  only what you send — but this command requires --name and one of
  --parent-id/--root on every call, so a stale field survives only if you
  typed it that way. To change one field without retyping the rest, use
  categories update <id>.

USAGE
  capigo categories replace <id> --tenant <code>
                            (--name <text> [--parent-id <uuid> | --root]
                             | --from-json <path|->)
                            [-o table|json|quiet]

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required.

  --name <text>
      Category name, 1 to 500 characters. Required. A name that duplicates
      another category's name in this tenant exits 8.

  --parent-id <uuid>
      The parent category. Mutually exclusive with --root. An id that does
      not exist, creates a cycle, or belongs to another tenant, exits 5.

  --root
      Set parent_id to null, making the category a root. Mutually exclusive
      with --parent-id.

  Exactly one of --parent-id and --root is required; replace always writes
  the parent.

        capigo categories replace <uuid> --tenant acme --name "Phu kien" --root

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with the individual field flags above: passing both exits 5.

  -o, --output table|json|quiet
      Print the replaced row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare category as it now stands:

      { "id": "...", "name": "Phu kien", "parent_id": null }

  Output modes and the JSON contract: capigo help output`,
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
		defer echoTenant(tenant, categoryReplaceTenant)
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

		emitServerTime(resp.ServerTime, "")

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

	categoriesCmd.AddCommand(categoriesListCmd, categoriesGetCmd, categoriesCreateCmd, categoriesUpdateCmd, categoriesReplaceCmd)
	rootCmd.AddCommand(categoriesCmd)
}
