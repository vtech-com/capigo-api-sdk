package cmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

var wmsWarehouseMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "Who may work in a warehouse, and with which scopes",
	Long: `Warehouse memberships: which tenant members may work in one warehouse, and
the scopes (scope.*) they hold there.

A membership is addressed by its own id, the .data[].id that members list
prints, not by the member's user id; user_id sits beside it. Only add takes a
user id, because the membership does not exist yet. Scopes hold in this
warehouse only. A tenant key such as wms_admin already covers every warehouse
and is changed with members permissions instead. capigo permissions list
--scope-type warehouse prints the scopes and what each includes.

Owners, holders of wms_admin and holders of scope.admin in the warehouse may
manage memberships.

USAGE
  capigo wms warehouses members <command> --tenant <code> --warehouse <code> [<args>]`,
}

const wmsMemberIDFlagHelp = `  <member-id>
      Warehouse membership id, as wms warehouses members list prints it at
      .data[].id. Not the user id. Positional, required.`

const wmsScopeFlagHelp = `  --scope <scope>   (repeatable)
      A warehouse scope, such as scope.inbound, from capigo permissions list
      (scope_type warehouse). One unknown scope refuses the whole call with
      exit 5 and changes nothing. A scope another one already includes is
      dropped when stored: scope.inbound already includes scope.view.`

const wmsMemberOutputSample = `      { "data": { "id": "6f0c2a9e-5b1d-4c7a-9e3f-2d8b7a1c4e55",
                  "user_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "scopes": ["scope.inbound", "scope.outbound"],
                  "status": "active",
                  "created_at": "2026-11-12T03:54:51.120Z",
                  "updated_at": "2026-11-12T03:54:51.120Z" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-11-12T03:54:51Z" } }`

const wmsMemberAccessExits = `  Exit 3 when the caller may not manage this warehouse's memberships. Exit 4
  when the warehouse is not visible to the caller or the membership is not in
  this warehouse.`

var wmsWarehouseMembersList wmsFlags

var wmsWarehouseMembersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the memberships of a warehouse",
	Long: `List who may work in a warehouse, with the scopes each one holds there.

PURPOSE
  Find a membership id before changing its scopes or removing it, and see what
  each member already holds. A caller who may not manage the warehouse sees
  only their own membership.

USAGE
  capigo wms warehouses members list --tenant <code> --warehouse <code>
          [--page <n>] [--limit <n>]

FLAGS
` + wmsTenantFlagHelp + `

` + wmsWarehouseFlagHelp + `

        capigo wms warehouses members list --tenant acme --warehouse WH-HN

` + wmsPagingFlagHelp + `

OUTPUT
  The memberships are at .data[]; read meta.total for the full count. id is the
  membership id the other commands here take:

      { "data": [
          { "id": "6f0c2a9e-5b1d-4c7a-9e3f-2d8b7a1c4e55",
            "user_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
            "scopes": ["scope.inbound", "scope.outbound"],
            "status": "active",
            "created_at": "2026-11-12T03:54:51.120Z",
            "updated_at": "2026-11-12T03:54:51.120Z" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false } }

  scopes holds what was granted, not what it includes: read implies in
  capigo permissions list for that. Members who reach the warehouse through
  a tenant key such as wms_admin, or as owner, are not listed.

` + wmsMemberAccessExits,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		f := &wmsWarehouseMembersList
		subject := "wms warehouses members list"
		warehouse := requireWarehouse(f.warehouse, subject)
		validateWMSLimit(f.limit)

		client, tenant, err := wmsSession(f.tenant, subject)
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.ListWarehouseMembers(context.Background(), warehouse, f.page, f.limit, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSList(resp, tenant, f.tenant)
	},
}

var (
	wmsWarehouseMembersAdd       wmsFlags
	wmsWarehouseMembersAddUserID string
	wmsWarehouseMembersAddScopes []string
)

var wmsWarehouseMembersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Give a tenant member access to a warehouse",
	Long: `Give one tenant member access to a warehouse, with the scopes they start with.

PURPOSE
  Let someone work in a warehouse. To change the scopes of someone who is
  already a member, use wms warehouses members scopes add or remove: add
  refuses an existing membership. A membership removed earlier comes back with
  its old id and the scopes given here.

USAGE
  capigo wms warehouses members add --tenant <code> --warehouse <code>
          --user-id <uuid> [--scope <scope>]...

FLAGS
` + wmsTenantFlagHelp + `

` + wmsWarehouseFlagHelp + `

  --user-id <uuid>
      The member's user id, as members list prints it at .data[].id. Required.
      The person must be an active member of the tenant.

` + wmsScopeFlagHelp + `
      With no --scope the member gets scope.view.

        capigo wms warehouses members add --tenant acme --warehouse WH-HN \
          --user-id 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
          --scope scope.inbound --scope scope.outbound

OUTPUT
  The new membership is at .data. Keep .data.id: the other commands here take
  it:

` + wmsMemberOutputSample + `

  Exit 4 also when the user is not an active member of the tenant. Exit 8 when
  the member already belongs to the warehouse; nothing is changed.

` + wmsMemberAccessExits,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		f := &wmsWarehouseMembersAdd
		subject := "wms warehouses members add"
		warehouse := requireWarehouse(f.warehouse, subject)
		userID := strings.TrimSpace(wmsWarehouseMembersAddUserID)
		if userID == "" {
			failValidation("%s: --user-id <uuid> is required; members list prints the user ids", subject)
		}

		client, tenant, err := wmsSession(f.tenant, subject)
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.AddWarehouseMember(context.Background(), warehouse, api.WarehouseMemberCreateRequest{
			UserID: userID, Scopes: trimmedValues(wmsWarehouseMembersAddScopes),
		}, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSWrite(resp, tenant, f.tenant, false)
	},
}

var wmsWarehouseMembersScopesCmd = &cobra.Command{
	Use:   "scopes",
	Short: "Add or remove scopes on a warehouse membership",
	Long: `Change the scopes of one warehouse membership, leaving every other scope as it
is. Each call is one atomic step, so two callers changing different scopes
both land.

USAGE
  capigo wms warehouses members scopes <add|remove> <member-id>
          --tenant <code> --warehouse <code> --scope <scope>...`,
}

type wmsMemberScopesFlags struct {
	wmsFlags
	scopes []string
}

var (
	wmsWarehouseMembersScopesAdd    wmsMemberScopesFlags
	wmsWarehouseMembersScopesRemove wmsMemberScopesFlags
)

// newWMSMemberScopesCommand builds scopes add or remove; the two differ only in
// the verb the route ends with and in what an unchanged scope means.
func newWMSMemberScopesCommand(change string, f *wmsMemberScopesFlags, purpose, repeat string) *cobra.Command {
	subject := "wms warehouses members scopes " + change
	return &cobra.Command{
		Use:   change + " <member-id>",
		Short: strings.ToUpper(change[:1]) + change[1:] + " scopes on a warehouse membership",
		Long: fill(`{Verb} warehouse scopes on one membership, leaving every other scope as it is.

PURPOSE
  {Purpose}

USAGE
  capigo wms warehouses members scopes {change} <member-id> --tenant <code>
          --warehouse <code> --scope <scope>...

FLAGS
`+wmsMemberIDFlagHelp+`

`+wmsTenantFlagHelp+`

`+wmsWarehouseFlagHelp+`

`+wmsScopeFlagHelp+`
      At least one is required. {Repeat}

        capigo wms warehouses members scopes {change} 6f0c2a9e-5b1d-4c7a-9e3f-2d8b7a1c4e55 \
          --tenant acme --warehouse WH-HN --scope scope.outbound

OUTPUT
  The membership, with every scope it now holds, is at .data:

`+wmsMemberOutputSample+`

`+wmsMemberAccessExits,
			"Verb", strings.ToUpper(change[:1])+change[1:],
			"Purpose", purpose,
			"Repeat", repeat,
			"change", change,
		),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			memberID := strings.TrimSpace(args[0])
			if memberID == "" {
				failValidation("%s: <member-id> must not be blank", subject)
			}
			warehouse := requireWarehouse(f.warehouse, subject)
			scopes := trimmedValues(f.scopes)
			if len(scopes) == 0 {
				failValidation("%s: give at least one --scope; capigo permissions list --scope-type warehouse prints them", subject)
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.ChangeWarehouseMemberScopes(context.Background(), warehouse, memberID, change,
				api.WarehouseMemberScopesRequest{Scopes: scopes}, tenant)
			if err != nil {
				return handleErr(err)
			}
			return writeWMSWrite(resp, tenant, f.tenant, false)
		},
	}
}

var (
	wmsWarehouseMembersScopesAddCmd = newWMSMemberScopesCommand("add", &wmsWarehouseMembersScopesAdd,
		"Grant more scopes to someone who already works in the warehouse. To let\n  someone in, use wms warehouses members add.",
		"A scope already held is not an error: nothing\n      is written.")
	wmsWarehouseMembersScopesRemoveCmd = newWMSMemberScopesCommand("remove", &wmsWarehouseMembersScopesRemove,
		"Take scopes away while keeping the person in the warehouse. To take them\n  out of it, use wms warehouses members remove.",
		"A scope not held is not an error. Removing the last\n      scope leaves scope.view.")
)

var wmsWarehouseMembersRemove wmsFlags

var wmsWarehouseMembersRemoveCmd = &cobra.Command{
	Use:   "remove <member-id>",
	Short: "Take a member's access to a warehouse away",
	Long: `Take one member's access to a warehouse away, with every scope they held there.

PURPOSE
  End a warehouse membership. The person stays a tenant member, and keeps
  their tenant keys and their other warehouses. Adding them back restores the
  same membership id.

USAGE
  capigo wms warehouses members remove <member-id> --tenant <code>
          --warehouse <code>

FLAGS
` + wmsMemberIDFlagHelp + `

` + wmsTenantFlagHelp + `

` + wmsWarehouseFlagHelp + `

        capigo wms warehouses members remove 6f0c2a9e-5b1d-4c7a-9e3f-2d8b7a1c4e55 \
          --tenant acme --warehouse WH-HN

OUTPUT
  The id of the removed membership is at .data:

      { "data": { "id": "6f0c2a9e-5b1d-4c7a-9e3f-2d8b7a1c4e55" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-11-12T03:54:51Z" } }

  Exit 3 also when the membership is the caller's own. Exit 4 when it was
  already removed.

` + wmsMemberAccessExits,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		f := &wmsWarehouseMembersRemove
		subject := "wms warehouses members remove"
		memberID := strings.TrimSpace(args[0])
		if memberID == "" {
			failValidation("%s: <member-id> must not be blank", subject)
		}
		warehouse := requireWarehouse(f.warehouse, subject)

		client, tenant, err := wmsSession(f.tenant, subject)
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.RemoveWarehouseMember(context.Background(), warehouse, memberID, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSWrite(resp, tenant, f.tenant, false)
	},
}

func init() {
	wmsWarehouseMembersListCmd.Flags().StringVar(&wmsWarehouseMembersList.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	wmsWarehouseMembersListCmd.Flags().StringVar(&wmsWarehouseMembersList.warehouse, "warehouse", "", "warehouse code (required)")
	wmsWarehouseMembersListCmd.Flags().IntVar(&wmsWarehouseMembersList.page, "page", 0, "page to fetch, from 1")
	wmsWarehouseMembersListCmd.Flags().IntVar(&wmsWarehouseMembersList.limit, "limit", 0, "rows per page, 1 to 50")

	wmsWarehouseMembersAddCmd.Flags().StringVar(&wmsWarehouseMembersAdd.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	wmsWarehouseMembersAddCmd.Flags().StringVar(&wmsWarehouseMembersAdd.warehouse, "warehouse", "", "warehouse code (required)")
	wmsWarehouseMembersAddCmd.Flags().StringVar(&wmsWarehouseMembersAddUserID, "user-id", "", "the member's user id (required)")
	wmsWarehouseMembersAddCmd.Flags().StringArrayVar(&wmsWarehouseMembersAddScopes, "scope", nil, "warehouse scope; repeatable")

	for _, c := range []struct {
		cmd *cobra.Command
		f   *wmsMemberScopesFlags
	}{{wmsWarehouseMembersScopesAddCmd, &wmsWarehouseMembersScopesAdd}, {wmsWarehouseMembersScopesRemoveCmd, &wmsWarehouseMembersScopesRemove}} {
		c.cmd.Flags().StringVar(&c.f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
		c.cmd.Flags().StringVar(&c.f.warehouse, "warehouse", "", "warehouse code (required)")
		c.cmd.Flags().StringArrayVar(&c.f.scopes, "scope", nil, "warehouse scope; repeatable")
	}

	wmsWarehouseMembersRemoveCmd.Flags().StringVar(&wmsWarehouseMembersRemove.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	wmsWarehouseMembersRemoveCmd.Flags().StringVar(&wmsWarehouseMembersRemove.warehouse, "warehouse", "", "warehouse code (required)")

	wmsWarehouseMembersScopesCmd.AddCommand(wmsWarehouseMembersScopesAddCmd, wmsWarehouseMembersScopesRemoveCmd)
	wmsWarehouseMembersCmd.AddCommand(
		wmsWarehouseMembersListCmd,
		wmsWarehouseMembersAddCmd,
		wmsWarehouseMembersScopesCmd,
		wmsWarehouseMembersRemoveCmd,
	)
	wmsWarehousesCmd.AddCommand(wmsWarehouseMembersCmd)
}
