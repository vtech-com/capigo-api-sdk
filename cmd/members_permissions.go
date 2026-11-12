package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var memberPermissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Add or remove a member's tenant permission keys",
	Long: `A member's tenant permission keys: the ones that hold across the whole tenant,
such as catalog_admin, po:view or wms_admin.

add and remove change only the keys named and leave every other key as it is,
in one atomic step, so two callers changing different keys both land. members
update --permission replaces the whole set instead. Access to one warehouse is
not a tenant key: wms warehouses members manage that. capigo permissions list
--scope-type tenant prints the keys these commands accept.

USAGE
  capigo members permissions <add|remove> <user-id> --tenant <code>
                             --permission <key>...`,
}

type memberPermissionsFlags struct {
	tenant      string
	permissions []string
}

var (
	memberPermissionsAdd    memberPermissionsFlags
	memberPermissionsRemove memberPermissionsFlags
)

// newMemberPermissionsCommand builds add or remove; the two differ only in the
// verb the route ends with and in what a key already in that state means.
func newMemberPermissionsCommand(change string, f *memberPermissionsFlags, purpose, repeat string) *cobra.Command {
	subject := "members permissions " + change
	return &cobra.Command{
		Use:   change + " <user-id>",
		Short: strings.ToUpper(change[:1]) + change[1:] + " tenant permission keys on a member",
		Long: fill(`{Verb} tenant permission keys on one member, leaving every other key as it is.

PURPOSE
  {Purpose}
  Only an active tenant owner may call it, and never on an owner. For access
  to one warehouse, use wms warehouses members instead.

USAGE
  capigo members permissions {change} <user-id> --tenant <code>
          --permission <key>...

FLAGS
  <user-id>
      The member's user id, as members list prints it at .data[].id.
      Positional, required.

  --tenant <code>
      Tenant the member belongs to. Required.

  --permission <key>   (repeatable)
      A key to {change}, 1 to 200 in all: a key whose scope_type is tenant in
      capigo permissions list. One unknown key refuses the whole call with
      exit 5 and changes nothing.
      {Repeat}

        capigo members permissions {change} 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
          --tenant acme --permission catalog_view --permission po:view

OUTPUT
  The member's stored state is at .data, with every key it now holds, sorted:

      { "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "role": "member", "status": "active",
                  "display_name": "Tram Nguyen", "member_code": "EMP-001",
                  "job_title": "Picker", "department": null, "bio": null,
                  "permissions": ["catalog_view", "po:view"], "positions": [] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-11-12T03:54:51Z" } }

  Exit 3 when the caller is not an active owner of the tenant. Exit 4 when the
  member is unknown, removed or in another tenant. Exit 8 when the member is an
  owner (PERMISSIONS_NOT_APPLICABLE): an owner already holds everything.`,
			"Verb", strings.ToUpper(change[:1])+change[1:],
			"Purpose", purpose,
			"Repeat", repeat,
			"change", change,
		),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			userID := strings.TrimSpace(args[0])
			if userID == "" {
				failValidation("%s: <user-id> must not be blank", subject)
			}
			keys := trimmedValues(f.permissions)
			if len(keys) == 0 {
				failValidation("%s: give at least one --permission; capigo permissions list --scope-type tenant prints the keys", subject)
			}

			client, cfg, err := buildClient()
			if err != nil {
				return handleErr(err)
			}
			tenant := resolveTenant(f.tenant, activeProfileOrEmpty(cfg))
			requireTenant(tenant, subject)

			resp, err := client.ChangeMemberPermissions(context.Background(), userID, change,
				api.MemberPermissionsRequest{Permissions: keys}, tenant)
			if err != nil {
				return handleErr(err)
			}
			data, apiMeta := api.SplitAnswer(resp.Body)
			meta := itemMeta(tenant, f.tenant, apiMeta)
			meta.ServerTime = resp.ServerTime
			return output.Write(os.Stdout, rawItem(data), meta)
		},
	}
}

// trimmedValues drops blank entries from a repeatable flag, so --permission ""
// is caught here rather than sent as a key the API will refuse.
func trimmedValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

var (
	membersPermissionsAddCmd = newMemberPermissionsCommand("add", &memberPermissionsAdd,
		"Grant tenant keys without reading the member first.",
		"A key the member already holds is not an error: the call answers\n      with the stored state and writes nothing.")
	membersPermissionsRemoveCmd = newMemberPermissionsCommand("remove", &memberPermissionsRemove,
		"Revoke tenant keys without reading the member first.",
		"A key the member does not hold is not an error: the call answers\n      with the stored state and writes nothing.")
)

func init() {
	for _, c := range []struct {
		cmd *cobra.Command
		f   *memberPermissionsFlags
	}{{membersPermissionsAddCmd, &memberPermissionsAdd}, {membersPermissionsRemoveCmd, &memberPermissionsRemove}} {
		c.cmd.Flags().StringVar(&c.f.tenant, "tenant", "", "tenant the member belongs to (required)")
		c.cmd.Flags().StringArrayVar(&c.f.permissions, "permission", nil, "tenant permission key; repeatable")
	}

	memberPermissionsCmd.AddCommand(membersPermissionsAddCmd, membersPermissionsRemoveCmd)
	memberCmd.AddCommand(memberPermissionsCmd)
}
