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
	productTypeListQuery string
	productTypeListPage  int
	productTypeListLimit int
)

var productTypesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List product types",
	Long: `List product types from the PCMS catalog.

Use --query / -q for a name-contains search.`,
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

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "product-types commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListProductTypes(ctx, tenant, productTypeListQuery, productTypeListPage, productTypeListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.ProductType]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.ProductType, len(envelope.Data))
		for i, pt := range envelope.Data {
			items[i] = output.ProductType{
				ID:   pt.ID,
				Name: pt.Name,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "product_type",
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
// product-types create
// --------------------------------------------------------------------------

var (
	productTypeCreateName        string
	productTypeCreateDescription string
	productTypeCreateFromJSON    string
)

var productTypesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new product type",
	Long: `Create a new product type in PCMS.

Provide --name and optional --description, or supply the full request body
with --from-json <file> (use - to read from stdin). When --from-json is
set, all individual field flags are ignored.`,
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

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:   envelope.Data.ID,
			Name: envelope.Data.Name,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

// --------------------------------------------------------------------------
// product-types update
// --------------------------------------------------------------------------

var (
	productTypeUpdateName        string
	productTypeUpdateDescription string
	productTypeUpdateFromJSON    string
)

var productTypesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing product type",
	Long: `Update an existing product type in PCMS.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged on the server.

Use --from-json to supply the full update body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.`,
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

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "product-types commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if productTypeUpdateFromJSON != "" {
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
			raw, err := readJSONInput(productTypeUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			req := api.UpdateProductTypeRequest{}
			fieldCount := 0
			if productTypeUpdateName != "" {
				req.Name = &productTypeUpdateName
				fieldCount++
			}
			if productTypeUpdateDescription != "" {
				req.Description = &productTypeUpdateDescription
				fieldCount++
			}
			if fieldCount == 0 {
				e := &api.APIError{
					Code:       "VALIDATION_ERROR",
					Message:    "at least one field must be provided for update",
					HTTPStatus: 400,
				}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.ProductType{
			ID:   envelope.Data.ID,
			Name: envelope.Data.Name,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "product_type"})
	},
}

func init() {
	productTypesListCmd.Flags().StringVarP(&productTypeListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	productTypesListCmd.Flags().IntVar(&productTypeListPage, "page", 1, "page number")
	productTypesListCmd.Flags().IntVar(&productTypeListLimit, "limit", 20, "items per page (1-100)")

	productTypesCreateCmd.Flags().StringVar(&productTypeCreateName, "name", "", "product type name (required unless --from-json is used)")
	productTypesCreateCmd.Flags().StringVar(&productTypeCreateDescription, "description", "", "product type description (max 2000 chars)")
	productTypesCreateCmd.Flags().StringVar(&productTypeCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateName, "name", "", "new product type name")
	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateDescription, "description", "", "new product type description (max 2000 chars)")
	productTypesUpdateCmd.Flags().StringVar(&productTypeUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	productTypesCmd.AddCommand(productTypesListCmd, productTypesCreateCmd, productTypesUpdateCmd)
	rootCmd.AddCommand(productTypesCmd)
}
