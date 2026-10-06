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
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var memberCmd = &cobra.Command{
	Use:   "members",
	Short: "Manage members",
	Long: `Workspace members in Capigo Mission.

--tenant is optional on both commands here: list and get search across every
tenant this key can reach when it is omitted.
  capigo help tenancy

USAGE
  capigo members <command> [--tenant <code>] [<args>]`,
}

var (
	memberListTenant string
	memberListQuery  string
	memberListPage   int
	memberListLimit  int
)

var membersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace members",
	Long: `List workspace members.

PURPOSE
  Find the people in a workspace — most often to turn a name into the UUID
  that tasks create --assignee and tasks list --assignee-id expect.

USAGE
  capigo members list [--tenant <code>] [-q <term>] [--page <n>]
                       [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to search. Optional — omit it to span every tenant this key can
      reach; meta.tenant is then empty.
      See capigo help tenancy.

        capigo members list --tenant acme

  -q, --query <term>
      Filter by member display name or email.

        capigo members list --tenant acme -q tram

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page, 1 to 50. The default, 0, sends no limit parameter; the
      server then applies its own default of 20. Above 50 the server rejects
      the call with exit 5.

        capigo members list --tenant acme --page 2 --limit 50

OUTPUT
  The members are at .data[]:

      {
        "data": [
          { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
            "display_name": "Tram Nguyen", "email": "tram@acme.vn",
            "role": "owner", "avatar_url": null,
            "title": null, "department": null, "birthday": null }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic. role is owner or
  member. title, department and birthday come from extra_data and are null
  when that data was never set.

  With --tenant omitted, meta.tenant and meta.tenant_source are absent: the
  results span every tenant this key can reach, and a member does not name its
  own tenant either. Pass --tenant when you need to know where a member lives.`,
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

		tenant := resolveTenant(memberListTenant, profile)

		params := url.Values{}
		if memberListQuery != "" {
			params.Set("q", memberListQuery)
		}
		if memberListPage > 0 {
			params.Set("page", strconv.Itoa(memberListPage))
		}
		if memberListLimit > 0 {
			params.Set("limit", strconv.Itoa(memberListLimit))
		}

		path := "/members"
		if len(params) > 0 {
			path += "?" + params.Encode()
		}

		resp, err := client.Do(ctx, "GET", path, nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, memberListTenant, envelope.Meta))
	},
}

var memberGetTenant string

var membersGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a member by id",
	Long: `Get one member by id.

PURPOSE
  Read a single member. This command addresses a member by id only; to find
  that id from a name or an email, use members list --query.

USAGE
  capigo members get <id> [--tenant <code>]

FLAGS
  <id>
      Member UUID. Positional, required.

  --tenant <code>
      Tenant to scope the lookup to. Optional; omit it to search every
      tenant this key can reach.

        capigo members get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The member is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "display_name": "Tram Nguyen", "email": "tram@acme.vn",
                  "role": "owner", "avatar_url": null,
                  "title": null, "department": null, "birthday": null },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  With --tenant omitted, meta.tenant and meta.tenant_source are both empty.

  Exit 4 when the member is not reachable — including a member who exists in
  a tenant this key cannot see.`,
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

		tenant := resolveTenant(memberGetTenant, profile)

		resp, err := client.Do(ctx, "GET", "/members/"+args[0], nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawItem(envelope.Data), itemMeta(tenant, memberGetTenant, envelope.Meta))
	},
}

// members invite flags
var (
	memberInviteTenant         string
	memberInviteEmail          string
	memberInviteMobile         string
	memberInviteIdempotencyKey string
	memberInviteCustomMessage  string
)

var membersInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a person to a tenant",
	Long: `Invite a person to a tenant by email, mobile or both.

PURPOSE
  Put a person on a tenant's pending invitations — the command behind the
  members screen's invite form. Only an active tenant owner may call it; a plain
  member is refused with 403. The platform sends no email or SMS, so delivering
  the invitation is your job: the answer that creates it carries the accept-link
  token, and the link is {your workspace's web origin}/invite?token={token}. The
  person must sign in with an account whose email or mobile matches the invitation
  to accept it. An invitation expires 30 days after it is created.

  The token is shown once. It is in the answer that creates the invitation and
  nowhere else: not in a replay, not in an error, not in any later read. If you
  lose that answer, cancel the invitation (members invitations cancel) and invite
  again; a new invite for the same person is refused while the first is live.
  Store the token before anything else. No answer carries the invitee's email or mobile.

USAGE
  capigo members invite --tenant <code> (--email <address> | --mobile <number>)
                        [--custom-message <text>] [--idempotency-key <key>]

FLAGS
  --tenant <code>
      Tenant to invite into. Required here: an invitation belongs to one tenant.
      See capigo help tenancy.

        capigo members invite --tenant acme --email tram@acme.vn

  --email <address>
      Invitee's email. Stored lowercase. Give this or --mobile, or both.

  --mobile <number>
      Invitee's mobile, in any common format; it is stored normalized.

        capigo members invite --tenant acme --mobile "+84 912 345 678"

  --custom-message <text>
      A note shown to the invitee, up to 500 characters. Blank means no message.

        capigo members invite --tenant acme --email tram@acme.vn \
          --custom-message "Welcome to the team"

  --idempotency-key <key>
      Make a retry safe. The same key with the same invitee returns the same
      invitation, in its current status and without the token, and writes nothing
      new; the same key with a different invitee is refused with exit 8 and code
      E0601. A blank key is refused here, because the API would read it as no key.

        capigo members invite --tenant acme --email tram@acme.vn \
          --idempotency-key onboard-tram-001

OUTPUT
  The invitation is at .data. The token is there only on the call that created it:

      { "data": { "id": "…", "status": "pending",
                  "expires_at": "2026-11-04T03:00:10.255+00:00",
                  "created_at": "2026-10-05T03:00:10.255561+00:00",
                  "token": "…" },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-05T03:00:10Z" } }

  Exit 5 when neither --email nor --mobile is given or an address does not
  validate. Exit 8 when the invitee is already a member or already has a live
  invitation, or when the key was used with a different invitee (E0601). Exit 3 for
  a caller who is not a tenant owner.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()

		email := strings.TrimSpace(memberInviteEmail)
		mobile := strings.TrimSpace(memberInviteMobile)
		if email == "" && mobile == "" {
			failValidation("members invite: give --email, --mobile or both; an invitation needs someone to reach")
		}
		idempotencyKey := requireUsableKey(
			cmd.Flags().Changed("idempotency-key"),
			memberInviteIdempotencyKey,
			"idempotency-key",
		)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		profile := activeProfileOrEmpty(cfg)

		tenant := resolveTenant(memberInviteTenant, profile)
		requireTenant(tenant, "members invite")

		body := map[string]string{}
		if email != "" {
			body["invitee_email"] = email
		}
		if mobile != "" {
			body["invitee_mobile"] = mobile
		}

		if v := strings.TrimSpace(memberInviteCustomMessage); v != "" {
			if len([]rune(v)) > 500 {
				failValidation("members invite: --custom-message must be at most 500 characters")
			}
			body["custom_message"] = v
		}

		headers := map[string]string{}
		if idempotencyKey != "" {
			headers["Idempotency-Key"] = idempotencyKey
		}

		resp, err := client.DoWithHeaders(ctx, "POST", "/members/invitations", body, tenant, headers)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, memberInviteTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	membersListCmd.Flags().StringVar(&memberListTenant, "tenant", "", "scope to this tenant code")
	membersListCmd.Flags().StringVarP(&memberListQuery, "query", "q", "", "filter by member name or email")
	membersListCmd.Flags().IntVar(&memberListPage, "page", 0, "page number (0 = server default)")
	membersListCmd.Flags().IntVar(&memberListLimit, "limit", 0, "items per page (0 = server default)")

	membersGetCmd.Flags().StringVar(&memberGetTenant, "tenant", "", "scope to this tenant code")

	membersInviteCmd.Flags().StringVar(&memberInviteTenant, "tenant", "", "tenant to invite into (required)")
	membersInviteCmd.Flags().StringVar(&memberInviteEmail, "email", "", "invitee's email")
	membersInviteCmd.Flags().StringVar(&memberInviteMobile, "mobile", "", "invitee's mobile")
	membersInviteCmd.Flags().StringVar(&memberInviteCustomMessage, "custom-message", "", "note shown to the invitee, up to 500 characters")
	membersInviteCmd.Flags().StringVar(&memberInviteIdempotencyKey, "idempotency-key", "", "make a retry safe; the same key with the same invitee replays")

	memberCmd.AddCommand(membersListCmd, membersGetCmd, membersInviteCmd, memberInvitationsCmd)
	rootCmd.AddCommand(memberCmd)
}
