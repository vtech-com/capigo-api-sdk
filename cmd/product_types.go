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
	Long: `List product types from the PCMS catalog. Tenant is required.

Use --query / -q for a name-contains search (case-insensitive, max 200 chars).
Each product type in the response has: id, name, description (string or null).`,
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
				Shown:   len(envelope.Data),
				Page:    envelope.Meta.Page,
				Limit:   envelope.Meta.Limit,
				Total:   envelope.Meta.Total,
				HasMore: envelope.Meta.HasMore,
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
	Long: `Create a new product type in PCMS. Tenant is required.

Provide --name and optional --description, or supply the full request body
with --from-json <file> (use - to read from stdin). When --from-json is
set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Smartphone" }
  { "name": "Laptop", "description": "Portable computers" }

Response: { "data": { "id": "uuid", "name": "string", "description": "string|null" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
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
	Long: `Partial update (PATCH) of an existing product type in PCMS. Tenant is required.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged on the server. Use --clear-description to explicitly set
description to null.

Use --from-json to supply the full update body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "New Name" }
  { "description": "Updated description" }
  { "description": null }

Response: { "data": { "id": "uuid", "name": "string", "description": "string|null" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
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
// product-types get
// --------------------------------------------------------------------------

var productTypeGetTenant string

var productTypesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a product type by ID",
	Long: `Get a single product type by ID from PCMS. Tenant is required.

Returns 404 for both not-found and cross-tenant resources (no info leakage).
Response: { "data": { "id": "uuid", "name": "string", "description": "string|null" } }`,
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
	Long: `Full replace (PUT) of an existing product type in PCMS. Tenant is required.

All fields are required by the server. You must provide either --description <text>
or --no-description (to set description to null); these flags are mutually exclusive.

Use --from-json to supply the full request body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Smartphone", "description": "Mobile phones and tablets" }
  { "name": "Laptop", "description": null }

Response: { "data": { "id": "uuid", "name": "string", "description": "string|null" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
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
