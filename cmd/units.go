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
}

// --------------------------------------------------------------------------
// units list
// --------------------------------------------------------------------------

var (
	unitListQuery string
	unitListPage  int
	unitListLimit int
)

var unitsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List units",
	Long: `List product units from the PCMS catalog. Tenant is required.

Use --query / -q for a name-contains search (case-insensitive, max 200 chars).
Each unit in the response has: id, name, abbreviation.`,
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
				Message:    "units commands require a tenant; pass --tenant <code> or set default",
				HTTPStatus: 400,
			}
			output.RenderError(os.Stderr, outputMode, e.Code, e.Message, "")
			os.Exit(api.ExitCodeFor(e))
		}

		resp, err := client.ListUnits(ctx, tenant, unitListQuery, unitListPage, unitListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.Envelope[[]api.Unit]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
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
			if envelope.Meta.HasMore {
				fmt.Fprintf(os.Stderr, "Showing %d of %d. Use --page / --limit to paginate.\n",
					len(envelope.Data), envelope.Meta.Total)
			}
		}

		return nil
	},
}

// --------------------------------------------------------------------------
// units create
// --------------------------------------------------------------------------

var (
	unitCreateName         string
	unitCreateAbbreviation string
	unitCreateFromJSON     string
)

var unitsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new unit",
	Long: `Create a new product unit in PCMS. Tenant is required.

Both --name and --abbreviation are required, or supply the full request body
with --from-json <file> (use - to read from stdin). When --from-json is
set, all individual field flags are ignored. Abbreviation is normalized to
lowercase by the server.

JSON body (--from-json):
  { "name": "Kilogram", "abbreviation": "kg" }
  { "name": "Piece", "abbreviation": "pc" }

Response: { "data": { "id": "uuid", "name": "string", "abbreviation": "string" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
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
	unitUpdateName         string
	unitUpdateAbbreviation string
	unitUpdateFromJSON     string
)

var unitsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Partial update of an existing unit (PATCH)",
	Long: `Partial update (PATCH) of an existing product unit in PCMS. Tenant is required.

All fields are optional; at least one must be provided. Fields not specified
are left unchanged on the server.

Use --from-json to supply the full update body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Kilogram" }
  { "abbreviation": "kg" }
  { "name": "Kilogram", "abbreviation": "kg" }

Response: { "data": { "id": "uuid", "name": "string", "abbreviation": "string" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
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

var unitsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a unit by ID",
	Long: `Get a single product unit by ID from PCMS. Tenant is required.

Returns 404 for both not-found and cross-tenant resources (no info leakage).
Response: { "data": { "id": "uuid", "name": "string", "abbreviation": "string" } }`,
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

		tenant, isGlobal := resolveTenant(profile)

		_ = api.PCMSRequiresTenant
		if isGlobal {
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
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
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
	unitReplaceName         string
	unitReplaceAbbreviation string
	unitReplaceFromJSON     string
)

var unitsReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Full replace of a unit (PUT)",
	Long: `Full replace (PUT) of an existing product unit in PCMS. Tenant is required.

All fields are required by the server: --name and --abbreviation must both be provided.

Use --from-json to supply the full request body as JSON (file path or - for
stdin). When --from-json is set, all individual field flags are ignored.

JSON body (--from-json):
  { "name": "Kilogram", "abbreviation": "kg" }

Response: { "data": { "id": "uuid", "name": "string", "abbreviation": "string" } }`,
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

		if resp.ServerTime != "" {
			fmt.Fprintf(os.Stderr, "Server time: %s\n", resp.ServerTime)
		}

		if outputMode == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope.Data)
		}

		return output.Render(os.Stdout, outputMode, output.Unit{
			ID:           envelope.Data.ID,
			Name:         envelope.Data.Name,
			Abbreviation: envelope.Data.Abbreviation,
		}, output.RenderOpts{GlobalMode: false, ResourceKind: "unit"})
	},
}

func init() {
	unitsListCmd.Flags().StringVarP(&unitListQuery, "query", "q", "", "name-contains filter (case-insensitive, max 200 chars)")
	unitsListCmd.Flags().IntVar(&unitListPage, "page", 1, "page number")
	unitsListCmd.Flags().IntVar(&unitListLimit, "limit", 20, "items per page (1-100)")

	unitsCreateCmd.Flags().StringVar(&unitCreateName, "name", "", "unit name (required unless --from-json is used)")
	unitsCreateCmd.Flags().StringVar(&unitCreateAbbreviation, "abbreviation", "", "unit abbreviation, e.g. kg (required unless --from-json is used)")
	unitsCreateCmd.Flags().StringVar(&unitCreateFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin)")

	unitsUpdateCmd.Flags().StringVar(&unitUpdateName, "name", "", "new unit name")
	unitsUpdateCmd.Flags().StringVar(&unitUpdateAbbreviation, "abbreviation", "", "new unit abbreviation")
	unitsUpdateCmd.Flags().StringVar(&unitUpdateFromJSON, "from-json", "", "path to JSON file with update body (use - for stdin); mutually exclusive with individual field flags")

	unitsReplaceCmd.Flags().StringVar(&unitReplaceName, "name", "", "unit name (required)")
	unitsReplaceCmd.Flags().StringVar(&unitReplaceAbbreviation, "abbreviation", "", "unit abbreviation, e.g. kg (required)")
	unitsReplaceCmd.Flags().StringVar(&unitReplaceFromJSON, "from-json", "", "path to JSON file with full request body (use - for stdin); mutually exclusive with individual field flags")

	unitsCmd.AddCommand(unitsListCmd, unitsCreateCmd, unitsUpdateCmd, unitsGetCmd, unitsReplaceCmd)
	rootCmd.AddCommand(unitsCmd)
}
