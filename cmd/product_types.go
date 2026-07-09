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
	Long: `Manage product-types in the Capigo Product Catalog Management System (PCMS).

Product-types are tenant-scoped reference data. Every command here requires a tenant.
  capigo help tenancy`,
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
	Long: `List product-types.

PURPOSE
  Read the product-types defined for a tenant, optionally narrowed by name.

INPUT
  --tenant <code>        required
  -q, --query <term>     name-contains filter, case-insensitive, max 200 chars
  --page <n>             page number
  --limit <n>            items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Each row is:

      { id, name, description }

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo product-types list --tenant acme -q pin

  # How many are there? Read meta.total rather than counting rows
  capigo product-types list --tenant acme --limit 1 -o json | jq '.meta.total'

SEE ALSO
  product-types get <id>       one product type in full
  product-types create         add a product type
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

INPUT
  --tenant <code>        required
  --name <text>          required, unless --from-json is used
  --description <text>   optional, max 2000 characters

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

  Body:

      { "name": "Pin Lien Cap", "description": "Pin va cap lien khoi" }
      { "name": "Pin Lien Cap" }

OUTPUT
  -o json emits the bare created product type:

      { id, name, description }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo product-types create --tenant acme --name "Pin Lien Cap"

  echo '{"name":"Pin Lien Cap"}' \
    | capigo product-types create --tenant acme --from-json -

SEE ALSO
  product-types update <id>    change some of its fields later
  product-types list           check whether it already exists
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
	Short: "Partial update of an existing product type (PATCH)",
	Long: `Update a product type. Fields you do not send are left unchanged.

PURPOSE
  Change part of a product type (PATCH). To overwrite every field at once, use
  product-types replace <id>.

INPUT
  <id>                   product type UUID (positional, required)
  --tenant <code>        required
  --name <text>          a new name
  --description <text>   a new description, max 2000 characters
  --clear-description    set description to null

  At least one field is required; sending none exits 5.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare updated product type:

      { id, name, description }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo product-types update <uuid> --tenant acme --description "Pin roi"
  capigo product-types update <uuid> --tenant acme --clear-description

SEE ALSO
  product-types replace <id>   overwrite every field instead
  product-types get <id>       read the current values first
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
	Short: "Get a product type by ID",
	Long: `Get one product type by UUID.

PURPOSE
  Read a single product type. This command addresses it by UUID only. To find that
  UUID from a name, use product-types list --query.

INPUT
  <id>                   product type UUID (positional, required)
  --tenant <code>        required

OUTPUT
  -o json emits the bare product type object:

      { id, name, description }

  Exit 4 when no such product type exists in the resolved tenant.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo product-types get <uuid> --tenant acme

SEE ALSO
  product-types list           find a product type by name
  product-types update <id>    change some of its fields
  product-types replace <id>   overwrite all of its fields
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
	Short: "Full replace of a product type (PUT)",
	Long: `Replace a product type. Every field is overwritten.

PURPOSE
  Overwrite a product type in full (PUT). A field you do not send is not preserved —
  it is reset. To change one field and keep the rest, use product-types update <id>.

INPUT
  <id>                   product type UUID (positional, required)
  --tenant <code>        required
  --name <text>          required
  --description <text>   the description, max 2000 characters
  --no-description       set description to null

  Exactly one of --description and --no-description must be given; they are
  mutually exclusive. There is no way to leave the description untouched here —
  that is what product-types update is for.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare product type as it now stands:

      { id, name, description }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo product-types replace <uuid> --tenant acme --name "Pin" --no-description

SEE ALSO
  product-types update <id>    change one field and keep the rest
  product-types get <id>       read the current values first
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

	productTypesCmd.AddCommand(productTypesListCmd, productTypesCreateCmd, productTypesUpdateCmd, productTypesGetCmd, productTypesReplaceCmd)
	rootCmd.AddCommand(productTypesCmd)
}
