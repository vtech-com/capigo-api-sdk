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

var brandsCmd = &cobra.Command{
	Use:   "brands",
	Short: "Manage PCMS brands",
}

// --------------------------------------------------------------------------
// brands list
// --------------------------------------------------------------------------

var (
	brandListQuery string
	brandListPage  int
	brandListLimit int
)

var brandsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List brands",
	Long: `List brands from the PCMS catalog.

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
				Message:    "brands commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListBrands(ctx, tenant, brandListQuery, brandListPage, brandListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Brand]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		items := make([]output.Brand, len(envelope.Data))
		for i, b := range envelope.Data {
			items[i] = output.Brand{
				ID:      b.ID,
				Name:    b.Name,
				LogoURL: b.LogoURL,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "brand",
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
// brands create
// --------------------------------------------------------------------------

var (
	brandCreateName     string
	brandCreateLogoURL  string
	brandCreateFromJSON string
)

var brandsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new brand",
	Long: `Create a new brand in PCMS.

Provide --name and optional --logo-url, or supply the full request body
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
				Message:    "brands commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if brandCreateFromJSON != "" {
			for _, f := range []string{"name", "logo-url"} {
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
			raw, err := readJSONInput(brandCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if brandCreateName == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--name is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			req := api.CreateBrandRequest{Name: brandCreateName}
			if brandCreateLogoURL != "" {
				req.LogoURL = &brandCreateLogoURL
			}
			body = req
		}

		resp, err := client.Do(ctx, "POST", "/pcms/brands", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Brand `json:"data"`
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

		return output.Render(os.Stdout, outputMode, output.Brand{
			ID:      envelope.Data.ID,
			Name:    envelope.Data.Name,
			LogoURL: envelope.Data.LogoURL,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "brand"})
	},
}

// --------------------------------------------------------------------------
// brands update
// --------------------------------------------------------------------------

var (
	brandUpdateName      string
	brandUpdateLogoURL   string
	brandUpdateClearLogo bool
	brandUpdateFromJSON  string
)

var brandsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing brand",
	Long: `Update an existing brand in PCMS.

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
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "brands commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if brandUpdateFromJSON != "" {
			for _, f := range []string{"name", "logo-url", "clear-logo"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(brandUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			m := map[string]any{}
			if brandUpdateName != "" {
				m["name"] = brandUpdateName
			}
			if brandUpdateClearLogo {
				m["logo_url"] = nil
			} else if brandUpdateLogoURL != "" {
				m["logo_url"] = brandUpdateLogoURL
			}
			if len(m) == 0 {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "at least one field must be provided for update", "")
				os.Exit(5)
			}
			body = m
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/brands/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Brand `json:"data"`
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

		return output.Render(os.Stdout, outputMode, output.Brand{
			ID:      envelope.Data.ID,
			Name:    envelope.Data.Name,
			LogoURL: envelope.Data.LogoURL,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "brand"})
	},
}

func init() {
	brandsListCmd.Flags().StringVarP(&brandListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	brandsListCmd.Flags().IntVar(&brandListPage, "page", 1, "page number")
	brandsListCmd.Flags().IntVar(&brandListLimit, "limit", 20, "items per page (1-100)")

	brandsCreateCmd.Flags().StringVar(&brandCreateName, "name", "", "brand name (required unless --from-json is used)")
	brandsCreateCmd.Flags().StringVar(&brandCreateLogoURL, "logo-url", "", "brand logo URL")
	brandsCreateCmd.Flags().StringVar(&brandCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	brandsUpdateCmd.Flags().StringVar(&brandUpdateName, "name", "", "new brand name")
	brandsUpdateCmd.Flags().StringVar(&brandUpdateLogoURL, "logo-url", "", "new brand logo URL")
	brandsUpdateCmd.Flags().BoolVar(&brandUpdateClearLogo, "clear-logo", false, "set logo_url to null (remove logo)")
	brandsUpdateCmd.Flags().StringVar(&brandUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	brandsCmd.AddCommand(brandsListCmd, brandsCreateCmd, brandsUpdateCmd)
	rootCmd.AddCommand(brandsCmd)
}
