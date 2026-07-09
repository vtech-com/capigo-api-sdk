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

var productTypesCmd = &cobra.Command{
	Use:   "product-types",
	Short: "Manage PCMS product types",
	Long: `Product types in the Capigo Product Catalog Management System (PCMS).

Product types are tenant-scoped reference data. Every command here requires a
tenant.

USAGE
  capigo product-types <command> --tenant <code> [<args>]`,
}

// --------------------------------------------------------------------------
// product-types list
// --------------------------------------------------------------------------

var (
	productTypeListTenant string
	productTypeListQuery  string
	productTypeListPage   int
	productTypeListLimit  int
)

var productTypesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List product types",
	Long: `List product types.

PURPOSE
  Read the product types defined for a tenant, optionally narrowed by name.
  To find the id of one by name, this is the command; to read it by id, use
  product-types get.

USAGE
  capigo product-types list --tenant <code> [-q <term>]
                            [--page <n>] [--limit <n>]
                            [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to read from. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

        capigo product-types list --tenant acme

  -q, --query <term>
      Name-contains filter, case-insensitive. 1 to 200 characters.

        capigo product-types list --tenant acme -q pin

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

        capigo product-types list --tenant acme --page 2 --limit 100

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of product types, then a summary line.

      ┌──────────────────────────────────────┬──────────────┐
      │ ID                                   │ Name         │
      ├──────────────────────────────────────┼──────────────┤
      │ 3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33 │ Pin Lien Cap │
      └──────────────────────────────────────┴──────────────┘
      Tenant: acme · Total: 12 · showing 12 (page 1/1)

  -o json emits the list envelope; the product types are at .data[]:

      {
        "data": [
          { "id": "3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33",
            "name": "Pin Lien Cap", "description": "Pin va cap lien khoi" }
        ],
        "meta": { "page": 1, "limit": 20, "total": 12, "has_more": false }
      }

      capigo product-types list --tenant acme --limit 1 -o json \
          | jq '.meta.total'

  Read meta.total for a count; a page of rows is not the whole tenant.`,
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

		tenant := resolveTenant(productTypeListTenant, profile)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "product-types commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		validatePCMSLimit(productTypeListLimit)

		resp, err := client.ListProductTypes(ctx, tenant, productTypeListQuery, productTypeListPage, productTypeListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.ProductType]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.ProductType{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.ProductType, len(envelope.Data))
		for i, pt := range envelope.Data {
			items[i] = output.ProductType{
				ID:          pt.ID,
				Name:        pt.Name,
				Description: pt.Description,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product_type",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, productTypeListTenant),
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
// product-types create
// --------------------------------------------------------------------------

var (
	productTypeCreateTenant      string
	productTypeCreateName        string
	productTypeCreateDescription string
	productTypeCreateFromJSON    string
)

var productTypesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new product type",
	Long: `Create a product type.

PURPOSE
  Add a product type to this tenant's reference data.

USAGE
  capigo product-types create --tenant <code>
                              [--name <text> [--description <text>]
                               | --from-json <path|->]

FLAGS
  --tenant <code>
      Tenant to create in. Required.

  --name <text>
      Product type name. Required, unless --from-json is used.

        capigo product-types create --tenant acme --name "Pin Lien Cap"

  --description <text>
      Free-text description. Optional, max 2000 characters.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --description: passing both exits 5.

      Body:

          { "name": "Pin Lien Cap", "description": "Pin va cap lien khoi" }
          { "name": "Pin Lien Cap" }

        echo '{"name":"Pin Lien Cap"}' \
          | capigo product-types create --tenant acme --from-json -

  -o, --output table|json|quiet
      Print the row, the bare created object, or its id. Defaults to table.
      See capigo help output.

OUTPUT
  -o json emits the bare created product type:

      { "id": "3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33",
        "name": "Pin Lien Cap", "description": "Pin va cap lien khoi" }

  quiet prints its id. Output modes and the JSON contract: capigo help output`,
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

		tenant := resolveTenant(productTypeCreateTenant, profile)
		defer echoTenant(tenant, productTypeCreateTenant)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "product-types commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if productTypeCreateFromJSON != "" {
			for _, f := range []string{"name", "description"} {
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
			raw, err := readJSONInput(productTypeCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if productTypeCreateName == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--name is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			req := api.CreateProductTypeRequest{Name: productTypeCreateName}
			if productTypeCreateDescription != "" {
				req.Description = &productTypeCreateDescription
			}
			body = req
		}

		resp, err := client.Do(ctx, "POST", "/pcms/product-types", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.ProductType `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:          envelope.Data.ID,
			Name:        envelope.Data.Name,
			Description: envelope.Data.Description,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

// --------------------------------------------------------------------------
// product-types update
// --------------------------------------------------------------------------

var (
	productTypeUpdateTenant           string
	productTypeUpdateName             string
	productTypeUpdateDescription      string
	productTypeUpdateClearDescription bool
	productTypeUpdateFromJSON         string
)

var productTypesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change some fields of a product type",
	Long: `Update a product type. Fields you do not send are left unchanged.

PURPOSE
  Change part of a product type without deciding every field. To rewrite
  name and description together, forcing a decision on both, use
  product-types replace <id> — the two commands hit the same API behavior;
  replace only adds the CLI-side requirement to touch both fields.

USAGE
  capigo product-types update <id> --tenant <code>
                                   [--name <text>]
                                   [--description <text> | --clear-description]
                                   [--from-json <path|->]

FLAGS
  <id>
      Product type id, a UUID. Positional, required.

  --tenant <code>
      Tenant the product type belongs to. Required.

  --name <text>
      A new name.

  --description <text>
      A new description, max 2000 characters. Mutually exclusive with
      --clear-description.

  --clear-description
      Set description to null. Mutually exclusive with --description.

        capigo product-types update <uuid> --tenant acme --description "Pin roi"
        capigo product-types update <uuid> --tenant acme --clear-description

      At least one field is required; sending none exits 5.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --description and --clear-description: passing
      both exits 5.

  -o, --output table|json|quiet
      Print the row, the bare updated object, or its id. Defaults to table.
      See capigo help output.

OUTPUT
  -o json emits the bare updated product type:

      { "id": "3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33",
        "name": "Pin Lien Cap", "description": "Pin roi" }

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

		tenant := resolveTenant(productTypeUpdateTenant, profile)
		defer echoTenant(tenant, productTypeUpdateTenant)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "product-types commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if productTypeUpdateFromJSON != "" {
			for _, f := range []string{"name", "description", "clear-description"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(productTypeUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			m := map[string]any{}
			if productTypeUpdateName != "" {
				m["name"] = productTypeUpdateName
			}
			if productTypeUpdateClearDescription {
				m["description"] = nil
			} else if productTypeUpdateDescription != "" {
				m["description"] = productTypeUpdateDescription
			}
			if len(m) == 0 {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "at least one field must be provided for update", "")
				os.Exit(5)
			}
			body = m
		}

		resp, err := client.Do(ctx, "PATCH", "/pcms/product-types/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.ProductType `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:          envelope.Data.ID,
			Name:        envelope.Data.Name,
			Description: envelope.Data.Description,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

// --------------------------------------------------------------------------
// product-types get
// --------------------------------------------------------------------------

var productTypeGetTenant string

var productTypesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a product type by id",
	Long: `Get one product type by id.

PURPOSE
  Read a single product type, addressed by id only. To find that id from a
  name, use product-types list --query.

USAGE
  capigo product-types get <id> --tenant <code> [-o table|json|quiet]

FLAGS
  <id>
      Product type id, a UUID. Positional, required.

  --tenant <code>
      Tenant the product type belongs to. Required. Exits 4 if the product
      type is not in it.

        capigo product-types get 3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33 \
            --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table:

      ┌──────────────────────────────────────┬──────────────┐
      │ ID                                   │ Name         │
      ├──────────────────────────────────────┼──────────────┤
      │ 3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33 │ Pin Lien Cap │
      └──────────────────────────────────────┴──────────────┘

  -o json emits the bare object. A get is not a list, so there is no envelope
  and no .data to reach for:

      { "id": "3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33",
        "name": "Pin Lien Cap", "description": "Pin va cap lien khoi" }

  Exit 4 when no such product type exists in the resolved tenant.`,
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

		tenant := resolveTenant(productTypeGetTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "product-types commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		resp, err := client.Do(ctx, "GET", "/pcms/product-types/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.ProductType `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:          envelope.Data.ID,
			Name:        envelope.Data.Name,
			Description: envelope.Data.Description,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

// --------------------------------------------------------------------------
// product-types replace
// --------------------------------------------------------------------------

var (
	productTypeReplaceTenant        string
	productTypeReplaceName          string
	productTypeReplaceDescription   string
	productTypeReplaceNoDescription bool
	productTypeReplaceFromJSON      string
)

var productTypesReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Overwrite every field of a product type",
	Long: `Replace a product type.

PURPOSE
  Rewrite a product type's name and description together in one call. The
  API does not clear an omitted field on this endpoint — like
  product-types update <id>, it changes only what you send. What sets this
  command apart is that it requires --name and one of --description or
  --no-description on every call, so a stale field survives only if you
  typed it that way. To change one field without deciding the other, use
  product-types update <id>.

USAGE
  capigo product-types replace <id> --tenant <code>
                                    (--name <text>
                                     (--description <text> | --no-description)
                                     | --from-json <path|->)

FLAGS
  <id>
      Product type id, a UUID. Positional, required.

  --tenant <code>
      Tenant the product type belongs to. Required.

  --name <text>
      Product type name. Required on every call.

  --description <text>
      The description, max 2000 characters. Mutually exclusive with
      --no-description; exactly one of the two is required on every call.

  --no-description
      Set description to null. Mutually exclusive with --description.

        capigo product-types replace <uuid> --tenant acme --name "Pin" \
            --no-description

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --description and --no-description: passing
      both exits 5.

  -o, --output table|json|quiet
      Print the row, the bare replaced object, or its id. Defaults to table.
      See capigo help output.

OUTPUT
  -o json emits the bare product type as it now stands:

      { "id": "3e91b0a2-4c7d-4f11-9a8e-6d5b1c2f9e33",
        "name": "Pin", "description": null }

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

		tenant := resolveTenant(productTypeReplaceTenant, profile)
		defer echoTenant(tenant, productTypeReplaceTenant)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "product-types commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if productTypeReplaceFromJSON != "" {
			for _, f := range []string{"name", "description", "no-description"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(productTypeReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if productTypeReplaceName == "" {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--name is required for replace", "")
				os.Exit(5)
			}
			descSet := cmd.Flags().Changed("description")
			if descSet && productTypeReplaceNoDescription {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--description and --no-description are mutually exclusive", "")
				os.Exit(5)
			}
			if !descSet && !productTypeReplaceNoDescription {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "one of --description or --no-description is required for replace", "")
				os.Exit(5)
			}
			req := api.ReplaceProductTypeRequest{Name: productTypeReplaceName}
			if !productTypeReplaceNoDescription {
				req.Description = &productTypeReplaceDescription
			}
			body = req
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/product-types/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.ProductType `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:          envelope.Data.ID,
			Name:        envelope.Data.Name,
			Description: envelope.Data.Description,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

func init() {
	productTypesListCmd.Flags().StringVar(&productTypeListTenant, "tenant", "", "tenant code (required)")
	productTypesListCmd.Flags().StringVarP(&productTypeListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	productTypesListCmd.Flags().IntVar(&productTypeListPage, "page", 0, "page number")
	productTypesListCmd.Flags().IntVar(&productTypeListLimit, "limit", 20, "items per page (1-100)")

	productTypesCreateCmd.Flags().StringVar(&productTypeCreateTenant, "tenant", "", "tenant code (required)")
	productTypesCreateCmd.Flags().StringVar(&productTypeCreateName, "name", "", "product type name (required unless --from-json is used)")
	productTypesCreateCmd.Flags().StringVar(&productTypeCreateDescription, "description", "", "product type description (max 2000 chars)")
	productTypesCreateCmd.Flags().StringVar(&productTypeCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateTenant, "tenant", "", "tenant code (required)")
	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateName, "name", "", "new product type name")
	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateDescription, "description", "", "new product type description (max 2000 chars)")
	productTypesUpdateCmd.Flags().BoolVar(&productTypeUpdateClearDescription, "clear-description", false, "set description to null (remove description)")
	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	productTypesGetCmd.Flags().StringVar(&productTypeGetTenant, "tenant", "", "tenant code (required)")

	productTypesReplaceCmd.Flags().StringVar(&productTypeReplaceTenant, "tenant", "", "tenant code (required)")
	productTypesReplaceCmd.Flags().StringVar(&productTypeReplaceName, "name", "", "product type name (required)")
	productTypesReplaceCmd.Flags().StringVar(&productTypeReplaceDescription, "description", "", "product type description (mutually exclusive with --no-description)")
	productTypesReplaceCmd.Flags().BoolVar(&productTypeReplaceNoDescription, "no-description", false, "set description to null (mutually exclusive with --description)")
	productTypesReplaceCmd.Flags().StringVar(&productTypeReplaceFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin); mutually exclusive with individual field flags")

	productTypesCmd.AddCommand(productTypesListCmd, productTypesGetCmd, productTypesCreateCmd, productTypesUpdateCmd, productTypesReplaceCmd)
	rootCmd.AddCommand(productTypesCmd)
}
