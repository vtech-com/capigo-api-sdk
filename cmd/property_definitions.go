package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

const propDefsPath = "/pcms/property-definitions"

var propertyDefinitionsCmd = &cobra.Command{
	Use:   "property-definitions",
	Short: "Manage PCMS property definitions",
	Long: `Property definitions, part of the Capigo Product Catalog Management System (PCMS).

A property definition is a tenant-wide, ordered key and label that products
carry values for. A product references a definition by key in its properties
map (capigo products get shows .data.properties). Every command here requires
a tenant, and every response names the tenant it resolved to, in meta.

USAGE
  capigo property-definitions <command> --tenant <code> [<args>]`,
}

// propDefCall sends one request and prints the envelope's data as an item or
// a list. Tenant is required, as for every PCMS group.
func propDefCall(tenantFlag, method, path string, body any, list bool) error {
	client, cfg, err := buildClient()
	if err != nil {
		return handleErr(err)
	}
	tenant := resolveTenant(tenantFlag, activeProfileOrEmpty(cfg))
	requireTenant(tenant, "property-definitions commands")

	resp, err := client.Do(context.Background(), method, path, body, tenant)
	if err != nil {
		return handleErr(err)
	}
	var envelope api.RawEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return handleErr(fmt.Errorf("decode response: %w", err))
	}
	meta := itemMeta(tenant, tenantFlag, envelope.Meta)
	meta.ServerTime = resp.ServerTime
	if list {
		return output.Write(os.Stdout, rawList(envelope.Data), meta)
	}
	return output.Write(os.Stdout, rawItem(envelope.Data), meta)
}

var (
	propDefListTenant   string
	propDefGetTenant    string
	propDefCreateTenant string
	propDefCreateLabel  string
	propDefCreateKey    string
	propDefUpdateTenant string
	propDefUpdateLabel  string
	propDefUpdateKey    string
	propDefReplTenant   string
	propDefReplLabel    string
	propDefReplKey      string
	propDefDelTenant    string
	propDefMoveTenant   string
	propDefMoveAfterID  string
	propDefMoveTop      bool
)

var propertyDefinitionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List property definitions",
	Long: `List the property definitions of a tenant, in their display order.

PURPOSE
  Read every property definition. Products reference a definition by its key
  in their properties map; capigo products get shows .data.properties. The
  list is not paged: one call returns all of them.

USAGE
  capigo property-definitions list --tenant <code>

FLAGS
  --tenant <code>
      Tenant to read from. Required.

        capigo property-definitions list --tenant acme

OUTPUT
  The definitions are at .data[], in display order:

      {
        "data": [
          { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb", "key": "material",
            "label": "Material" },
          { "id": "8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44", "key": "origin",
            "label": "Origin" }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  meta.tenant is the tenant this call ran against. See capigo help tenancy.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return propDefCall(propDefListTenant, "GET", propDefsPath, nil, true)
	},
}

var propertyDefinitionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a property definition by id",
	Long: `Get one property definition by id.

PURPOSE
  Read a single definition. Products reference it by key in their properties
  map; capigo products get shows .data.properties.

USAGE
  capigo property-definitions get <id> --tenant <code>

FLAGS
  <id>
      Property definition id, a UUID. Positional, required.

  --tenant <code>
      Tenant the definition belongs to. Required. Exits 4 if it is not in it.

        capigo property-definitions get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The definition is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "key": "material", "label": "Material" },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  Exit 4 when no such definition exists in the resolved tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return propDefCall(propDefGetTenant, "GET", propDefsPath+"/"+url.PathEscape(args[0]), nil, false)
	},
}

var propertyDefinitionsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a property definition",
	Long: `Create a property definition.

PURPOSE
  Add a key and label that products can carry values for. When --key is
  omitted the server generates it from the label. A key is never reused and
  never changes, even after the definition is deleted.

USAGE
  capigo property-definitions create --tenant <code> --label <text> [--key <key>]

FLAGS
  --tenant <code>
      Tenant to create the definition in. Required.

  --label <text>
      Display label, 1 to 50 characters. Required.

  --key <key>
      Key products use to reference the definition: a lowercase letter or
      digit first, then lowercase letters, digits or underscores, 50
      characters at most. Optional.

        capigo property-definitions create --tenant acme --label Material --key material

OUTPUT
  The created definition is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "key": "material", "label": "Material" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  Read .data.key: it is the server's key when --key was omitted. meta.tenant
  is the tenant the definition was written to.

  Exit 5 if --label is missing.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if propDefCreateLabel == "" {
			failValidation("--label is required")
		}
		return propDefCall(propDefCreateTenant, "POST", propDefsPath,
			api.CreatePropertyDefinitionRequest{Label: propDefCreateLabel, Key: propDefCreateKey}, false)
	},
}

const propDefLabelHelp = `OUTPUT
  The definition as it now stands is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "key": "material", "label": "Fabric" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.

  Exit 5 if --label is missing. Exit 4 if <id> is not in the resolved tenant.`

var propertyDefinitionsUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change the label of a property definition",
	Long: `Change the label of a property definition.

PURPOSE
  Set a new label. The relabel reaches every product carrying the definition.
  The key never changes. replace <id> has the same effect; it sends PUT.

USAGE
  capigo property-definitions update <id> --tenant <code> --label <text> [--key <key>]

FLAGS
  <id>
      Property definition id, a UUID. Positional, required.

  --tenant <code>
      Tenant the definition belongs to. Required.

  --label <text>
      New label, 1 to 50 characters. Required.

  --key <key>
      Optional. Must equal the definition's current key; a different key is
      refused.

        capigo property-definitions update 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --label Fabric

` + propDefLabelHelp,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if propDefUpdateLabel == "" {
			failValidation("--label is required")
		}
		return propDefCall(propDefUpdateTenant, "PATCH", propDefsPath+"/"+url.PathEscape(args[0]),
			api.SetPropertyDefinitionLabelRequest{Label: propDefUpdateLabel, Key: propDefUpdateKey}, false)
	},
}

var propertyDefinitionsReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Replace the label of a property definition",
	Long: `Replace the label of a property definition.

PURPOSE
  Set a new label with PUT. The effect is the same as update <id>, kept for
  parity with the other reference-data groups. The relabel reaches every
  product carrying the definition. The key never changes.

USAGE
  capigo property-definitions replace <id> --tenant <code> --label <text> [--key <key>]

FLAGS
  <id>
      Property definition id, a UUID. Positional, required.

  --tenant <code>
      Tenant the definition belongs to. Required.

  --label <text>
      New label, 1 to 50 characters. Required.

  --key <key>
      Optional. Must equal the definition's current key; a different key is
      refused.

        capigo property-definitions replace 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --label Fabric

` + propDefLabelHelp,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		if propDefReplLabel == "" {
			failValidation("--label is required")
		}
		return propDefCall(propDefReplTenant, "PUT", propDefsPath+"/"+url.PathEscape(args[0]),
			api.SetPropertyDefinitionLabelRequest{Label: propDefReplLabel, Key: propDefReplKey}, false)
	},
}

var propertyDefinitionsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a property definition",
	Long: `Soft-delete a property definition.

PURPOSE
  The definition leaves the list. It is never refused for being in use: every
  product carrying it keeps the property as an ad hoc entry, with its key,
  label and value unchanged. The key stays burned and is never reused.

  A repeated delete is exit 4: the first one succeeded, so a script that lost
  the answer and retries should treat exit 4 as done.

USAGE
  capigo property-definitions delete <id> --tenant <code>

FLAGS
  <id>
      Property definition id, a UUID. Positional, required.

  --tenant <code>
      Tenant the definition belongs to. Required.

        capigo property-definitions delete 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The answer names the definition by id, because every read answers exit 4
  once the delete has run:

      { "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  The change reaches every product that carried the definition: its entry
  becomes ad hoc, key, label and value kept.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return propDefCall(propDefDelTenant, "DELETE", propDefsPath+"/"+url.PathEscape(args[0]), nil, false)
	},
}

var propertyDefinitionsMoveCmd = &cobra.Command{
	Use:   "move <id>",
	Short: "Reorder a property definition",
	Long: `Move a property definition within the display order.

PURPOSE
  Put the definition right after another one, or at the top. Exactly one of
  --after-id and --top is required.

USAGE
  capigo property-definitions move <id> --tenant <code> (--after-id <uuid> | --top)

FLAGS
  <id>
      Property definition id, a UUID. Positional, required.

  --tenant <code>
      Tenant the definition belongs to. Required.

  --after-id <uuid>
      Id of the live definition to land behind.

  --top
      Move to the top of the order. Mutually exclusive with --after-id.

        capigo property-definitions move 8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44 \
            --tenant acme --after-id 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb

OUTPUT
  The server's answer is at .data, with the tenant in meta:

      { "data": { "id": "8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44",
                  "key": "origin", "label": "Origin" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  Run list to read the new order.

  Exit 5 if neither or both of --after-id and --top are given.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		afterSet := cmd.Flags().Changed("after-id")
		if afterSet == propDefMoveTop {
			failValidation("exactly one of --after-id or --top is required")
		}
		req := api.MovePropertyDefinitionRequest{}
		if afterSet {
			req.AfterID = &propDefMoveAfterID
		}
		return propDefCall(propDefMoveTenant, "POST", propDefsPath+"/"+url.PathEscape(args[0])+"/actions/move", req, false)
	},
}

func init() {
	for _, p := range []struct {
		c *cobra.Command
		t *string
	}{
		{propertyDefinitionsListCmd, &propDefListTenant},
		{propertyDefinitionsGetCmd, &propDefGetTenant},
		{propertyDefinitionsCreateCmd, &propDefCreateTenant},
		{propertyDefinitionsUpdateCmd, &propDefUpdateTenant},
		{propertyDefinitionsReplaceCmd, &propDefReplTenant},
		{propertyDefinitionsDeleteCmd, &propDefDelTenant},
		{propertyDefinitionsMoveCmd, &propDefMoveTenant},
	} {
		p.c.Flags().StringVar(p.t, "tenant", "", "tenant code (required)")
	}

	propertyDefinitionsCreateCmd.Flags().StringVar(&propDefCreateLabel, "label", "", "display label, 1-50 chars (required)")
	propertyDefinitionsCreateCmd.Flags().StringVar(&propDefCreateKey, "key", "", "key; generated from the label when omitted")
	propertyDefinitionsUpdateCmd.Flags().StringVar(&propDefUpdateLabel, "label", "", "new label, 1-50 chars (required)")
	propertyDefinitionsUpdateCmd.Flags().StringVar(&propDefUpdateKey, "key", "", "must equal the current key")
	propertyDefinitionsReplaceCmd.Flags().StringVar(&propDefReplLabel, "label", "", "new label, 1-50 chars (required)")
	propertyDefinitionsReplaceCmd.Flags().StringVar(&propDefReplKey, "key", "", "must equal the current key")
	propertyDefinitionsMoveCmd.Flags().StringVar(&propDefMoveAfterID, "after-id", "", "id of the definition to land behind")
	propertyDefinitionsMoveCmd.Flags().BoolVar(&propDefMoveTop, "top", false, "move to the top")

	propertyDefinitionsCmd.AddCommand(propertyDefinitionsListCmd, propertyDefinitionsGetCmd, propertyDefinitionsCreateCmd,
		propertyDefinitionsUpdateCmd, propertyDefinitionsReplaceCmd, propertyDefinitionsDeleteCmd, propertyDefinitionsMoveCmd)
	rootCmd.AddCommand(propertyDefinitionsCmd)
}
