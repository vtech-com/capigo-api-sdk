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
                       [--limit <n>] [-o table|json|quiet]

FLAGS
  --tenant <code>
      Tenant to search. Optional — omit it to span every tenant this key can
      reach; table output then gains a Tenant column.
      See capigo help tenancy.

        capigo members list --tenant acme

  -q, --query <term>
      Filter by member display name or email.

        capigo members list --tenant acme -q tram

  --page <n>
      Page to fetch. Pages start at 1. The default, 0, sends no page
      parameter and lets the server choose.

  --limit <n>
      Items per page. The default, 0, sends no limit parameter; the server
      then applies its own default of 20.

        capigo members list --tenant acme --page 2 --limit 50

  -o, --output table|json|quiet
      Print rows, the JSON envelope, or bare ids. Defaults to table.
      See capigo help output.

OUTPUT
  A table of members, then a summary line. With --tenant omitted, a Tenant
  column is added.

      ┌──────────┬─────────────┬──────────────┬────────┐
      │ ID       │ Name        │ Email        │ Role   │
      ├──────────┼─────────────┼──────────────┼────────┤
      │ 4d9a1c07 │ Tram Nguyen │ tram@acme.vn │ owner  │
      │ 8e2f61ab │ Son Nguyen  │ son@acme.vn  │ member │
      └──────────┴─────────────┴──────────────┴────────┘
      Tenant: acme · Total: 2 · showing 2 (page 1/1) (all rows shown)

  Ids are shortened here to fit the page; the command prints them in full.
  Role is owner or member.

  -o json emits the list envelope; each row is a member:

      { "id": "4d9a1c07-...", "display_name": "Tram Nguyen",
        "email": "tram@acme.vn", "role": "owner", "avatar_url": null }

  The envelope, meta.total and list footers: capigo help output`,
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

		var envelope api.Envelope[[]api.Member]
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			data := envelope.Data
			if data == nil {
				data = []api.Member{}
			}
			return output.WriteJSONList(os.Stdout, data, envelope.Meta)
		}

		items := make([]output.Member, len(envelope.Data))
		for i, m := range envelope.Data {
			items[i] = output.Member{
				ID:    m.ID,
				Name:  m.DisplayName,
				Email: m.Email,
				Role:  m.Role,
			}
		}

		if err := output.Render(os.Stdout, outputMode, items, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "member",
		}); err != nil {
			return handleErr(err)
		}

		if outputMode == "table" {
			output.WriteListSummary(os.Stdout, output.ListSummary{
				Tenant:     derefTenant(tenant),
				TenantNote: tenantNote(tenant, memberListTenant),
				Shown:      len(envelope.Data),
				Page:       envelope.Meta.Page,
				Limit:      envelope.Meta.Limit,
				Total:      envelope.Meta.Total,
				HasMore:    envelope.Meta.HasMore,
			})
		}

		return nil
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
  capigo members get <id> [--tenant <code>] [-o table|json|quiet]

FLAGS
  <id>
      Member UUID. Positional, required.

  --tenant <code>
      Tenant to scope the lookup to. Optional; omit it to search every
      tenant this key can reach.

        capigo members get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

  -o, --output table|json|quiet
      Print a row, the JSON object, or the bare id. Defaults to table.
      See capigo help output.

OUTPUT
  A single-row table. Ids are shortened here to fit the page; the command
  prints them in full.

      ┌──────────┬─────────────┬──────────────┬───────┐
      │ ID       │ Name        │ Email        │ Role  │
      ├──────────┼─────────────┼──────────────┼───────┤
      │ 4d9a1c07 │ Tram Nguyen │ tram@acme.vn │ owner │
      └──────────┴─────────────┴──────────────┴───────┘

  -o json emits the bare member object:

      { "id": "4d9a1c07-...", "display_name": "Tram Nguyen",
        "email": "tram@acme.vn", "role": "owner", "avatar_url": null }

  Exit 4 when the member is not reachable — including a member who exists in
  a tenant this key cannot see.

  Output modes and the JSON contract: capigo help output`,
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

		var envelope struct {
			Data api.Member `json:"data"`
		}
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		if outputMode == "json" {
			return output.WriteJSONObject(os.Stdout, envelope.Data)
		}

		if err := output.Render(os.Stdout, outputMode, output.Member{
			ID:    envelope.Data.ID,
			Name:  envelope.Data.DisplayName,
			Email: envelope.Data.Email,
			Role:  envelope.Data.Role,
		}, output.RenderOpts{
			GlobalMode:   tenant == nil,
			ResourceKind: "member",
		}); err != nil {
			return handleErr(err)
		}

		return nil
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
