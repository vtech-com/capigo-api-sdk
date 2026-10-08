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

// products options flags
var (
	productOptionsTenant        string
	productOptionsOptions       []string
	productOptionsNone          bool
	productOptionsStrategy      string
	productOptionsOverridesFile string
)

// parseOptionFlag reads "Color=Red,Blue" into a name and its values.
func parseOptionFlag(raw string) (map[string]any, error) {
	name, list, ok := strings.Cut(raw, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return nil, fmt.Errorf("--option %q: write it as Name=value1,value2", raw)
	}
	values := []string{}
	for _, v := range strings.Split(list, ",") {
		if v = strings.TrimSpace(v); v != "" {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("--option %q: give at least one value after the =", raw)
	}
	if len(values) > 10 {
		return nil, fmt.Errorf("--option %q: an option holds at most 10 values", raw)
	}
	return map[string]any{"name": name, "values": values}, nil
}

var productsOptionsCmd = &cobra.Command{
	Use:   "options <id>",
	Short: "Save a product's options and regenerate its variants",
	Long: `Save the complete option set of a product and regenerate its variants.

PURPOSE
  The call behind the PCMS editor's option save: send the product's options as they
  should be afterwards (at most two, each with at most ten values), and the variants
  for every combination are generated in one transaction. All or nothing: a failure
  rolls the whole save back.

  --strategy decides what happens to the variants that already exist:
    replace_all     regenerate the matrix; variants of a removed combination are kept
                    as orphaned rather than lost
    add_new         add variants for new combinations only; existing ones, and a
                    combination whose variant you deleted, are left alone
    keep_as_manual  turn the existing matrix variants into manual ones, then
                    generate the new combinations

  A repeat changes nothing: when the options equal the current ones and, for
  replace_all and keep_as_manual, the live matrix variants are exactly the
  combinations the save would generate, the call writes nothing and meta reports
  zero changes. Overrides are ignored on such a no-op. Two identical saves sent at
  once: one applies, and the other is exit 8 (E9446) if it overlaps the first, or a
  no-op when it starts after the first finished.

USAGE
  capigo products options <id> --tenant <code> --strategy <strategy>
                         (--option <Name=v1,v2>... | --no-options)
                         [--overrides-file <path|->]

FLAGS
  <id>
      Product UUID. Positional, required.

  --tenant <code>
      Tenant the product belongs to. Required.

  --strategy <replace_all|add_new|keep_as_manual>
      Required.

  --option <Name=v1,v2,...>   (repeatable, at most 2)
      One option and its values, in order. The set you give replaces the product's
      options.

        capigo products options 8f2a… --tenant acme --strategy replace_all \\
          --option "Color=Red,Blue" --option "Size=S,M"

  --no-options
      Save the product with no options (instead of --option).

  --overrides-file <path|->
      A JSON array that sets fields of generated variants, matched by name, for
      example [ { "name": "Red / S", "sku": "TEE-RS", "barcode": "893…" } ]. Allowed
      fields: name (required), sku, barcode, weight, width, height, depth. A
      clashing live SKU or barcode is exit 5 and the whole save is rolled back.

OUTPUT
  The updated PRODUCT is at .data, with its options and variants, and what the save
  changed is in .meta:

      { "data": { "id": "8f2a…", "options": [ … ], "variants": [ … ] },
        "meta": { "tenant": "acme", "tenant_source": "flag", … } }

  Exit 4 for a product that does not exist or belongs to another tenant. Exit 3 for
  a key without the catalog content permission.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		switch productOptionsStrategy {
		case "replace_all", "add_new", "keep_as_manual":
		case "":
			failValidation("products options: --strategy is required (replace_all, add_new or keep_as_manual)")
		default:
			failValidation("products options: --strategy must be replace_all, add_new or keep_as_manual")
		}
		if productOptionsNone && len(productOptionsOptions) > 0 {
			failValidation("products options: give --option or --no-options, not both")
		}
		if !productOptionsNone && len(productOptionsOptions) == 0 {
			failValidation("products options: give at least one --option, or --no-options")
		}
		if len(productOptionsOptions) > 2 {
			failValidation("products options: a product has at most 2 options")
		}
		options := []map[string]any{}
		for _, raw := range productOptionsOptions {
			o, err := parseOptionFlag(raw)
			if err != nil {
				failValidation("products options: %v", err)
			}
			options = append(options, o)
		}
		body := map[string]any{"options": options, "strategy": productOptionsStrategy}
		if productOptionsOverridesFile != "" {
			raw, err := readJSONInput(productOptionsOverridesFile)
			if err != nil {
				return handleErr(fmt.Errorf("read --overrides-file: %w", err))
			}
			var overrides []json.RawMessage
			if err := json.Unmarshal(raw, &overrides); err != nil {
				failValidation("products options: --overrides-file must be a JSON array of objects: %v", err)
			}
			body["overrides"] = overrides
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(productOptionsTenant, profile)
		requireTenant(tenant, "products options")

		resp, err := client.Do(ctx, "PUT", "/pcms/products/"+url.PathEscape(args[0])+"/options", body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, productOptionsTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	productsOptionsCmd.Flags().StringVar(&productOptionsTenant, "tenant", "", "tenant the product belongs to (required)")
	productsOptionsCmd.Flags().StringVar(&productOptionsStrategy, "strategy", "", "replace_all, add_new or keep_as_manual (required)")
	productsOptionsCmd.Flags().StringArrayVar(&productOptionsOptions, "option", nil, "one option as Name=v1,v2; repeat for the second")
	productsOptionsCmd.Flags().BoolVar(&productOptionsNone, "no-options", false, "save the product with no options")
	productsOptionsCmd.Flags().StringVar(&productOptionsOverridesFile, "overrides-file", "", "JSON array of variant overrides, or - for stdin")

}
