package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

const vatPoliciesPath = "/pcms/vat-policies"

var vatPoliciesCmd = &cobra.Command{
	Use:   "vat-policies",
	Short: "Manage PCMS VAT policies",
	Long: `VAT policies, part of the Capigo Product Catalog Management System (PCMS).

A VAT policy is a named VAT rate, in percent, that belongs to a tenant. A
product or product type points at one with --vat-policy-id; a product with no
policy of its own takes its type's rate. Every command here requires a
tenant, and every response names the tenant it resolved to, in meta.

USAGE
  capigo vat-policies <command> --tenant <code> [<args>]`,
}

// vatPolicyCall sends one request and prints the envelope's data as an item or
// a list. Tenant is required, as for every PCMS group.
func vatPolicyCall(tenantFlag, method, path string, body any, list bool) error {
	client, cfg, err := buildClient()
	if err != nil {
		return handleErr(err)
	}
	tenant := resolveTenant(tenantFlag, activeProfileOrEmpty(cfg))
	requireTenant(tenant, "vat-policies commands")

	resp, err := client.Do(context.Background(), method, path, body, tenant)
	if err != nil {
		return handleErr(err)
	}
	var envelope api.RawEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return handleErr(fmt.Errorf("decode response: %w", err))
	}
	meta := itemMeta(tenant, tenantFlag, envelope.Meta)
	meta.ServerTime = resp.ServerTime
	if list {
		return output.Write(os.Stdout, rawList(envelope.Data), meta)
	}
	return output.Write(os.Stdout, rawItem(envelope.Data), meta)
}

var (
	vatPolicyListTenant   string
	vatPolicyGetTenant    string
	vatPolicyCreateTenant string
	vatPolicyCreateName   string
	vatPolicyCreateRate   float64
	vatPolicyUpdateTenant string
	vatPolicyUpdateName   string
	vatPolicyUpdateRate   float64
	vatPolicyReplTenant   string
	vatPolicyReplName     string
	vatPolicyReplRate     float64
	vatPolicyDelTenant    string
)

var vatPoliciesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List VAT policies",
	Long: `List the VAT policies of a tenant, in creation order.

PURPOSE
  Read every VAT policy. Resolve a --vat-policy-id for products and product
  types from here. The list is not paged: one call returns all of them.

USAGE
  capigo vat-policies list --tenant <code>

FLAGS
  --tenant <code>
      Tenant to read from. Required.

        capigo vat-policies list --tenant acme

OUTPUT
  The policies are at .data[], in creation order. rate is a percent:

      {
        "data": [
          { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
            "name": "Standard goods", "rate": 8 },
          { "id": "8f2e0a91-6c3d-4b17-9e2a-1f5d7c8b0e44",
            "name": "Exempt", "rate": 0 }
        ],
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  meta.tenant is the tenant this call ran against. See capigo help tenancy.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return vatPolicyCall(vatPolicyListTenant, "GET", vatPoliciesPath, nil, true)
	},
}

var vatPoliciesGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a VAT policy by id",
	Long: `Get one VAT policy by id.

PURPOSE
  Read a single VAT policy.

USAGE
  capigo vat-policies get <id> --tenant <code>

FLAGS
  <id>
      VAT policy id, a UUID. Positional, required.

  --tenant <code>
      Tenant the policy belongs to. Required. Exits 4 if it is not in it.

        capigo vat-policies get 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The policy is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "name": "Standard goods", "rate": 8 },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  Exit 4 when no such policy exists in the resolved tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return vatPolicyCall(vatPolicyGetTenant, "GET", vatPoliciesPath+"/"+url.PathEscape(args[0]), nil, false)
	},
}

const vatPolicyWriteOutputHelp = `OUTPUT
  The policy as it now stands is at .data:

      {
        "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb",
                  "name": "Standard goods", "rate": 8 },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-07-09T04:12:33Z" }
      }

  meta.tenant is the tenant the write landed in. See capigo help tenancy.`

var vatPoliciesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a VAT policy",
	Long: `Create a VAT policy.

PURPOSE
  Add a named VAT rate to the tenant. Needs the owner or catalog admin right.

USAGE
  capigo vat-policies create --tenant <code> --name <text> --rate <percent>

FLAGS
  --tenant <code>
      Tenant to create the policy in. Required.

  --name <text>
      Policy name, 1 to 500 characters, trimmed on save. Unique among the
      tenant's policies, letter case aside (a taken name exits 8). Required.

  --rate <percent>
      VAT rate in percent, 0 to 100, at most two decimals; a third decimal is
      refused, never rounded. Required.

        capigo vat-policies create --tenant acme --name "Standard goods" --rate 8

` + vatPolicyWriteOutputHelp + `

  Exit 5 if --name or --rate is missing.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if vatPolicyCreateName == "" {
			failValidation("--name is required")
		}
		if !cmd.Flags().Changed("rate") {
			failValidation("--rate is required")
		}
		return vatPolicyCall(vatPolicyCreateTenant, "POST", vatPoliciesPath,
			api.CreateVatPolicyRequest{Name: vatPolicyCreateName, Rate: vatPolicyCreateRate}, false)
	},
}

var vatPoliciesUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Change the name or rate of a VAT policy",
	Long: `Change the name, the rate or both of a VAT policy.

PURPOSE
  This is PATCH: fields you do not send are left unchanged, and at least one
  is required. replace <id> sends PUT and requires both.

USAGE
  capigo vat-policies update <id> --tenant <code> [--name <text>] [--rate <percent>]

FLAGS
  <id>
      VAT policy id, a UUID. Positional, required.

  --tenant <code>
      Tenant the policy belongs to. Required.

  --name <text>
      New name, 1 to 500 characters. Unique among the tenant's policies.

  --rate <percent>
      New rate, 0 to 100, at most two decimals. 0 is a valid rate.

        capigo vat-policies update 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --rate 10

` + vatPolicyWriteOutputHelp + `

  Exit 5 if neither --name nor --rate is given. Exit 4 if <id> is not in the
  resolved tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		req := api.UpdateVatPolicyRequest{}
		if cmd.Flags().Changed("name") {
			req.Name = &vatPolicyUpdateName
		}
		if cmd.Flags().Changed("rate") {
			req.Rate = &vatPolicyUpdateRate
		}
		if req.Name == nil && req.Rate == nil {
			failValidation("at least one of --name or --rate is required")
		}
		return vatPolicyCall(vatPolicyUpdateTenant, "PATCH", vatPoliciesPath+"/"+url.PathEscape(args[0]), req, false)
	},
}

var vatPoliciesReplaceCmd = &cobra.Command{
	Use:   "replace <id>",
	Short: "Replace the name and rate of a VAT policy",
	Long: `Replace the name and the rate of a VAT policy.

PURPOSE
  Set both fields with PUT. Both are required. To change one field, use
  update <id>, which sends PATCH.

USAGE
  capigo vat-policies replace <id> --tenant <code> --name <text> --rate <percent>

FLAGS
  <id>
      VAT policy id, a UUID. Positional, required.

  --tenant <code>
      Tenant the policy belongs to. Required.

  --name <text>
      New name, 1 to 500 characters. Required.

  --rate <percent>
      New rate, 0 to 100, at most two decimals. Required.

        capigo vat-policies replace 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb \
            --tenant acme --name "Reduced goods" --rate 5

` + vatPolicyWriteOutputHelp + `

  Exit 5 if --name or --rate is missing. Exit 4 if <id> is not in the
  resolved tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if vatPolicyReplName == "" {
			failValidation("--name is required")
		}
		if !cmd.Flags().Changed("rate") {
			failValidation("--rate is required")
		}
		return vatPolicyCall(vatPolicyReplTenant, "PUT", vatPoliciesPath+"/"+url.PathEscape(args[0]),
			api.CreateVatPolicyRequest{Name: vatPolicyReplName, Rate: vatPolicyReplRate}, false)
	},
}

var vatPoliciesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a VAT policy",
	Long: `Soft-delete a VAT policy.

PURPOSE
  The policy leaves every read and its name is free for a new policy at once.
  Needs the owner or catalog admin right. A policy a live product or product
  type still uses is refused (exit 8): point those at another policy, or clear
  it, first.

  A repeated delete is exit 4: the first one succeeded, so a script that lost
  the answer and retries should treat exit 4 as done.

USAGE
  capigo vat-policies delete <id> --tenant <code>

FLAGS
  <id>
      VAT policy id, a UUID. Positional, required.

  --tenant <code>
      Tenant the policy belongs to. Required.

        capigo vat-policies delete 4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb --tenant acme

OUTPUT
  The answer names the policy by id, because every read answers exit 4 once
  the delete has run:

      { "data": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return vatPolicyCall(vatPolicyDelTenant, "DELETE", vatPoliciesPath+"/"+url.PathEscape(args[0]), nil, false)
	},
}

func init() {
	for _, p := range []struct {
		c *cobra.Command
		t *string
	}{
		{vatPoliciesListCmd, &vatPolicyListTenant},
		{vatPoliciesGetCmd, &vatPolicyGetTenant},
		{vatPoliciesCreateCmd, &vatPolicyCreateTenant},
		{vatPoliciesUpdateCmd, &vatPolicyUpdateTenant},
		{vatPoliciesReplaceCmd, &vatPolicyReplTenant},
		{vatPoliciesDeleteCmd, &vatPolicyDelTenant},
	} {
		p.c.Flags().StringVar(p.t, "tenant", "", "tenant code (required)")
	}

	vatPoliciesCreateCmd.Flags().StringVar(&vatPolicyCreateName, "name", "", "policy name, 1-500 chars (required)")
	vatPoliciesCreateCmd.Flags().Float64Var(&vatPolicyCreateRate, "rate", 0, "VAT rate in percent, 0-100, two decimals at most (required)")
	vatPoliciesUpdateCmd.Flags().StringVar(&vatPolicyUpdateName, "name", "", "new name, 1-500 chars")
	vatPoliciesUpdateCmd.Flags().Float64Var(&vatPolicyUpdateRate, "rate", 0, "new rate in percent, 0-100")
	vatPoliciesReplaceCmd.Flags().StringVar(&vatPolicyReplName, "name", "", "new name, 1-500 chars (required)")
	vatPoliciesReplaceCmd.Flags().Float64Var(&vatPolicyReplRate, "rate", 0, "new rate in percent, 0-100 (required)")

	vatPoliciesCmd.AddCommand(vatPoliciesListCmd, vatPoliciesGetCmd, vatPoliciesCreateCmd,
		vatPoliciesUpdateCmd, vatPoliciesReplaceCmd, vatPoliciesDeleteCmd)
	rootCmd.AddCommand(vatPoliciesCmd)
}
