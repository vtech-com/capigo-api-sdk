package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

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
            "role": "owner", "avatar_url": null }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false }
      }

  Read meta.total rather than counting .data[]: a page never holds more than
  --limit, so a full count needs meta, not arithmetic. role is owner or
  member.

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
                  "role": "owner", "avatar_url": null },
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

func init() {
	membersListCmd.Flags().StringVar(&memberListTenant, "tenant", "", "scope to this tenant code")
	membersListCmd.Flags().StringVarP(&memberListQuery, "query", "q", "", "filter by member name or email")
	membersListCmd.Flags().IntVar(&memberListPage, "page", 0, "page number (0 = server default)")
	membersListCmd.Flags().IntVar(&memberListLimit, "limit", 0, "items per page (0 = server default)")

	membersGetCmd.Flags().StringVar(&memberGetTenant, "tenant", "", "scope to this tenant code")

	memberCmd.AddCommand(membersListCmd, membersGetCmd)
	rootCmd.AddCommand(memberCmd)
}
