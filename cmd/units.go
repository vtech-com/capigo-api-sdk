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

var unitsCmd = &cobra.Command{
	Use:   "units",
	Short: "Manage PCMS units",
	Long: `Units of measure for products, in the Capigo Product Catalog Management
System (PCMS).

Units are tenant-scoped reference data. Every command here requires a
tenant.
  capigo help tenancy

USAGE
  capigo units <command> --tenant <code> [<args>]`,
}

// --------------------------------------------------------------------------
// units list
// --------------------------------------------------------------------------

var (
	unitListTenant string
	unitListQuery  string
	unitListPage   int
	unitListLimit  int
)

var unitsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List units",
	Long: `List units.

PURPOSE
  Read the units defined for a tenant, optionally narrowed by name. To read
  a single unit whose id you already have, use units get.

USAGE
  capigo units list --tenant <code> [-q <term>] [--page <n>] [--limit <n>]
                    [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to read from. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

        capigo units list --tenant acme

  -q, --query <term>
      Name-contains filter, case-insensitive, up to 200 characters.

        capigo units list --tenant acme -q kilo

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Rows per page, 1 to 100. Defaults to 20.

        capigo units list --tenant acme --page 2 --limit 100

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of units, then a summary line. Ids are shortened here to fit the
  page; the command prints them in full.

      ┌──────────┬──────────┬──────────────┐
      │ ID       │ Name     │ Abbreviation │
      ├──────────┼──────────┼──────────────┤
      │ 7c1f2e88 │ Kilogram │ kg           │
      │ 9ab2c744 │ Piece    │ pc           │
      └──────────┴──────────┴──────────────┘
      Tenant: acme · Total: 2 (all rows shown)

  -o json emits the list envelope; the units are at .data[]:

      {
        "data": [
          { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10", "name": "Kilogram",
            "abbreviation": "kg" },
          { "id": "9ab2c744-1e3a-4b8c-9f10-5c1e2a4d9f10", "name": "Piece",
            "abbreviation": "pc" }
        ],
        "meta": { "page": 1, "limit": 20, "total": 2, "has_more": false }
      }`,
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

		tenant := resolveTenant(unitListTenant, profile)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "units commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		validatePCMSLimit(unitListLimit)

		resp, err := client.ListUnits(ctx, tenant, unitListQuery, unitListPage, unitListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Unit]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Unit{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Unit, len(envelope.Data))
		for i, u := range envelope.Data {
			items[i] = output.Unit{
				ID:           u.ID,
				Name:         u.Name,
				Abbreviation: u.Abbreviation,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   false,
			ResourceKind: "unit",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, unitListTenant),
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
// units get
// --------------------------------------------------------------------------

var unitGetTenant string

var unitsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a unit by id",
	Long: `Get one unit by id.

PURPOSE
  Read a single unit, addressed by id only. To find that id from a name, use
  units list --query.

USAGE
  capigo units get <id> --tenant <code> [-o table|json|quiet]

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required. Exits 4 if the unit is not in
      it.

        capigo units get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table:

      ┌──────────────────────────────────────┬──────────┬──────────────┐
      │ ID                                   │ Name     │ Abbreviation │
      ├──────────────────────────────────────┼──────────┼──────────────┤
      │ 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb │ Kilogram │ kg           │
      └──────────────────────────────────────┴──────────┴──────────────┘

  -o json emits the bare object. A get is not a list, so there is no envelope
  and no .data to reach for:

      {
        "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
        "name": "Kilogram",
        "abbreviation": "kg"
      }

  Exit 4 when no such unit exists in the resolved tenant.`,
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

		tenant := resolveTenant(unitGetTenant, profile)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "units commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		resp, err := client.Do(ctx, "GET", "/pcms/units/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Unit `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Unit{
			ID:           envelope.Data.ID,
			Name:         envelope.Data.Name,
			Abbreviation: envelope.Data.Abbreviation,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "unit"})
	},
}

// --------------------------------------------------------------------------
// units create
// --------------------------------------------------------------------------

var (
	unitCreateTenant       string
	unitCreateName         string
	unitCreateAbbreviation string
	unitCreateFromJSON     string
)

var unitsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new unit",
	Long: `Create a unit.

PURPOSE
  Add a unit to this tenant's reference data.

USAGE
  capigo units create --tenant <code>
                      (--name <text> --abbreviation <text>
                       | --from-json <path|->)
                      [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to create in. Required.

        capigo units create --tenant acme --name Kilogram --abbreviation kg

  --name <text>
      Unit name, 1 to 500 characters. Required, unless --from-json is used.

  --abbreviation <text>
      Unit abbreviation, 1 to 20 characters, e.g. kg. Required, unless
      --from-json is used. The server lowercases it. An abbreviation that
      duplicates an existing unit's abbreviation in this tenant exits 8.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. The
      individual field flags above are ignored when this is set.

      Body:

          { "name": "Kilogram", "abbreviation": "kg" }
          { "name": "Piece", "abbreviation": "pc" }

        echo '{"name":"Piece","abbreviation":"pc"}' \
          | capigo units create --tenant acme --from-json -

  -o, --output table|json|quiet
      Print the created row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare created unit:

      { "id": "...", "name": "Kilogram", "abbreviation": "kg" }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output`,
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

		tenant := resolveTenant(unitCreateTenant, profile)
		defer echoTenant(tenant, unitCreateTenant)
		if tenant == nil {
			e := &api.APIError{
				Code:       "VALIDATION_ERROR",
				Message:    "units commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		var body any
		if unitCreateFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
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
			raw, err := readJSONInput(unitCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if unitCreateName == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--name is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			if unitCreateAbbreviation == "" {
				e := &api.APIError{Code: "VALIDATION_ERROR", Message: "--abbreviation is required", HTTPStatus: 400}
				output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
				os.Exit(api.ExitCodeFor(e))
			}
			body = api.CreateUnitRequest{
				Name:         unitCreateName,
				Abbreviation: unitCreateAbbreviation,
			}
		}

		resp, err := client.Do(ctx, "POST", "/pcms/units", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Unit `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Unit{
			ID:           envelope.Data.ID,
			Name:         envelope.Data.Name,
			Abbreviation: envelope.Data.Abbreviation,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "unit"})
	},
}

// --------------------------------------------------------------------------
// units update
// --------------------------------------------------------------------------

var (
	unitUpdateTenant       string
	unitUpdateName         string
	unitUpdateAbbreviation string
	unitUpdateFromJSON     string
)

var unitsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change some fields of a unit",
	Long: `Update a unit. Fields you do not send are left unchanged.

PURPOSE
  Change part of a unit: its name, or its abbreviation. To overwrite every
  field at once, use units replace <id>. A unit has no nullable field.

USAGE
  capigo units update <id> --tenant <code>
                      ([--name <text>] [--abbreviation <text>]
                       | --from-json <path|->)
                      [-o table|json|quiet]

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required.

  --name <text>
      A new name, 1 to 500 characters.

  --abbreviation <text>
      A new abbreviation, 1 to 20 characters. The server lowercases it. An
      abbreviation that duplicates another unit's abbreviation in this
      tenant exits 8.

        capigo units update <uuid> --tenant acme --abbreviation KG

  At least one of --name, --abbreviation is required; sending neither
  exits 5.

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with the individual field flags above: passing both exits 5.

  -o, --output table|json|quiet
      Print the updated row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare updated unit:

      { "id": "...", "name": "Kilogram", "abbreviation": "kg" }

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

		tenant := resolveTenant(unitUpdateTenant, profile)
		defer echoTenant(tenant, unitUpdateTenant)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "units commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if unitUpdateFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(unitUpdateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			m := map[string]any{}
			if unitUpdateName != "" {
				m["name"] = unitUpdateName
			}
			if unitUpdateAbbreviation != "" {
				m["abbreviation"] = unitUpdateAbbreviation
			}
			if len(m) == 0 {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "at least one field must be provided for update", "")
				os.Exit(5)
			}
			body = m
		}

		resp, err := client.Do(ctx, "PATCH", "/pcms/units/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Unit `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Unit{
			ID:           envelope.Data.ID,
			Name:         envelope.Data.Name,
			Abbreviation: envelope.Data.Abbreviation,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "unit"})
	},
}

// --------------------------------------------------------------------------
// units replace
// --------------------------------------------------------------------------

var (
	unitReplaceTenant       string
	unitReplaceName         string
	unitReplaceAbbreviation string
	unitReplaceFromJSON     string
)

var unitsReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Overwrite every field of a unit",
	Long: `Replace a unit's name and abbreviation in one call.

PURPOSE
  Rewrite a unit's name and abbreviation together in one call. The API does
  not clear a field you omit on PUT — like PATCH, it changes only what you
  send — but this command requires --name and --abbreviation on every call,
  so a stale field survives only if you typed it that way. To change one
  field without retyping the rest, use units update <id>.

USAGE
  capigo units replace <id> --tenant <code>
                       (--name <text> --abbreviation <text>
                        | --from-json <path|->)
                       [-o table|json|quiet]

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required.

  --name <text>
      Unit name, 1 to 500 characters. Required.

  --abbreviation <text>
      Unit abbreviation, 1 to 20 characters. Required; the server lowercases
      it. An abbreviation that duplicates another unit's abbreviation in
      this tenant exits 8.

        capigo units replace <uuid> --tenant acme --name Kilogram \
          --abbreviation kg

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with the individual field flags above: passing both exits 5.

  -o, --output table|json|quiet
      Print the replaced row, the JSON object, or its bare id. Defaults to
      table. See capigo help output.

OUTPUT
  -o json emits the bare unit as it now stands:

      { "id": "...", "name": "Kilogram", "abbreviation": "kg" }

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

		tenant := resolveTenant(unitReplaceTenant, profile)
		defer echoTenant(tenant, unitReplaceTenant)
		if tenant == nil {
			output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "units commands require a tenant; pass --tenant <code> or set default", "")
			os.Exit(5)
		}

		var body any
		if unitReplaceFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
				if cmd.Flags().Changed(f) {
					output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", fmt.Sprintf("--from-json and --%s are mutually exclusive", f), "")
					os.Exit(5)
				}
			}
			raw, err := readJSONInput(unitReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if unitReplaceName == "" {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--name is required for replace", "")
				os.Exit(5)
			}
			if unitReplaceAbbreviation == "" {
				output.RenderError(os.Stderr, outputMode, "VALIDATION_ERROR", "--abbreviation is required for replace", "")
				os.Exit(5)
			}
			body = api.ReplaceUnitRequest{
				Name:         unitReplaceName,
				Abbreviation: unitReplaceAbbreviation,
			}
		}

		resp, err := client.Do(ctx, "PUT", "/pcms/units/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope struct {
			Data api.Unit `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		emitServerTime(resp.ServerTime, "")

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Unit{
			ID:           envelope.Data.ID,
			Name:         envelope.Data.Name,
			Abbreviation: envelope.Data.Abbreviation,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "unit"})
	},
}

func init() {
	unitsListCmd.Flags().StringVar(&unitListTenant, "tenant", "", "tenant code (required)")
	unitsListCmd.Flags().StringVarP(&unitListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	unitsListCmd.Flags().IntVar(&unitListPage, "page", 0, "page number")
	unitsListCmd.Flags().IntVar(&unitListLimit, "limit", 20, "items per page (1-100)")

	unitsCreateCmd.Flags().StringVar(&unitCreateTenant, "tenant", "", "tenant code (required)")
	unitsCreateCmd.Flags().StringVar(&unitCreateName, "name", "", "unit name (required unless --from-json is used)")
	unitsCreateCmd.Flags().StringVar(&unitCreateAbbreviation, "abbreviation", "", "unit abbreviation, e.g. kg (required unless --from-json is used)")
	unitsCreateCmd.Flags().StringVar(&unitCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	unitsUpdateCmd.Flags().StringVar(&unitUpdateTenant, "tenant", "", "tenant code (required)")
	unitsUpdateCmd.Flags().StringVar(&unitUpdateName, "name", "", "new unit name")
	unitsUpdateCmd.Flags().StringVar(&unitUpdateAbbreviation, "abbreviation", "", "new unit abbreviation")
	unitsUpdateCmd.Flags().StringVar(&unitUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	unitsGetCmd.Flags().StringVar(&unitGetTenant, "tenant", "", "tenant code (required)")

	unitsReplaceCmd.Flags().StringVar(&unitReplaceTenant, "tenant", "", "tenant code (required)")
	unitsReplaceCmd.Flags().StringVar(&unitReplaceName, "name", "", "unit name (required)")
	unitsReplaceCmd.Flags().StringVar(&unitReplaceAbbreviation, "abbreviation", "", "unit abbreviation, e.g. kg (required)")
	unitsReplaceCmd.Flags().StringVar(&unitReplaceFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin); mutually exclusive with individual field flags")

	unitsCmd.AddCommand(unitsListCmd, unitsGetCmd, unitsCreateCmd, unitsUpdateCmd, unitsReplaceCmd)
	rootCmd.AddCommand(unitsCmd)
}
