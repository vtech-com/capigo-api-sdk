package cmd

import (
	"context"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Look up the permission keys and warehouse scopes",
	Long: `The catalog of every permission key the API accepts.

Two kinds of key live here, told apart by scope_type. A tenant key is held in a
member's permission set across the whole tenant: members permissions add and
remove change those. A warehouse scope (scope.*) is held in one warehouse only:
wms warehouses members change those. Read the catalog instead of hard-coding
keys: a key added later appears here.

USAGE
  capigo permissions <command> [<args>]`,
}

var (
	permissionsListModule    string
	permissionsListScopeType string
)

var permissionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every permission key and warehouse scope",
	Long: `List every permission key and warehouse scope, with what each one includes.

PURPOSE
  Find the exact key to grant before calling members permissions add or wms
  warehouses members scopes add, and see what a key already includes so you
  do not grant it twice. The list is the same for every caller and needs no
  tenant.

USAGE
  capigo permissions list [--module catalog|po|wms]
                          [--scope-type tenant|warehouse]

FLAGS
  --module catalog|po|wms
      Keep only the keys of one module. The filter runs in the CLI: the API
      always answers the whole list.

        capigo permissions list --module po

  --scope-type tenant|warehouse
      Keep only tenant keys, or only warehouse scopes.

        capigo permissions list --module wms --scope-type warehouse

OUTPUT
  The keys are at .data[]. implies lists every key this one already includes,
  transitively, and is [] when none:

      { "data": [
          { "key": "wms_admin", "label": "WMS Admin",
            "description": "Full administrative access to all warehouses in the workspace, bypassing per-warehouse scopes",
            "module": "wms", "scope_type": "tenant",
            "implies": ["scope.admin", "scope.view", "scope.inbound", "…"] },
          { "key": "scope.inbound", "label": "Inbound",
            "description": "Inbound receipts and their putaway plans. …",
            "module": "wms", "scope_type": "warehouse",
            "implies": ["scope.view"] } ],
        "meta": {} }

  Treat key as opaque: never build one from its parts. Exit 5 when --module
  or --scope-type names a value outside the lists above.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		module := strings.TrimSpace(permissionsListModule)
		if module != "" && !slices.Contains(api.PermissionModules, module) {
			failValidation("permissions list: --module must be one of %s", strings.Join(api.PermissionModules, ", "))
		}
		scopeType := strings.TrimSpace(permissionsListScopeType)
		if scopeType != "" && !slices.Contains(api.PermissionScopeTypes, scopeType) {
			failValidation("permissions list: --scope-type must be one of %s", strings.Join(api.PermissionScopeTypes, ", "))
		}

		client, _, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.ListPermissions(context.Background())
		if err != nil {
			return handleErr(err)
		}
		data, _ := api.SplitAnswer(resp.Body)
		kept, err := api.FilterPermissions(data, module, scopeType)
		if err != nil {
			return handleErr(err)
		}
		return output.Write(os.Stdout, rawList(kept), output.Meta{})
	},
}

func init() {
	permissionsListCmd.Flags().StringVar(&permissionsListModule, "module", "", "catalog, po or wms")
	permissionsListCmd.Flags().StringVar(&permissionsListScopeType, "scope-type", "", "tenant or warehouse")

	permissionsCmd.AddCommand(permissionsListCmd)
	rootCmd.AddCommand(permissionsCmd)
}
