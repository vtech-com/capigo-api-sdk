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
	Long: `Manage categories in the Capigo Product Catalog Management System (PCMS).

Categories are tenant-scoped reference data. Every command here requires a tenant.
  capigo help tenancy`,
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
  Read the categories defined for a tenant, optionally narrowed by name.

INPUT
  --tenant <code>        required
  -q, --query <term>     name-contains filter, case-insensitive, max 200 chars
  --page <n>             page number
  --limit <n>            items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Each row is:

      { id, name, parent_id }

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo categories list --tenant acme -q pin

  # How many are there? Read meta.total rather than counting rows
  capigo categories list --tenant acme --limit 1 -o json | jq '.meta.total'

SEE ALSO
  categories get <id>       one category in full
  categories create         add a category
  capigo help output      output modes and the JSON contract
  capigo help tenancy     how --tenant resolves`,
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
  Add a category to this tenant's reference data.

INPUT
  --tenant <code>        required
  --name <text>          required, unless --from-json is used
  --parent-id <uuid>     the parent category; omit it to create a root category

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

  Body:

      { "name": "Phu kien", "parent_id": "8f2a-..." }
      { "name": "Phu kien" }

OUTPUT
  -o json emits the bare created category:

      { id, name, parent_id }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo categories create --tenant acme --name "Phu kien"

  echo '{"name":"Pin","parent_id":"8f2a-..."}' \
    | capigo categories create --tenant acme --from-json -

SEE ALSO
  categories update <id>    change some of its fields later
  categories list           check whether it already exists
  capigo help exit-codes  what exit 5 means`,
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
	Short: "Partial update of an existing category (PATCH)",
	Long: `Update a category. Fields you do not send are left unchanged.

PURPOSE
  Change part of a category (PATCH). To overwrite every field at once, use
  categories replace <id>.

INPUT
  <id>                   category UUID (positional, required)
  --tenant <code>        required
  --name <text>          a new name
  --parent-id <uuid>     a new parent category
  --clear-parent         set parent_id to null, promoting it to a root category

  At least one field is required; sending none exits 5.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare updated category:

      { id, name, parent_id }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo categories update <uuid> --tenant acme --name "Phu kien dien thoai"
  capigo categories update <uuid> --tenant acme --clear-parent

SEE ALSO
  categories replace <id>   overwrite every field instead
  categories get <id>       read the current values first
  capigo help exit-codes  what exit 5 means`,
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
// categories get
// --------------------------------------------------------------------------

var categoryGetTenant string

var categoriesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a category by ID",
	Long: `Get one category by UUID.

PURPOSE
  Read a single category. This command addresses it by UUID only. To find that
  UUID from a name, use categories list --query.

INPUT
  <id>                   category UUID (positional, required)
  --tenant <code>        required

OUTPUT
  -o json emits the bare category object:

      { id, name, parent_id }

  Exit 4 when no such category exists in the resolved tenant.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo categories get <uuid> --tenant acme

SEE ALSO
  categories list           find a category by name
  categories update <id>    change some of its fields
  categories replace <id>   overwrite all of its fields
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
	Long: `Replace a category. Every field is overwritten.

PURPOSE
  Overwrite a category in full (PUT). A field you do not send is not preserved —
  it is reset. To change one field and keep the rest, use categories update <id>.

INPUT
  <id>                   category UUID (positional, required)
  --tenant <code>        required
  --name <text>          required
  --parent-id <uuid>     the parent category
  --root                 set parent_id to null, making it a root category

  Exactly one of --parent-id and --root must be given; they are mutually
  exclusive; replace always writes the parent. To change one field and keep
  the rest, use categories update <id>.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare category as it now stands:

      { id, name, parent_id }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo categories replace <uuid> --tenant acme --name "Phu kien" --root

SEE ALSO
  categories update <id>    change one field and keep the rest
  categories get <id>       read the current values first
  capigo help exit-codes  what exit 5 means`,
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

	categoriesCmd.AddCommand(categoriesListCmd, categoriesCreateCmd, categoriesUpdateCmd, categoriesGetCmd, categoriesReplaceCmd)
	rootCmd.AddCommand(categoriesCmd)
}
