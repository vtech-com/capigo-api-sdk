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
	Long: `Manage units in the Capigo Product Catalog Management System (PCMS).

Units are tenant-scoped reference data. Every command here requires a tenant.
  capigo help tenancy`,
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
  Read the units defined for a tenant, optionally narrowed by name.

INPUT
  --tenant <code>        required
  -q, --query <term>     name-contains filter, case-insensitive, max 200 chars
  --page <n>             page number
  --limit <n>            items per page, 1-100 (default 20)

OUTPUT
  -o json emits the list envelope. Each row is:

      { id, name, abbreviation }

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo units list --tenant acme -q kg

  # How many are there? Read meta.total rather than counting rows
  capigo units list --tenant acme --limit 1 -o json | jq '.meta.total'

SEE ALSO
  units get <id>       one unit in full
  units create         add a unit
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

INPUT
  --tenant <code>        required
  --name <text>          required, unless --from-json is used
  --abbreviation <text>  required, unless --from-json is used, e.g. kg

  The server lowercases the abbreviation.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

  Body:

      { "name": "Kilogram", "abbreviation": "kg" }
      { "name": "Piece", "abbreviation": "pc" }

OUTPUT
  -o json emits the bare created unit:

      { id, name, abbreviation }

  quiet prints its id.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo units create --tenant acme --name Kilogram --abbreviation kg

  echo '{"name":"Piece","abbreviation":"pc"}' \
    | capigo units create --tenant acme --from-json -

SEE ALSO
  units update <id>    change some of its fields later
  units list           check whether it already exists
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
	Short: "Partial update of an existing unit (PATCH)",
	Long: `Update a unit. Fields you do not send are left unchanged.

PURPOSE
  Change part of a unit (PATCH). To overwrite every field at once, use
  units replace <id>.

INPUT
  <id>                   unit UUID (positional, required)
  --tenant <code>        required
  --name <text>          a new name
  --abbreviation <text>  a new abbreviation; the server lowercases it

  A unit has no nullable field.

  At least one field is required; sending none exits 5.

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare updated unit:

      { id, name, abbreviation }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo units update <uuid> --tenant acme --abbreviation KG

SEE ALSO
  units replace <id>   overwrite every field instead
  units get <id>       read the current values first
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
// units get
// --------------------------------------------------------------------------

var unitGetTenant string

var unitsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a unit by ID",
	Long: `Get one unit by UUID.

PURPOSE
  Read a single unit. This command addresses it by UUID only. To find that
  UUID from a name, use units list --query.

INPUT
  <id>                   unit UUID (positional, required)
  --tenant <code>        required

OUTPUT
  -o json emits the bare unit object:

      { id, name, abbreviation }

  Exit 4 when no such unit exists in the resolved tenant.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo units get <uuid> --tenant acme

SEE ALSO
  units list           find a unit by name
  units update <id>    change some of its fields
  units replace <id>   overwrite all of its fields
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
	Short: "Full replace of a unit (PUT)",
	Long: `Replace a unit. Every field is overwritten.

PURPOSE
  Overwrite a unit in full (PUT). A field you do not send is not preserved —
  it is reset. To change one field and keep the rest, use units update <id>.

INPUT
  <id>                   unit UUID (positional, required)
  --tenant <code>        required
  --name <text>          required
  --abbreviation <text>  required; the server lowercases it

  Or --from-json <path|-> to send the whole body, where - reads stdin.
  --from-json and the individual field flags are MUTUALLY EXCLUSIVE: passing
  both exits 5.

OUTPUT
  -o json emits the bare unit as it now stands:

      { id, name, abbreviation }

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo units replace <uuid> --tenant acme --name Kilogram --abbreviation kg

SEE ALSO
  units update <id>    change one field and keep the rest
  units get <id>       read the current values first
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

	unitsCmd.AddCommand(unitsListCmd, unitsCreateCmd, unitsUpdateCmd, unitsGetCmd, unitsReplaceCmd)
	rootCmd.AddCommand(unitsCmd)
}
