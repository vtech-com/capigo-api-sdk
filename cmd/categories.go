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
	categoryListQuery string
	categoryListPage  int
	categoryListLimit int
)

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List categories",
	Long: `List categories from the PCMS catalog.

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
				Message:    "categories commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListCategories(ctx, tenant, categoryListQuery, categoryListPage, categoryListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Category]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
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
	categoryCreateName     string
	categoryCreateParentID string
	categoryCreateFromJSON string
)

var categoriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new category",
	Long: `Create a new category in PCMS.

Provide --name and optional --parent-id, or supply the full request body
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
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
	categoryUpdateName     string
	categoryUpdateParentID string
	categoryUpdateFromJSON string
)

var categoriesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing category",
	Long: `Update an existing category in PCMS.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged. Pass --parent-id "" (empty string via --from-json null)
to promote a category to root.

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
				Message:    "categories commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if categoryUpdateFromJSON != "" {
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
			raw, err := readJSONInput(categoryUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			req := api.UpdateCategoryRequest{}
			fieldCount := 0
			if categoryUpdateName != "" {
				req.Name = &categoryUpdateName
				fieldCount++
			}
			if categoryUpdateParentID != "" {
				req.ParentID = &categoryUpdateParentID
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Category{
			ID:       envelope.Data.ID,
			Name:     envelope.Data.Name,
			ParentID: envelope.Data.ParentID,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "category"})
	},
}

func init() {
	categoriesListCmd.Flags().StringVarP(&categoryListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	categoriesListCmd.Flags().IntVar(&categoryListPage, "page", 1, "page number")
	categoriesListCmd.Flags().IntVar(&categoryListLimit, "limit", 20, "items per page (1-100)")

	categoriesCreateCmd.Flags().StringVar(&categoryCreateName, "name", "", "category name (required unless --from-json is used)")
	categoriesCreateCmd.Flags().StringVar(&categoryCreateParentID, "parent-id", "", "parent category UUID (omit for root)")
	categoriesCreateCmd.Flags().StringVar(&categoryCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateName, "name", "", "new category name")
	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateParentID, "parent-id", "", "new parent category UUID")
	categoriesUpdateCmd.Flags().StringVar(&categoryUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	categoriesCmd.AddCommand(categoriesListCmd, categoriesCreateCmd, categoriesUpdateCmd)
	rootCmd.AddCommand(categoriesCmd)
}
