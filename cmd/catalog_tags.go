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

// Catalog tags change one at a time: the API refuses any write that carries a
// whole tag list (E9474), so there is no set-the-list command here or on
// products update.

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List PCMS catalog tags",
	Long: `Catalog tags of the Product Catalog Management System (PCMS).

PURPOSE
  A catalog tag is a name and a colour that products and variants carry. One
  set per tenant: "Hè" and " hè " are the same tag, kept under the spelling it
  was first created with. Catalog tags and task labels (capigo labels) are
  separate sets.

  Tags have no create command. Naming one on products tags add, variants tags
  add, or products create --tags creates it, with a colour the system assigns.
  This group lists what exists.

USAGE
  capigo tags list --tenant <code>`,
}

var tagListTenant string

var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every catalog tag in a tenant",
	Long: `List every catalog tag in a tenant.

PURPOSE
  See which tags exist, including ones no product carries any more, to reuse a
  name instead of coining a near-duplicate.

USAGE
  capigo tags list --tenant <code>

FLAGS
  --tenant <code>
      Tenant to list. Required. Falls back to CAPIGO_TENANT, then to
      default_tenant in the config file. Exits 5 if none resolves.

        capigo tags list --tenant acme

OUTPUT
  Tags are at .data. There is no pagination.

      {
        "data": [ { "name": "Hè", "color": "sky" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  color is one of sky, orange, green, rose, teal, amber, violet, indigo,
  fuchsia, lime. Nobody chooses it.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(tagListTenant, activeProfileOrEmpty(cfg))
		requireTenant(tenant, "tags commands")

		resp, err := client.Do(context.Background(), "GET", "/pcms/tags", nil, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, rawList(envelope.Data), itemMeta(tenant, tagListTenant, envelope.Meta))
	},
}

// --------------------------------------------------------------------------
// products tags add|remove, variants tags add|remove
// --------------------------------------------------------------------------

// tagSubject is what a tag is added to or removed from. The two subjects share
// one command shape and differ only in path and in what the answer holds.
type tagSubject struct {
	group    string // "products" or "variants"
	path     string // "/pcms/products" or "/pcms/variants"
	tenant   *string
	answer   string // the OUTPUT section of the help
	removeOn string // the extra rule a remove follows, in help
}

var (
	productTagsTenant string
	variantTagsTenant string
)

var productTagSubject = tagSubject{
	group:  "products",
	path:   "/pcms/products",
	tenant: &productTagsTenant,
	answer: `The product's tags after the call are at .data:

      {
        "data": { "id": "8f2a1c07-...",
                  "tags": [ { "name": "Hè", "color": "sky" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-22T04:12:33Z" }
      }

  Every variant of the product inherits its tags.`,
	removeOn: `The tag stays in the tenant's tag list, and a variant that carries it as
  its own keeps it.`,
}

var variantTagSubject = tagSubject{
	group:  "variants",
	path:   "/pcms/variants",
	tenant: &variantTagsTenant,
	answer: `The variant's tags after the call are at .data, its own apart from the
  ones it inherits from its product:

      {
        "data": { "id": "6f1c9a3d-...",
                  "own_tags":       [ { "name": "Size lẻ", "color": "lime" } ],
                  "inherited_tags": [ { "name": "Hè", "color": "sky" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-22T04:12:33Z" }
      }

  A tag the variant carries both ways is listed in inherited_tags only.`,
	removeOn: `Removing a tag the variant only inherits is refused with E9455 (exit 5):
  remove it on the product with products tags remove. Removing one it carries
  both ways drops its own copy; the tag stays inherited. The tag stays in the
  tenant's tag list.`,
}

func newTagCmd(s tagSubject, verb string) *cobra.Command {
	noun := strings.TrimSuffix(s.group, "s")
	method, short, rule := "POST", "Add one tag to a "+noun, ""
	switch verb {
	case "add":
		rule = `A name matching an existing tag in another letter case, or with other
  surrounding spaces, adds that tag under its existing name. A new name creates
  the tag. Adding a tag the ` + noun + ` already carries changes nothing and
  still exits 0.`
	case "remove":
		method, short = "DELETE", "Remove one tag from a "+noun
		rule = `The name matches in any letter case. Removing a tag the ` + noun + ` does
  not carry changes nothing and still exits 0.

  ` + s.removeOn
	}

	cmd := &cobra.Command{
		Use:   verb + " <" + noun + "-id> <name>",
		Short: short,
		Long: short + `, by name.

PURPOSE
  Tags change one at a time. Each one added or removed is recorded as its own
  change and sent to VTech POS. The key's member is the actor and needs the
  right to edit the product.

  ` + rule + `

USAGE
  capigo ` + s.group + ` tags ` + verb + ` <` + noun + `-id> <name> --tenant <code>

FLAGS
  <` + noun + `-id>
      ` + strings.ToUpper(noun[:1]) + noun[1:] + ` UUID. Positional, required.

  <name>
      Tag name: 1-30 letters, digits, spaces, - or _ after surrounding spaces
      are trimmed. Quote a name holding a space. Any other name is refused
      with E9473 (exit 5) and nothing changes.

  --tenant <code>
      Tenant the ` + noun + ` belongs to. Required. Exits 4 if the ` + noun + ` is
      not in it.

        capigo ` + s.group + ` tags ` + verb + ` 8f2a1c07-... "Hè" --tenant acme

OUTPUT
  ` + s.answer,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			id, name := args[0], args[1]
			if strings.TrimSpace(name) == "" {
				failValidation("<name> must not be empty")
			}

			client, cfg, err := buildClient()
			if err != nil {
				return handleErr(err)
			}

			tenant := resolveTenant(*s.tenant, activeProfileOrEmpty(cfg))
			requireTenant(tenant, s.group+" commands")

			// Escaped: a name is typed text, and "Size/lẻ" must stay one segment.
			path := s.path + "/" + url.PathEscape(id) + "/tags/" + url.PathEscape(name)
			resp, err := client.Do(context.Background(), method, path, nil, tenant)
			if err != nil {
				return handleErr(err)
			}

			var envelope api.RawEnvelope
			if err := json.Unmarshal(resp.Body, &envelope); err != nil {
				return handleErr(fmt.Errorf("decode response: %w", err))
			}

			meta := itemMeta(tenant, *s.tenant, envelope.Meta)
			meta.ServerTime = resp.ServerTime
			return output.Write(os.Stdout, rawItem(envelope.Data), meta)
		},
	}
	cmd.Flags().StringVar(s.tenant, "tenant", "", "tenant code (required)")
	return cmd
}

func newTagsGroup(s tagSubject) *cobra.Command {
	noun := strings.TrimSuffix(s.group, "s")
	group := &cobra.Command{
		Use:   "tags",
		Short: "Add or remove a " + noun + "'s catalog tags, one at a time",
		Long: `Add or remove a ` + noun + `'s catalog tags, one at a time.

There is no command that sets the whole list: the API refuses it. To read the
tags, use ` + s.group + ` get. To list the tenant's tags, use tags list.

USAGE
  capigo ` + s.group + ` tags (add | remove) <` + noun + `-id> <name> --tenant <code>`,
	}
	group.AddCommand(newTagCmd(s, "add"), newTagCmd(s, "remove"))
	return group
}

var (
	productsTagsCmd = newTagsGroup(productTagSubject)
	variantsTagsCmd = newTagsGroup(variantTagSubject)
)

func init() {
	tagsListCmd.Flags().StringVar(&tagListTenant, "tenant", "", "tenant code (required)")
	tagsCmd.AddCommand(tagsListCmd)
	rootCmd.AddCommand(tagsCmd)
}
