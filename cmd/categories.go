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
tenant, and every response names the tenant it resolved to, in meta.

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
                         [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to read from. Required.

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

OUTPUT
  The categories are at .data[]. A root category has a null parent_id:

      {
        "data": [
          { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10", "name": "Phu kien",
            "parent_id": null },
          { "id": "9ab2c744-1e3a-4b8c-9f10-5c1e2a4d9f10", "name": "Op lung dt",
            "parent_id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10" }
        ],
        "meta": {
          "tenant": "acme", "tenant_source": "flag",
          "page": 1, "limit": 20, "total": 2, "has_more": false
        }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic.

  meta.tenant is the tenant this call actually ran against, and
  meta.tenant_source says whether that came from the flag, from CAPIGO_TENANT,
  or from the config file. See capigo help tenancy.`,
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
		requireTenant(tenant, "categories commands")

		validatePCMSLimit(categoryListLimit)

		resp, err := client.ListCategories(ctx, tenant, categoryListQuery, categoryListPage, categoryListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Category]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, envelope.Data, listMeta(tenant, categoryListTenant, envelope.Meta))
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
  capigo categories get <id> --tenant <code>

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required. Exits 4 if the category is
      not in it.

        capigo categories get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The category is at .data — an object, where a list puts an array. The
  envelope is the same either way:

      {
        "data": {
          "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
          "name": "Phu kien",
          "parent_id": null
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  A single-item read carries no pagination meta; there is nothing to page.

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
		requireTenant(tenant, "categories commands")

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

		return output.Write(os.Stdout, envelope.Data, itemMeta(tenant, categoryGetTenant))
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
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --parent-id: passing both exits 5.

      Body:

          { "name": "Pin", "parent_id": "8f2a-..." }
          { "name": "Phu kien" }

        echo '{"name":"Pin","parent_id":"8f2a-..."}' \
          | capigo categories create --tenant acme --from-json -

OUTPUT
  The created category is at .data:

      {
        "data": { "id": "8f2a-...", "name": "Pin", "parent_id": "8f2a-..." },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the category was written to. Read it: a write
  that landed in the wrong tenant looks exactly like a write that succeeded.

  Exit 5 if --name is missing (and --from-json is not used), or if
  --from-json is combined with a field flag. Exit 8 on a name collision.`,
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
		requireTenant(tenant, "categories commands")

		var body any
		if categoryCreateFromJSON != "" {
			for _, f := range []string{"name", "parent-id"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(categoryCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if categoryCreateName == "" {
				failValidation("--name is required")
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

		meta := itemMeta(tenant, categoryCreateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
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
  Change one or a few fields of a category without restating the rest. This
  is PATCH: the API accepts any subset, so long as it is not empty, and leaves
  every field you do not send unchanged.

  categories replace <id> is the other half of the pair. It sends PUT, which
  the API refuses unless every field is present — use it when you want the
  whole record stated.

USAGE
  capigo categories update <id> --tenant <code>
                           ([--name <text>]
                            [--parent-id <uuid> | --clear-parent]
                            | --from-json <path|->)

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required. Exits 4 if the category is
      not in it.

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

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --parent-id and --clear-parent: passing both
      exits 5.

        echo '{"name":"Phu kien dt"}' \
          | capigo categories update <uuid> --tenant acme --from-json -

OUTPUT
  The category as it now stands is at .data:

      {
        "data": { "id": "...", "name": "Phu kien dt", "parent_id": null },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if no field flag is given (and --from-json is not used), or if
  --from-json is combined with a field flag. Exit 4 if <id> is not in the
  resolved tenant. Exit 8 on a name collision.`,
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
		requireTenant(tenant, "categories commands")

		var body any
		if categoryUpdateFromJSON != "" {
			for _, f := range []string{"name", "parent-id", "clear-parent"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
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
				failValidation("at least one field must be provided for update")
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

		meta := itemMeta(tenant, categoryUpdateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
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
  Send the category's whole field set in one call. PUT is a true replace: the
  API requires every field on the request and rejects one that leaves any out,
  with exit 5 and the message "Required". parent_id may be null — that is what
  --root sends — but it must be sent. This command's flags mirror that rule:
  --name, and one of --parent-id or --root, on every call. To change one field
  and leave the rest alone, use categories update <id>, which sends PATCH.

USAGE
  capigo categories replace <id> --tenant <code>
                            (--name <text> [--parent-id <uuid> | --root]
                             | --from-json <path|->)

FLAGS
  <id>
      Category id, a UUID. Positional, required.

  --tenant <code>
      Tenant the category belongs to. Required. Exits 4 if the category is
      not in it.

  --name <text>
      Category name, 1 to 500 characters. Required, unless --from-json is
      used. A name that duplicates another category's name in this tenant
      exits 8.

  --parent-id <uuid>
      The parent category. Mutually exclusive with --root. An id that does
      not exist, creates a cycle, or belongs to another tenant, exits 5.

  --root
      Set parent_id to null, making the category a root. Mutually exclusive
      with --parent-id.

  Exactly one of --parent-id and --root is required unless --from-json is
  used; replace always writes the parent.

        capigo categories replace <uuid> --tenant acme --name "Phu kien" --root

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --parent-id and --root: passing both exits 5.

OUTPUT
  The category as it now stands is at .data:

      {
        "data": { "id": "...", "name": "Phu kien", "parent_id": null },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if --name or a parent flag is missing (and --from-json is not
  used), if both --parent-id and --root are given, or if --from-json is
  combined with a field flag. Exit 4 if <id> is not in the resolved tenant.
  Exit 8 on a name collision.`,
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
		requireTenant(tenant, "categories commands")

		var body any
		if categoryReplaceFromJSON != "" {
			for _, f := range []string{"name", "parent-id", "root"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(categoryReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if categoryReplaceName == "" {
				failValidation("--name is required for replace")
			}
			parentIDSet := cmd.Flags().Changed("parent-id")
			if parentIDSet && categoryReplaceRoot {
				failValidation("--parent-id and --root are mutually exclusive")
			}
			if !parentIDSet && !categoryReplaceRoot {
				failValidation("one of --parent-id or --root is required for replace")
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

		meta := itemMeta(tenant, categoryReplaceTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
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
