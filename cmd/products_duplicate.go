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

// products duplicate flags
var (
	productDuplicateTenant         string
	productDuplicateName           string
	productDuplicateIdempotencyKey string
)

var productsDuplicateCmd = &cobra.Command{
	Use:   "duplicate <id>",
	Short: "Copy a product as a new draft",
	Long: `Copy a product, with its options, variants and prices, as a new draft.

PURPOSE
  The call behind the Duplicate action in PCMS, done in one transaction. The caller
  needs the tenant owner, catalog admin or catalog contributor role (exit 3
  otherwise). An unknown, deleted, malformed or another tenant's id is exit 4.

  Copied: description, currency, tags, and the brand, category, product type and unit
  (one deleted since is left empty on the copy). The live options in order. Every live,
  active variant that came from the options, with its name, option values, prices,
  weight, dimensions, legacy code and extra data. A product without options copies one
  variant: the oldest.

  Not copied: the SKU (every variant gets a new generated one), the barcode and the
  manufacturer code (each is unique in the tenant), aliases, notes, product-level extra
  data, media, manual, orphaned, inactive and deleted variants, and anything stock-side.
  The copy is always a draft with no publication date, whatever the source's status.
  When no variant can be copied the call exits 5 with E9485 and creates nothing.

  A copy is a create, so a repeat would make a second copy. Give --idempotency-key to
  make a retry safe: the same key with the same source and name returns the copy as it
  is now (exit 0, HTTP 200) and creates nothing; the same key with another source or
  name, or whose copy was deleted, is exit 8 (E0601). Without a key there is no
  protection.

USAGE
  capigo products duplicate <id> --tenant <code> [--name <name>]
                            [--idempotency-key <key>]

FLAGS
  <id>
      Source product UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required.

  --name <name>
      Name of the copy, 1 to 500 characters. Left out, the copy is named
      "<source name> (bản sao)". Part of the idempotency comparison: writing out the
      default name is not the same request as omitting it.

  --idempotency-key <key>
      1 to 255 characters, scoped to the tenant. A blank key is refused here, because
      the API would read it as no key.

        capigo products duplicate 8f2a… --tenant acme --name "Red tee (copy)" \\
          --idempotency-key dup-red-tee-001

OUTPUT
  The new product is at .data, in the same shape as products get, with status draft
  and its own generated SKUs:

      { "data": { "id": "…", "name": "Red tee (copy)", "status": "DRAFT",
                  "variants": [ … ], "options": [ … ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		idempotencyKey := requireUsableKey(cmd.Flags().Changed("idempotency-key"), productDuplicateIdempotencyKey, "idempotency-key")
		if len(idempotencyKey) > 255 {
			failValidation("products duplicate: --idempotency-key is over 255 characters")
		}
		var body any
		if cmd.Flags().Changed("name") {
			name := strings.TrimSpace(productDuplicateName)
			if name == "" || len([]rune(name)) > 500 {
				failValidation("products duplicate: --name must be 1 to 500 characters")
			}
			body = map[string]string{"name": name}
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(productDuplicateTenant, profile)
		requireTenant(tenant, "products duplicate")

		headers := map[string]string{}
		if idempotencyKey != "" {
			headers["Idempotency-Key"] = idempotencyKey
		}
		resp, err := client.DoWithHeaders(ctx, "POST", "/pcms/products/"+url.PathEscape(args[0])+"/actions/duplicate", body, tenant, headers)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, productDuplicateTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	productsDuplicateCmd.Flags().StringVar(&productDuplicateTenant, "tenant", "", "tenant the product belongs to (required)")
	productsDuplicateCmd.Flags().StringVar(&productDuplicateName, "name", "", "name of the copy; default \"<source name> (bản sao)\"")
	productsDuplicateCmd.Flags().StringVar(&productDuplicateIdempotencyKey, "idempotency-key", "", "make a retry safe; the same key with the same source and name replays")
}
