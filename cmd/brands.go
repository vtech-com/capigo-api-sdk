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
	Long: `Brands, part of the Capigo Product Catalog Management System (PCMS).

Brands are tenant-scoped reference data. Every command here requires a
tenant, and every response names the tenant it resolved to, in meta.

USAGE
  capigo brands <command> --tenant <code> [<args>]`,
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
  Read the brands defined for a tenant, optionally narrowed by name. To
  resolve a name to an id before creating or updating a product, this is the
  command to run. For one brand in full, use brands get.

USAGE
  capigo brands list --tenant <code> [-q <term>] [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to read from. Required.

        capigo brands list --tenant acme

  -q, --query <term>
      Name-contains filter, case-insensitive, max 200 characters. Empty or
      omitted means no filter.

        capigo brands list --tenant acme -q nike

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

        capigo brands list --tenant acme --page 2 --limit 100

OUTPUT
  The brands are at .data[]:

      {
        "data": [
          { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb", "name": "Coolmate",
            "logo_url": null },
          { "id": "8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44", "name": "Nike",
            "logo_url": "https://cdn.capigo.app/b/nike.png" }
        ],
        "meta": {
          "tenant": "acme",
          "tenant_source": "flag",
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

		tenant := resolveTenant(brandListTenant, profile)
		requireTenant(tenant, "brands")

		validatePCMSLimit(brandListLimit)

		resp, err := client.ListBrands(ctx, tenant, brandListQuery, brandListPage, brandListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Brand]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, envelope.Data, listMeta(tenant, brandListTenant, envelope.Meta))
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
  Add a brand to this tenant's reference data. A URL-safe slug is generated
  from the name server-side. Check brands list -q first: the server rejects
  a name whose slug collides with an existing brand.

USAGE
  capigo brands create --tenant <code> (--name <text> [--logo-url <url>]
                       | --from-json <path|->)

FLAGS
  --tenant <code>
      Tenant to create the brand in. Required.

  --name <text>
      Brand name, 1 to 500 characters. Required unless --from-json is used.

        capigo brands create --tenant acme --name Nike

  --logo-url <url>
      Logo URL. Optional; a brand with no logo is created if omitted.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --logo-url: passing both exits 5.

      Body:

          { "name": "Nike", "logo_url": "https://example.com/logo.png" }
          { "name": "No Brand" }

        echo '{"name":"No Brand"}' \
            | capigo brands create --tenant acme --from-json -

OUTPUT
  The created brand is at .data:

      {
        "data": { "id": "8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44",
                  "name": "Nike", "logo_url": null },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the brand was written to. Read it: a write that
  landed in the wrong tenant looks exactly like a write that succeeded.

  Exit 5 if --name is missing (and --from-json is not used), or if
  --from-json is combined with a field flag. Exit 8 if a brand with the same
  name (slug) already exists in the tenant.`,
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
		requireTenant(tenant, "brands")

		var body any
		if brandCreateFromJSON != "" {
			for _, f := range []string{"name", "logo-url"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(brandCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if brandCreateName == "" {
				failValidation("--name is required")
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

		meta := itemMeta(tenant, brandCreateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
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
	Short: "Change some fields of a brand",
	Long: `Update a brand. Fields you do not send are left unchanged.

PURPOSE
  Change one or a few fields of a brand (PATCH) without restating the rest.
  brands replace <id> (PUT) sends the same kind of partial body but the CLI
  requires every field there, so use replace when you want to be forced to
  state the whole record.

USAGE
  capigo brands update <id> --tenant <code>
                       ([--name <text>] [--logo-url <url> | --clear-logo]
                       | --from-json <path|->)
                      

FLAGS
  <id>
      Brand id, a UUID. Positional, required.

  --tenant <code>
      Tenant the brand belongs to. Required. Exits 4 if the brand is not in
      it.

  --name <text>
      New name, 1 to 500 characters. Renaming recomputes the slug, so it can
      collide with another brand's name (exit 8).

        capigo brands update 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --name "Nike Vietnam"

  --logo-url <url>
      New logo URL. Mutually exclusive with --clear-logo.

  --clear-logo
      Set logo_url to null. Mutually exclusive with --logo-url.

        capigo brands update 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --clear-logo

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --logo-url and --clear-logo: passing both
      exits 5.

OUTPUT
  The brand as it now stands is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "name": "Nike Vietnam", "logo_url": null },
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

		tenant := resolveTenant(brandUpdateTenant, profile)
		requireTenant(tenant, "brands")

		var body any
		if brandUpdateFromJSON != "" {
			for _, f := range []string{"name", "logo-url", "clear-logo"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
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
				failValidation("at least one field must be provided for update")
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

		meta := itemMeta(tenant, brandUpdateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
	},
}

// --------------------------------------------------------------------------
// brands get
// --------------------------------------------------------------------------

var brandGetTenant string

var brandsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a brand by id",
	Long: `Get one brand by id.

PURPOSE
  Read a single brand, addressed by id only. To find that id from a name, use
  brands list --query.

USAGE
  capigo brands get <id> --tenant <code>

FLAGS
  <id>
      Brand id, a UUID. Positional, required.

  --tenant <code>
      Tenant the brand belongs to. Required. Exits 4 if the brand is not in it.

        capigo brands get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The brand is at .data — an object, where a list puts an array. The envelope
  is the same either way:

      {
        "data": {
          "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
          "name": "Coolmate",
          "logo_url": "https://cdn.capigo.app/b/coolmate.png"
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  A single-item read carries no pagination meta; there is nothing to page.

  Exit 4 when no such brand exists in the resolved tenant.`,
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
		requireTenant(tenant, "brands")

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

		return output.Write(os.Stdout, envelope.Data, itemMeta(tenant, brandGetTenant))
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
	Short: "Send a brand's full field set at once",
	Long: `Replace a brand by sending every field at once.

PURPOSE
  Send the brand's whole field set in one call (PUT), so nothing is left
  implicit. The API itself does not clear a field you omit on PUT — the same
  as PATCH, it changes only what you send — but this command requires --name
  and a logo decision on every call, so a stale field can only survive if you
  typed it that way on purpose. To touch one field and leave the rest alone
  without restating it, use brands update <id> instead.

USAGE
  capigo brands replace <id> --tenant <code>
                       (--name <text> (--logo-url <url> | --no-logo)
                       | --from-json <path|->)
                      

FLAGS
  <id>
      Brand id, a UUID. Positional, required.

  --tenant <code>
      Tenant the brand belongs to. Required. Exits 4 if the brand is not in
      it.

  --name <text>
      New name, 1 to 500 characters. Required unless --from-json is used.
      Renaming recomputes the slug, so it can collide with another brand's
      name (exit 8).

  --logo-url <url>
      New logo URL. Exactly one of --logo-url and --no-logo is required
      unless --from-json is used; they are mutually exclusive.

  --no-logo
      Set logo_url to null. Mutually exclusive with --logo-url.

        capigo brands replace 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --name Nike --no-logo

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name, --logo-url and --no-logo: passing both exits 5.

OUTPUT
  The brand as it now stands is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "name": "Nike", "logo_url": null },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if --name or a logo flag is missing (and --from-json is not used),
  if both --logo-url and --no-logo are given, or if --from-json is combined
  with a field flag. Exit 4 if <id> is not in the resolved tenant. Exit 8 on
  a name collision.`,
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
		requireTenant(tenant, "brands")

		var body any
		if brandReplaceFromJSON != "" {
			for _, f := range []string{"name", "logo-url", "no-logo"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(brandReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if brandReplaceName == "" {
				failValidation("--name is required for replace")
			}
			logoSet := cmd.Flags().Changed("logo-url")
			if logoSet && brandReplaceNoLogo {
				failValidation("--logo-url and --no-logo are mutually exclusive")
			}
			if !logoSet && !brandReplaceNoLogo {
				failValidation("one of --logo-url or --no-logo is required for replace")
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

		meta := itemMeta(tenant, brandReplaceTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, envelope.Data, meta)
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

	brandsCmd.AddCommand(brandsListCmd, brandsGetCmd, brandsCreateCmd, brandsUpdateCmd, brandsReplaceCmd)
	rootCmd.AddCommand(brandsCmd)
}
