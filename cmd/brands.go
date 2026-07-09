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
	Long: `Manage brands in the Capigo Product Catalog Management System (PCMS).

Brands are tenant-scoped reference data. Every command here requires a tenant.
  capigo help tenancy`,
}

// --------------------------------------------------------------------------
// brands list
// --------------------------------------------------------------------------

var (
	brandListTenant string
	brandListQuery  string
	brandListPage   int
	brandListLimit  int
)

var brandsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List brands",
	Long: `List brands.

PURPOSE
  Read the brands defined for a tenant, optionally narrowed by name.

INPUT
  --tenant <code>        required
  -q, --query <term>     name-contains filter, case-insensitive, max 200 chars
  --page <n>             page number
  --limit <n>            items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Each row is:

      { id, name, logo_url }

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo brands list --tenant acme -q nike

  # How many are there? Read meta.total rather than counting rows
  capigo brands list --tenant acme --limit 1 -o json | jq '.meta.total'

SEE ALSO
  brands get <id>       one brand in full
  brands create         add a brand
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

		tenant := resolveTenant(brandListTenant, profile)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "brands commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		validatePCMSLimit(brandListLimit)

		resp, err := client.ListBrands(ctx, tenant, brandListQuery, brandListPage, brandListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Brand]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Brand{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
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
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, brandListTenant),
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
// brands create
// --------------------------------------------------------------------------

var (
	brandCreateTenant   string
	brandCreateName     string
	brandCreateLogoURL  string
	brandCreateFromJSON string
)

var brandsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new brand",
	Long: `Create a brand.

PURPOSE
  Add a brand to this tenant's reference data.

INPUT
  --tenant <code>        required
  --name <text>          required, unless --from-json is used
  --logo-url <url>       optional

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

  Body:

      { "name": "Nike", "logo_url": "https://example.com/logo.png" }
      { "name": "No Brand" }

OUTPUT
  -o json emits the bare created brand:

      { id, name, logo_url }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo brands create --tenant acme --name Nike

  echo '{"name":"No Brand"}' | capigo brands create --tenant acme --from-json -

SEE ALSO
  brands update <id>    change some of its fields later
  brands list           check whether it already exists
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

		tenant := resolveTenant(brandCreateTenant, profile)
		defer echoTenant(tenant, brandCreateTenant)
		if tenant == nil {
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

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
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
	brandUpdateTenant    string
	brandUpdateName      string
	brandUpdateLogoURL   string
	brandUpdateClearLogo bool
	brandUpdateFromJSON  string
)

var brandsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Partial update of an existing brand (PATCH)",
	Long: `Update a brand. Fields you do not send are left unchanged.

PURPOSE
  Change part of a brand (PATCH). To overwrite every field at once, use
  brands replace <id>.

INPUT
  <id>                   brand UUID (positional, required)
  --tenant <code>        required
  --name <text>          a new name
  --logo-url <url>       a new logo URL
  --clear-logo           set logo_url to null

  At least one field is required; sending none exits 5.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare updated brand:

      { id, name, logo_url }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo brands update <uuid> --tenant acme --name "Nike Vietnam"
  capigo brands update <uuid> --tenant acme --clear-logo

SEE ALSO
  brands replace <id>   overwrite every field instead
  brands get <id>       read the current values first
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

		tenant := resolveTenant(brandUpdateTenant, profile)
		defer echoTenant(tenant, brandUpdateTenant)
		if tenant == nil {
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

		resp, err := client.Do(ctx, "PATCH", "/pcms/brands/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Brand `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Brand{
			ID:      envelope.Data.ID,
			Name:    envelope.Data.Name,
			LogoURL: envelope.Data.LogoURL,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "brand"})
	},
}

// --------------------------------------------------------------------------
// brands get
// --------------------------------------------------------------------------

var brandGetTenant string

var brandsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a brand by ID",
	Long: `Get one brand by UUID.

PURPOSE
  Read a single brand. This command addresses it by UUID only. To find that
  UUID from a name, use brands list --query.

INPUT
  <id>                   brand UUID (positional, required)
  --tenant <code>        required

OUTPUT
  -o json emits the bare brand object:

      { id, name, logo_url }

  Exit 4 when no such brand exists in the resolved tenant.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo brands get <uuid> --tenant acme

SEE ALSO
  brands list           find a brand by name
  brands update <id>    change some of its fields
  brands replace <id>   overwrite all of its fields
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

		tenant := resolveTenant(brandGetTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "brands commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		resp, err := client.Do(ctx, "GET", "/pcms/brands/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Brand `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Brand{
			ID:      envelope.Data.ID,
			Name:    envelope.Data.Name,
			LogoURL: envelope.Data.LogoURL,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "brand"})
	},
}

// --------------------------------------------------------------------------
// brands replace
// --------------------------------------------------------------------------

var (
	brandReplaceTenant   string
	brandReplaceName     string
	brandReplaceLogoURL  string
	brandReplaceNoLogo   bool
	brandReplaceFromJSON string
)

var brandsReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Full replace of a brand (PUT)",
	Long: `Replace a brand. Every field is overwritten.

PURPOSE
  Overwrite a brand in full (PUT). A field you do not send is not preserved —
  it is reset. To change one field and keep the rest, use brands update <id>.

INPUT
  <id>                   brand UUID (positional, required)
  --tenant <code>        required
  --name <text>          required
  --logo-url <url>       the logo URL
  --no-logo              set logo_url to null

  Exactly one of --logo-url and --no-logo must be given; they are mutually
  exclusive; replace always writes the logo. To change one field and keep the
  rest, use brands update <id>.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare brand as it now stands:

      { id, name, logo_url }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo brands replace <uuid> --tenant acme --name Nike --no-logo

SEE ALSO
  brands update <id>    change one field and keep the rest
  brands get <id>       read the current values first
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

		tenant := resolveTenant(brandReplaceTenant, profile)
		defer echoTenant(tenant, brandReplaceTenant)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "brands commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if brandReplaceFromJSON != "" {
			for _, f := range []string{"name", "logo-url", "no-logo"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(brandReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if brandReplaceName == "" {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--name is required for replace", "")
				os.Exit(5)
			}
			logoSet := cmd.Flags().Changed("logo-url")
			if logoSet && brandReplaceNoLogo {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--logo-url and --no-logo are mutually exclusive", "")
				os.Exit(5)
			}
			if !logoSet && !brandReplaceNoLogo {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "one of --logo-url or --no-logo is required for replace", "")
				os.Exit(5)
			}
			req := api.ReplaceBrandRequest{Name: brandReplaceName}
			if !brandReplaceNoLogo {
				req.LogoURL = &brandReplaceLogoURL
			}
			body = req
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

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Brand{
			ID:      envelope.Data.ID,
			Name:    envelope.Data.Name,
			LogoURL: envelope.Data.LogoURL,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "brand"})
	},
}

func init() {
	brandsListCmd.Flags().StringVar(&brandListTenant, "tenant", "", "tenant code (required)")
	brandsListCmd.Flags().StringVarP(&brandListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	brandsListCmd.Flags().IntVar(&brandListPage, "page", 0, "page number")
	brandsListCmd.Flags().IntVar(&brandListLimit, "limit", 20, "items per page (1-100)")

	brandsCreateCmd.Flags().StringVar(&brandCreateTenant, "tenant", "", "tenant code (required)")
	brandsCreateCmd.Flags().StringVar(&brandCreateName, "name", "", "brand name (required unless --from-json is used)")
	brandsCreateCmd.Flags().StringVar(&brandCreateLogoURL, "logo-url", "", "brand logo URL")
	brandsCreateCmd.Flags().StringVar(&brandCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	brandsUpdateCmd.Flags().StringVar(&brandUpdateTenant, "tenant", "", "tenant code (required)")
	brandsUpdateCmd.Flags().StringVar(&brandUpdateName, "name", "", "new brand name")
	brandsUpdateCmd.Flags().StringVar(&brandUpdateLogoURL, "logo-url", "", "new brand logo URL")
	brandsUpdateCmd.Flags().BoolVar(&brandUpdateClearLogo, "clear-logo", false, "set logo_url to null (remove logo)")
	brandsUpdateCmd.Flags().StringVar(&brandUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	brandsGetCmd.Flags().StringVar(&brandGetTenant, "tenant", "", "tenant code (required)")

	brandsReplaceCmd.Flags().StringVar(&brandReplaceTenant, "tenant", "", "tenant code (required)")
	brandsReplaceCmd.Flags().StringVar(&brandReplaceName, "name", "", "brand name (required)")
	brandsReplaceCmd.Flags().StringVar(&brandReplaceLogoURL, "logo-url", "", "brand logo URL (mutually exclusive with --no-logo)")
	brandsReplaceCmd.Flags().BoolVar(&brandReplaceNoLogo, "no-logo", false, "set logo_url to null (mutually exclusive with --logo-url)")
	brandsReplaceCmd.Flags().StringVar(&brandReplaceFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin); mutually exclusive with individual field flags")

	brandsCmd.AddCommand(brandsListCmd, brandsCreateCmd, brandsUpdateCmd, brandsGetCmd, brandsReplaceCmd)
	rootCmd.AddCommand(brandsCmd)
}
