package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var joinRequestCmd = &cobra.Command{
	Use:   "join-requests",
	Short: "List and decide join requests",
	Long: `Join requests: people who asked to join a tenant, and the owner's decision.

--tenant is required on every command here: a join request belongs to one tenant,
and only an active owner of that tenant may decide it.
  capigo help tenancy

USAGE
  capigo join-requests <command> --tenant <code> [<args>]`,
}

var joinRequestStatuses = []string{"pending", "approved", "rejected"}

// join-requests list flags
var (
	joinRequestListTenant string
	joinRequestListStatus string
	joinRequestListEmail  string
	joinRequestListPage   int
	joinRequestListLimit  int
)

var joinRequestsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a tenant's join requests",
	Long: `List the join requests a tenant has received, newest first.

PURPOSE
  Find the id of a request to approve or reject — the join requests list on the
  members screen. Only an active tenant owner may call it; a plain member is
  refused with 403. The requester's contact email and mobile are shown here, to
  the owner only; approve and reject answers never carry them.

USAGE
  capigo join-requests list --tenant <code> [--status <status>]
                            [--email <address>] [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant whose requests to list. Required.

        capigo join-requests list --tenant acme --status pending

  --status <status>
      One of pending, approved, rejected. Left out, every status is listed.

  --email <address>
      Only requests whose contact email is this address. Matched whole, ignoring
      case.

  --page <n>
      Page to fetch, from 1. The default, 0, lets the server choose.

  --limit <n>
      Items per page, 1 to 50. The default, 0, lets the server apply its own
      default of 20.

OUTPUT
  The requests are at .data[]; read meta.total for the full count:

      { "data": [
          { "id": "7c1e…", "status": "pending",
            "contact_email": "tram@acme.vn", "contact_mobile": "84912345678",
            "bio": null, "created_at": "2026-10-05T03:00:10+00:00",
            "updated_at": "2026-10-05T03:00:10+00:00" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false } }

  Exit 5 for an unknown --status or a page or limit out of range. Exit 3 for a
  caller who is not a tenant owner.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		path := "/join-requests" + listQuery("join-requests list",
			joinRequestListStatus, joinRequestStatuses, joinRequestListEmail, joinRequestListPage, joinRequestListLimit)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(joinRequestListTenant, profile)
		requireTenant(tenant, "join-requests list")

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, joinRequestListTenant, envelope.Meta))
	},
}

// join-requests approve flags
var (
	joinRequestApproveTenant      string
	joinRequestApproveDisplayName string
	joinRequestApproveMemberCode  string
	joinRequestApproveJobTitle    string
	joinRequestApproveDepartment  string
)

var joinRequestsApproveCmd = &cobra.Command{
	Use:   "approve <id>",
	Short: "Approve a join request, making the requester a member",
	Long: `Approve a pending join request: the requester becomes a member of the tenant.

PURPOSE
  Admit someone who asked to join — the command behind the Approve button on the
  join requests list. Only an active tenant owner may call it; a plain member is
  refused with 403. The new member gets the member role, and an owner is never
  created this way.

  Only one decision counts. If two owners decide the same request at once, one
  wins and the other exits 8 with code E2403; a request that was already approved
  or rejected answers the same way. A requester whose membership in the tenant is
  inactive or banned is refused with exit 8 and code E2302, and the request stays
  pending; set their status back to active first, or reject the request. If
  creating the member fails, the request goes back to pending and can be tried
  again. A request of another tenant, and an id that names nothing, both exit 4.

  No answer carries the requester's email or mobile.

USAGE
  capigo join-requests approve <id> --tenant <code> --display-name <name>
                               [--member-code <code>] [--job-title <title>]
                               [--department <name>]

FLAGS
  <id>
      Join request UUID. Positional, required.

  --tenant <code>
      Tenant that owns the request. Required.

        capigo join-requests approve 7c1e… --tenant acme --display-name "Tram Nguyen"

  --display-name <name>
      Name shown for the new member. Required, and not blank.

  --member-code <code>
      Optional internal code for the member. Left out, the platform generates one;
      a code already used in the tenant exits 8 with code E4103.

  --job-title <title>
      Optional job title.

  --department <name>
      Optional department.

OUTPUT
  The decided request is at .data. member_id is the new member; it is absent when
  the requester was already an active member and nothing was created:

      { "data": { "id": "7c1e…", "status": "approved",
                  "member_id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-05T03:54:51Z" } }`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		displayName := strings.TrimSpace(joinRequestApproveDisplayName)
		if displayName == "" {
			failValidation("join-requests approve: --display-name is required and must not be blank")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile := activeProfileOrEmpty(cfg)

		tenant := resolveTenant(joinRequestApproveTenant, profile)
		requireTenant(tenant, "join-requests approve")

		body := map[string]string{"display_name": displayName}
		if v := strings.TrimSpace(joinRequestApproveMemberCode); v != "" {
			body["member_code"] = v
		}
		if v := strings.TrimSpace(joinRequestApproveJobTitle); v != "" {
			body["job_title"] = v
		}
		if v := strings.TrimSpace(joinRequestApproveDepartment); v != "" {
			body["department"] = v
		}

		resp, err := client.Do(ctx, "POST", "/join-requests/"+url.PathEscape(args[0])+"/actions/approve", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, joinRequestApproveTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// join-requests reject flags
var joinRequestRejectTenant string

var joinRequestsRejectCmd = &cobra.Command{
	Use:   "reject <id>",
	Short: "Reject a join request",
	Long: `Reject a pending join request: no member is created.

PURPOSE
  Decline someone who asked to join — the command behind the Reject button on the
  join requests list. Only an active tenant owner may call it; a plain member is
  refused with 403. There is no reason to give: the platform records none, so
  none is accepted. The requester may submit a new request later.

  Only one decision counts. If another decision got there first, or the request
  was already decided, this exits 8 with code E2403 and changes nothing. A request
  of another tenant, and an id that names nothing, both exit 4.

USAGE
  capigo join-requests reject <id> --tenant <code>

FLAGS
  <id>
      Join request UUID. Positional, required.

  --tenant <code>
      Tenant that owns the request. Required.

        capigo join-requests reject 7c1e… --tenant acme

OUTPUT
  The decided request is at .data:

      { "data": { "id": "7c1e…", "status": "rejected" },
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

		tenant := resolveTenant(joinRequestRejectTenant, profile)
		requireTenant(tenant, "join-requests reject")

		// No body: the API refuses a field, and there is none to send.
		resp, err := client.Do(ctx, "POST", "/join-requests/"+url.PathEscape(args[0])+"/actions/reject", nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, joinRequestRejectTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	joinRequestsListCmd.Flags().StringVar(&joinRequestListTenant, "tenant", "", "tenant whose requests to list (required)")
	joinRequestsListCmd.Flags().StringVar(&joinRequestListStatus, "status", "", "pending, approved or rejected")
	joinRequestsListCmd.Flags().StringVar(&joinRequestListEmail, "email", "", "only requests with this contact email")
	joinRequestsListCmd.Flags().IntVar(&joinRequestListPage, "page", 0, "page to fetch, from 1")
	joinRequestsListCmd.Flags().IntVar(&joinRequestListLimit, "limit", 0, "items per page, 1 to 50")

	joinRequestsApproveCmd.Flags().StringVar(&joinRequestApproveTenant, "tenant", "", "tenant that owns the request (required)")
	joinRequestsApproveCmd.Flags().StringVar(&joinRequestApproveDisplayName, "display-name", "", "name shown for the new member (required)")
	joinRequestsApproveCmd.Flags().StringVar(&joinRequestApproveMemberCode, "member-code", "", "optional member code; generated when omitted")
	joinRequestsApproveCmd.Flags().StringVar(&joinRequestApproveJobTitle, "job-title", "", "optional job title")
	joinRequestsApproveCmd.Flags().StringVar(&joinRequestApproveDepartment, "department", "", "optional department")

	joinRequestsRejectCmd.Flags().StringVar(&joinRequestRejectTenant, "tenant", "", "tenant that owns the request (required)")

	joinRequestCmd.AddCommand(joinRequestsListCmd, joinRequestsApproveCmd, joinRequestsRejectCmd)
	rootCmd.AddCommand(joinRequestCmd)
}
