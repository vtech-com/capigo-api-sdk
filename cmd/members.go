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

// members update flags
var (
	memberUpdateTenant           string
	memberUpdateDisplayName      string
	memberUpdateEmail            string
	memberUpdateMobile           string
	memberUpdateMemberCode       string
	memberUpdateJobTitle         string
	memberUpdateDepartment       string
	memberUpdateBio              string
	memberUpdateRole             string
	memberUpdateStatus           string
	memberUpdatePositions        []string
	memberUpdatePermissions      []string
	memberUpdateClearPositions   bool
	memberUpdateClearPermissions bool
)

var membersUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a member's profile, role, status, positions or permissions",
	Long: `Update one member of a tenant: the HRIS-sync call.

PURPOSE
  Change what the members screen lets an owner change, from a script. Send only
  the fields you mean to change; the rest stay as they are. Only an active tenant
  owner may call it; a plain member is refused with 403. <id> is the member's
  user id, the id members list and members get print.

  The writes are separate steps and are not rolled back: profile fields first,
  then status active, role, permissions, positions, and status inactive or banned
  last. If a later step fails, the earlier ones stay applied and the error names
  what was already applied. Running the same command again is safe.

  An owner cannot change their own role or status (exit 3, E9206); a profile edit
  of their own is fine. Permissions apply to non-owner members only (exit 8,
  PERMISSIONS_NOT_APPLICABLE). The role of a member who is not active changes
  only together with --status active (exit 8, MEMBER_NOT_ACTIVE). A member code or
  email another member already uses is exit 8 (E4103, E4104). A position that does
  not exist in the tenant is exit 5. An id that names nothing, or a member of
  another tenant, is exit 4.

USAGE
  capigo members update <id> --tenant <code> [--display-name <name>]
                         [--email <address>] [--mobile <number>]
                         [--member-code <code>] [--job-title <title>]
                         [--department <name>] [--bio <text>]
                         [--role owner|member] [--status active|inactive|banned]
                         [--position <uuid>]... [--clear-positions]
                         [--permission <key>]... [--clear-permissions]

FLAGS
  <id>
      Member user UUID. Positional, required.

  --tenant <code>
      Tenant the member belongs to. Required.

  --display-name <name>
      New display name. It cannot be cleared.

  --email <address>, --mobile <number>, --member-code <code>, --job-title <title>,
  --department <name>, --bio <text>
      New value. An empty value clears the field: --job-title "" removes the title.
      A flag you leave out changes nothing.

        capigo members update 4d9a… --tenant acme --member-code EMP-001 --job-title Picker
        capigo members update 4d9a… --tenant acme --job-title ""

  --role owner|member, --status active|inactive|banned
      Change the role or status.

  --position <uuid>   (repeatable)
      The member's complete set of positions after the call. It replaces the old
      set. --clear-positions empties it.

  --permission <key>   (repeatable)
      The member's complete set of permissions after the call, for a non-owner.
      It replaces the old set. --clear-permissions empties it.

        capigo members update 4d9a… --tenant acme --permission wms_admin --permission catalog_view

OUTPUT
  The member's stored state is at .data (no email or mobile):

      { "data": { "id": "4d9a…", "role": "member", "status": "active",
                  "display_name": "Tram Nguyen", "member_code": "EMP-001",
                  "job_title": "Picker", "department": null, "bio": null,
                  "permissions": ["catalog_view", "wms_admin"], "positions": [] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-07T03:54:51Z" } }

  Exit 5 when no field is given. Exit 3 for a caller who is not an owner, or who
  changes their own role or status.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		flags := cmd.Flags()

		body := map[string]any{}
		text := func(flag, field, value string) {
			if flags.Changed(flag) {
				body[field] = strings.TrimSpace(value)
			}
		}
		text("display-name", "display_name", memberUpdateDisplayName)
		text("email", "email", memberUpdateEmail)
		text("mobile", "mobile", memberUpdateMobile)
		text("member-code", "member_code", memberUpdateMemberCode)
		text("job-title", "job_title", memberUpdateJobTitle)
		text("department", "department", memberUpdateDepartment)
		text("bio", "bio", memberUpdateBio)
		if flags.Changed("display-name") && strings.TrimSpace(memberUpdateDisplayName) == "" {
			failValidation("members update: --display-name cannot be blank; a display name cannot be cleared")
		}
		if flags.Changed("role") {
			if memberUpdateRole != "owner" && memberUpdateRole != "member" {
				failValidation("members update: --role must be owner or member")
			}
			body["role"] = memberUpdateRole
		}
		if flags.Changed("status") {
			switch memberUpdateStatus {
			case "active", "inactive", "banned":
				body["status"] = memberUpdateStatus
			default:
				failValidation("members update: --status must be active, inactive or banned")
			}
		}
		if memberUpdateClearPositions && len(memberUpdatePositions) > 0 {
			failValidation("members update: give --position or --clear-positions, not both")
		}
		if memberUpdateClearPermissions && len(memberUpdatePermissions) > 0 {
			failValidation("members update: give --permission or --clear-permissions, not both")
		}
		if len(memberUpdatePositions) > 0 {
			body["positions"] = memberUpdatePositions
		} else if memberUpdateClearPositions {
			body["positions"] = []string{}
		}
		if len(memberUpdatePermissions) > 0 {
			body["permissions"] = memberUpdatePermissions
		} else if memberUpdateClearPermissions {
			body["permissions"] = []string{}
		}
		if len(body) == 0 {
			failValidation("members update: give at least one field to change")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(memberUpdateTenant, profile)
		requireTenant(tenant, "members update")

		resp, err := client.Do(ctx, "PATCH", "/members/"+url.PathEscape(args[0]), body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, memberUpdateTenant, envelope.Meta)
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

	membersUpdateCmd.Flags().StringVar(&memberUpdateTenant, "tenant", "", "tenant the member belongs to (required)")
	membersUpdateCmd.Flags().StringVar(&memberUpdateDisplayName, "display-name", "", "new display name")
	membersUpdateCmd.Flags().StringVar(&memberUpdateEmail, "email", "", "new email; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateMobile, "mobile", "", "new mobile; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateMemberCode, "member-code", "", "new member code; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateJobTitle, "job-title", "", "new job title; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateDepartment, "department", "", "new department; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateBio, "bio", "", "new bio; empty clears it")
	membersUpdateCmd.Flags().StringVar(&memberUpdateRole, "role", "", "owner or member")
	membersUpdateCmd.Flags().StringVar(&memberUpdateStatus, "status", "", "active, inactive or banned")
	membersUpdateCmd.Flags().StringArrayVar(&memberUpdatePositions, "position", nil, "position UUID; repeat for the complete set")
	membersUpdateCmd.Flags().BoolVar(&memberUpdateClearPositions, "clear-positions", false, "remove every position")
	membersUpdateCmd.Flags().StringArrayVar(&memberUpdatePermissions, "permission", nil, "permission key; repeat for the complete set")
	membersUpdateCmd.Flags().BoolVar(&memberUpdateClearPermissions, "clear-permissions", false, "remove every permission")

	memberCmd.AddCommand(membersListCmd, membersGetCmd, membersInviteCmd, memberInvitationsCmd, membersUpdateCmd)
	rootCmd.AddCommand(memberCmd)
}
