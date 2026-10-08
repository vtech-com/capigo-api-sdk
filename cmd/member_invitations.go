package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var invitationStatuses = []string{"pending", "approved", "cancelled", "expired"}

var memberInvitationsCmd = &cobra.Command{
	Use:   "invitations",
	Short: "List and cancel a tenant's invitations",
	Long: `Invitations a tenant owner has sent: list them, cancel one that is still pending.

--tenant is required on both commands here: an invitation belongs to one tenant,
and only an active owner of that tenant may read or cancel it.
  capigo help tenancy

USAGE
  capigo members invitations <command> --tenant <code> [<args>]`,
}

// listQuery builds "?status=&email=&page=&limit=" for the owner-only list
// endpoints, refusing a status the API would refuse so the mistake costs no call.
func listQuery(command, status string, statuses []string, email string, page, limit int) string {
	params := url.Values{}
	if status != "" {
		valid := false
		for _, s := range statuses {
			if s == status {
				valid = true
			}
		}
		if !valid {
			failValidation("%s: --status must be one of: %s", command, strings.Join(statuses, ", "))
		}
		params.Set("status", status)
	}
	if v := strings.TrimSpace(email); v != "" {
		params.Set("email", v)
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if len(params) == 0 {
		return ""
	}
	return "?" + params.Encode()
}

// members invitations list flags
var (
	invitationListTenant string
	invitationListStatus string
	invitationListEmail  string
	invitationListPage   int
	invitationListLimit  int
)

var memberInvitationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a tenant's invitations",
	Long: `List the invitations a tenant has sent, newest first.

PURPOSE
  See who was invited and what became of it — the invitations list on the members
  screen. Only an active tenant owner may call it; a plain member is refused with
  403. No row carries the accept-link token: it is shown once, when the invitation
  is created. An invitee's email and mobile are shown here, to the owner only.

USAGE
  capigo members invitations list --tenant <code> [--status <status>]
                                  [--email <address>] [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant whose invitations to list. Required.

        capigo members invitations list --tenant acme

  --status <status>
      One of pending, approved, cancelled, expired. expired is a pending
      invitation past its expiry date, which the platform has not yet marked.
      Left out, every status is listed.

        capigo members invitations list --tenant acme --status expired

  --email <address>
      Only invitations sent to this address. Matched whole, ignoring case.

  --page <n>
      Page to fetch, from 1. The default, 0, lets the server choose.

  --limit <n>
      Items per page, 1 to 50. The default, 0, lets the server apply its own
      default of 20.

OUTPUT
  The invitations are at .data[]; read meta.total for the full count:

      { "data": [
          { "id": "…", "status": "pending", "invitee_email": "tram@acme.vn",
            "invitee_mobile": null, "expires_at": "2026-11-04T03:00:10+00:00",
            "expired": false, "custom_message": "Welcome",
            "created_at": "2026-10-05T03:00:10+00:00" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false } }

  Exit 5 for an unknown --status or a page or limit out of range. Exit 3 for a
  caller who is not a tenant owner.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		path := "/members/invitations" + listQuery("members invitations list",
			invitationListStatus, invitationStatuses, invitationListEmail, invitationListPage, invitationListLimit)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(invitationListTenant, profile)
		requireTenant(tenant, "members invitations list")

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, invitationListTenant, envelope.Meta))
	},
}

var invitationCancelTenant string

var memberInvitationsCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a pending invitation",
	Long: `Cancel an invitation that is still pending, so its link stops working.

PURPOSE
  Withdraw an invitation — the Cancel action on the members screen's invitations
  list. Only an active tenant owner may call it; a plain member is refused with
  403. Cancelling frees the invitee to be invited again.

  An invitation that was already accepted, refused or cancelled exits 8 with
  code E2308 and changes nothing. An invitation of another tenant, and an id that
  names nothing, both exit 4.

USAGE
  capigo members invitations cancel <id> --tenant <code>

FLAGS
  <id>
      Invitation UUID, from members invitations list. Positional, required.

  --tenant <code>
      Tenant that sent the invitation. Required.

        capigo members invitations cancel 7c1e… --tenant acme

OUTPUT
  The cancelled invitation is at .data:

      { "data": { "id": "7c1e…", "status": "cancelled" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-05T03:54:52Z" } }`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(invitationCancelTenant, profile)
		requireTenant(tenant, "members invitations cancel")

		resp, err := client.Do(ctx, "POST", "/members/invitations/"+url.PathEscape(args[0])+"/actions/cancel", nil, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, invitationCancelTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	memberInvitationsListCmd.Flags().StringVar(&invitationListTenant, "tenant", "", "tenant whose invitations to list (required)")
	memberInvitationsListCmd.Flags().StringVar(&invitationListStatus, "status", "", "pending, approved, cancelled or expired")
	memberInvitationsListCmd.Flags().StringVar(&invitationListEmail, "email", "", "only invitations sent to this address")
	memberInvitationsListCmd.Flags().IntVar(&invitationListPage, "page", 0, "page to fetch, from 1")
	memberInvitationsListCmd.Flags().IntVar(&invitationListLimit, "limit", 0, "items per page, 1 to 50")

	memberInvitationsCancelCmd.Flags().StringVar(&invitationCancelTenant, "tenant", "", "tenant that sent the invitation (required)")

	memberInvitationsCmd.AddCommand(memberInvitationsListCmd, memberInvitationsCancelCmd)
}
