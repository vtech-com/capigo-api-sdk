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
tenant, and every response names the tenant it resolved to, in meta.

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

FLAGS
  --tenant <code>
      Tenant to read from. Required.

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

OUTPUT
  The units are at .data[]:

      {
        "data": [
          { "id": "7c1f2e88-0a3d-4f21-9b77-5c1e2a4d9f10", "name": "Kilogram",
            "abbreviation": "kg" },
          { "id": "9ab2c744-1e3a-4b8c-9f10-5c1e2a4d9f10", "name": "Piece",
            "abbreviation": "pc" }
        ],
        "meta": {
          "tenant": "acme", "tenant_source": "flag",
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

		tenant := resolveTenant(unitListTenant, profile)
		requireTenant(tenant, "units commands")

		validatePCMSLimit(unitListLimit)

		resp, err := client.ListUnits(ctx, tenant, unitListQuery, unitListPage, unitListLimit)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, unitListTenant, envelope.Meta))
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
  capigo units get <id> --tenant <code>

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required. Exits 4 if the unit is not in
      it.

        capigo units get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The unit is at .data — an object, where a list puts an array. The envelope
  is the same either way:

      {
        "data": {
          "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
          "name": "Kilogram",
          "abbreviation": "kg"
        },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  A single-item read carries no pagination meta; there is nothing to page.

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
		requireTenant(tenant, "units commands")

		resp, err := client.Do(ctx, "GET", "/pcms/units/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, unitGetTenant))
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
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --abbreviation: passing both exits 5.

      Body:

          { "name": "Kilogram", "abbreviation": "kg" }
          { "name": "Piece", "abbreviation": "pc" }

        echo '{"name":"Piece","abbreviation":"pc"}' \
          | capigo units create --tenant acme --from-json -

OUTPUT
  The created unit is at .data:

      {
        "data": { "id": "...", "name": "Kilogram", "abbreviation": "kg" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the unit was written to. Read it: a write that
  landed in the wrong tenant looks exactly like a write that succeeded.

  Exit 5 if --name or --abbreviation is missing (and --from-json is not
  used), or if --from-json is combined with a field flag. Exit 8 if a unit
  with the same abbreviation already exists in the tenant.`,
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
		requireTenant(tenant, "units commands")

		var body any
		if unitCreateFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(unitCreateFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if unitCreateName == "" {
				failValidation("--name is required")
			}
			if unitCreateAbbreviation == "" {
				failValidation("--abbreviation is required")
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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, unitCreateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
  Change one or a few fields of a unit without restating the rest. This is
  PATCH: the API accepts any subset, so long as it is not empty, and leaves
  every field you do not send unchanged. A unit has no nullable field.

  units replace <id> is the other half of the pair. It sends PUT, which the
  API refuses unless both fields are present.

USAGE
  capigo units update <id> --tenant <code>
                      ([--name <text>] [--abbreviation <text>]
                       | --from-json <path|->)

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required. Exits 4 if the unit is not in
      it.

  --name <text>
      A new name, 1 to 500 characters.

  --abbreviation <text>
      A new abbreviation, 1 to 20 characters. The server lowercases it. An
      abbreviation that duplicates another unit's abbreviation in this
      tenant exits 8.

        capigo units update <uuid> --tenant acme --abbreviation KG

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --abbreviation: passing both exits 5.

OUTPUT
  The unit as it now stands is at .data:

      {
        "data": { "id": "...", "name": "Kilogram", "abbreviation": "kg" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if no field flag is given (and --from-json is not used), or if
  --from-json is combined with a field flag. Exit 4 if <id> is not in the
  resolved tenant. Exit 8 on an abbreviation collision.`,
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
		requireTenant(tenant, "units commands")

		var body any
		if unitUpdateFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
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
				failValidation("at least one field must be provided for update")
			}
			body = m
		}

		resp, err := client.Do(ctx, "PATCH", "/pcms/units/"+id, body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, unitUpdateTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
  Send the unit's whole field set in one call. PUT is a true replace: the API
  requires both fields on the request and rejects one that leaves either out,
  with exit 5 and the message "Required". This command's flags mirror that
  rule — --name and --abbreviation on every call — so an incomplete record is
  refused here rather than at the server. To change one field without retyping
  the other, use units update <id>, which sends PATCH.

USAGE
  capigo units replace <id> --tenant <code>
                       (--name <text> --abbreviation <text>
                        | --from-json <path|->)

FLAGS
  <id>
      Unit id, a UUID. Positional, required.

  --tenant <code>
      Tenant the unit belongs to. Required. Exits 4 if the unit is not in
      it.

  --name <text>
      Unit name, 1 to 500 characters. Required, unless --from-json is used.

  --abbreviation <text>
      Unit abbreviation, 1 to 20 characters. Required, unless --from-json is
      used; the server lowercases it. An abbreviation that duplicates
      another unit's abbreviation in this tenant exits 8.

        capigo units replace <uuid> --tenant acme --name Kilogram \
          --abbreviation kg

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --name and --abbreviation: passing both exits 5.

OUTPUT
  The unit as it now stands is at .data:

      {
        "data": { "id": "...", "name": "Kilogram", "abbreviation": "kg" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if --name or --abbreviation is missing (and --from-json is not
  used), or if --from-json is combined with a field flag. Exit 4 if <id> is
  not in the resolved tenant. Exit 8 on an abbreviation collision.`,
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
		requireTenant(tenant, "units commands")

		var body any
		if unitReplaceFromJSON != "" {
			for _, f := range []string{"name", "abbreviation"} {
				if cmd.Flags().Changed(f) {
					failValidation("--from-json and --%s are mutually exclusive", f)
				}
			}
			raw, err := readJSONInput(unitReplaceFromJSON)
			if err != nil {
				return handleErr(fmt.Errorf("read --from-json: %w", err))
			}
			body = json.RawMessage(raw)
		} else {
			if unitReplaceName == "" {
				failValidation("--name is required for replace")
			}
			if unitReplaceAbbreviation == "" {
				failValidation("--abbreviation is required for replace")
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

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, unitReplaceTenant)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
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
