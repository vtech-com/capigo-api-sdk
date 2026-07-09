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

Reads may span tenants: omit --tenant to see every tenant this key can reach.
  capigo help tenancy`,
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
  Find the people in a workspace — most often to turn a name into the UUID that
  tasks create --assignee and tasks list --assignee-id expect.

INPUT
  --tenant <code>     optional; omit it to span every accessible tenant
  -q, --query <term>  filter by member name or email
  --page <n>          page number (0 = server default)
  --limit <n>         items per page (0 = server default)

OUTPUT
  -o json emits the list envelope. Each row is a member:

      { id, display_name, email, role, avatar_url }

  role is owner or member.

  The envelope, meta.total and list footers: capigo help output

EXAMPLES
  capigo members list --tenant acme -q tram

  # Name to UUID, for an assignment
  capigo members list --tenant acme -q tram -o json | jq -r '.data[0].id'

SEE ALSO
  members get <id>      one member in full
  tasks list            filter tasks by --assignee-id
  capigo help tenancy   when --tenant may be omitted`,
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
	Short: "Get a member by ID",
	Long: `Get one member by UUID.

PURPOSE
  Read a single member. This command addresses a member by UUID only; to find
  that UUID from a name or an email, use members list --query.

INPUT
  <id>              member UUID (positional, required)
  --tenant <code>   optional; scopes the lookup

OUTPUT
  -o json emits the bare member object:

      { id, display_name, email, role, avatar_url }

  Exit 4 when the member is not reachable — including a member who exists in a
  tenant this key cannot see.

  Output modes and the JSON contract: capigo help output

EXAMPLES
  capigo members get <uuid> --tenant acme

SEE ALSO
  members list            find a member by name or email
  capigo help exit-codes  what exit 4 means`,
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
