package cmd

import (
	"bufio"
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

const variantLookupMaxCodes = 100

// readCodeLines reads one code per line, dropping blank lines.
func readCodeLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v := strings.TrimSpace(sc.Text()); v != "" {
			out = append(out, v)
		}
	}
	return out, sc.Err()
}

// dedupeCodes trims, drops blanks and removes repeats, keeping the first order.
func dedupeCodes(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

// variants lookup flags
var (
	variantLookupTenant      string
	variantLookupSkus        []string
	variantLookupBarcodes    []string
	variantLookupSkuFile     string
	variantLookupBarcodeFile string
)

var variantsLookupCmd = &cobra.Command{
	Use:   "lookup",
	Short: "Look up many variants by SKU and barcode in one call",
	Long: `Turn up to 100 SKUs and barcodes into variants in one call.

PURPOSE
  An import or a POS sync holds codes, not variant ids. This reads them all at once
  and answers the variants found plus the codes that matched nothing. Codes are
  trimmed and matched exactly, case included. A variant matched by both a SKU and a
  barcode appears once. Orphaned and deleted variants are never returned; an
  inactive variant is returned with status inactive.

  The batch reads the catalog search index, which refreshes about once a minute, so
  a variant created or deleted in the last minute may be missing or still listed.
  Use variants resolve, which reads the tables, when you need it exact.

USAGE
  capigo variants lookup --tenant <code> [--sku <sku>]... [--barcode <code>]...
                         [--sku-file <path>] [--barcode-file <path>]

FLAGS
  --tenant <code>
      Tenant to search. Required. See capigo help tenancy.

  --sku <sku>   (repeatable)
  --barcode <code>   (repeatable)
      A code to look up.

        capigo variants lookup --tenant acme --sku SKU-A1 --sku SKU-B1 --barcode 8930000000011

  --sku-file <path>, --barcode-file <path>
      A text file with one code per line. Blank lines are skipped. Use these for an
      import; they add to any --sku and --barcode given.

  At most 100 codes in total, counted after trimming and removing repeats, each at
  most 200 characters. Over that is exit 5 here, before any call.

OUTPUT
  The variants found are at .data.variants[]; the codes that matched nothing are
  at .data.not_found.skus[] and .data.not_found.barcodes[]:

      { "data": { "variants": [ { "id": "…", "sku": "SKU-A1", "barcode": "8930000000011",
                                  "status": "active", "product": { "id": "…" } } ],
                  "not_found": { "skus": ["SKU-B1"], "barcodes": [] } },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  Exit 5 when no code is given or there are more than 100.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		skus := append([]string{}, variantLookupSkus...)
		barcodes := append([]string{}, variantLookupBarcodes...)
		if variantLookupSkuFile != "" {
			more, err := readCodeLines(variantLookupSkuFile)
			if err != nil {
				failValidation("variants lookup: cannot read --sku-file: %v", err)
			}
			skus = append(skus, more...)
		}
		if variantLookupBarcodeFile != "" {
			more, err := readCodeLines(variantLookupBarcodeFile)
			if err != nil {
				failValidation("variants lookup: cannot read --barcode-file: %v", err)
			}
			barcodes = append(barcodes, more...)
		}
		skus, barcodes = dedupeCodes(skus), dedupeCodes(barcodes)
		if len(skus)+len(barcodes) == 0 {
			failValidation("variants lookup: give at least one --sku, --barcode, --sku-file or --barcode-file")
		}
		if len(skus)+len(barcodes) > variantLookupMaxCodes {
			failValidation("variants lookup: %d codes given; a call takes at most %d in total", len(skus)+len(barcodes), variantLookupMaxCodes)
		}
		for _, c := range append(append([]string{}, skus...), barcodes...) {
			if len([]rune(c)) > 200 {
				failValidation("variants lookup: a code is over 200 characters")
			}
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(variantLookupTenant, profile)
		requireTenant(tenant, "variants lookup")

		body := map[string][]string{}
		if len(skus) > 0 {
			body["skus"] = skus
		}
		if len(barcodes) > 0 {
			body["barcodes"] = barcodes
		}
		resp, err := client.Do(ctx, "POST", "/pcms/variants/lookup", body, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, variantLookupTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

// variants resolve flags
var (
	variantResolveTenant string
	variantResolveCode   string
)

var variantsResolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Turn one scanned code into one variant",
	Long: `Turn one code of any kind into one variant, the way the PCMS scan screen does.

PURPOSE
  A scanner, a POS or an agent holds a code and needs the variant. The code is tried
  in this order, stopping at the first live variant: SKU, the SKU inside a QR link,
  barcode, manufacturer code. For each kind an exact match is tried first, then a
  case-insensitive one. This reads the tables, so it is exact where variants lookup
  can lag by a minute.

  No live, non-orphaned variant is exit 4. A retired SKU (the SKU of a deleted
  variant) is also exit 4, with code E9483 and the old SKU in the message, so an
  out-of-date label can be told from a typo.

USAGE
  capigo variants resolve --tenant <code> --code <code>

FLAGS
  --tenant <code>
      Tenant to search. Required.

  --code <code>
      The code to resolve. Give it as it is: the CLI percent-encodes it, so a QR
      link's own ? and &, and a + in a code, arrive intact. At most 2048 characters.

        capigo variants resolve --tenant acme --code 8930000000011

OUTPUT
  The variant is at .data.variant and the identifier that matched is at
  .data.matched_by (sku, qr_url, barcode or manufacturer_code):

      { "data": { "variant": { "id": "…", "sku": "SKU-A1", "barcode": "8930000000011" },
                  "matched_by": "barcode" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()

		code := strings.TrimSpace(variantResolveCode)
		if code == "" {
			failValidation("variants resolve: --code is required and must not be blank")
		}
		if len([]rune(code)) > 2048 {
			failValidation("variants resolve: --code is over 2048 characters")
		}

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile := activeProfileOrEmpty(cfg)
		tenant := resolveTenant(variantResolveTenant, profile)
		requireTenant(tenant, "variants resolve")

		resp, err := client.Do(ctx, "GET", "/pcms/variants/resolve?code="+url.QueryEscape(code), nil, tenant)
		if err != nil {
			return handleErr(err)
		}
		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}
		meta := itemMeta(tenant, variantResolveTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	variantsLookupCmd.Flags().StringVar(&variantLookupTenant, "tenant", "", "tenant to search (required)")
	variantsLookupCmd.Flags().StringArrayVar(&variantLookupSkus, "sku", nil, "SKU to look up; repeatable")
	variantsLookupCmd.Flags().StringArrayVar(&variantLookupBarcodes, "barcode", nil, "barcode to look up; repeatable")
	variantsLookupCmd.Flags().StringVar(&variantLookupSkuFile, "sku-file", "", "file with one SKU per line")
	variantsLookupCmd.Flags().StringVar(&variantLookupBarcodeFile, "barcode-file", "", "file with one barcode per line")

	variantsResolveCmd.Flags().StringVar(&variantResolveTenant, "tenant", "", "tenant to search (required)")
	variantsResolveCmd.Flags().StringVar(&variantResolveCode, "code", "", "the code to resolve (required)")

	variantsCmd.AddCommand(variantsLookupCmd, variantsResolveCmd)
}
