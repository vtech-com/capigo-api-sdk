package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDedupeCodesTrimsDropsBlanksAndRepeats(t *testing.T) {
	got := dedupeCodes([]string{" A ", "", "A", "B", "  ", "B", "C"})
	if strings.Join(got, ",") != "A,B,C" {
		t.Errorf("dedupeCodes = %v, want A,B,C", got)
	}
}

func TestReadCodeLinesSkipsBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codes.txt")
	if err := os.WriteFile(path, []byte("A1\n\n  B2 \nC3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readCodeLines(path)
	if err != nil || strings.Join(got, ",") != "A1,B2,C3" {
		t.Errorf("readCodeLines = %v, %v", got, err)
	}
}

func TestVariantsLookupPostsBothArrays(t *testing.T) {
	variantLookupTenant = "acme"
	variantLookupSkus = []string{"SKU-A1", " SKU-A1 ", "SKU-B1"}
	variantLookupBarcodes = []string{"893"}
	t.Cleanup(func() { variantLookupTenant, variantLookupSkus, variantLookupBarcodes = "", nil, nil })

	seen, printed := callAgainst(t, `{"data":{"variants":[],"not_found":{"skus":[],"barcodes":[]}}}`,
		func() { _ = variantsLookupCmd.RunE(variantsLookupCmd, nil) })

	if seen.method != "POST" || seen.path != "/pcms/variants/lookup" || seen.tenant != "acme" {
		t.Errorf("call = %s %s tenant=%q", seen.method, seen.path, seen.tenant)
	}
	skus, _ := seen.body["skus"].([]any)
	if len(skus) != 2 {
		t.Errorf("skus = %v, want the two distinct SKUs", seen.body["skus"])
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"barcodes", "skus"}) {
		t.Errorf("body keys = %v", got)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestVariantsResolvePercentEncodesTheCode(t *testing.T) {
	variantResolveTenant = "acme"
	variantResolveCode = "https://x.vn/q?a=1&b=2+3"
	t.Cleanup(func() { variantResolveTenant, variantResolveCode = "", "" })

	seen, _ := callAgainst(t, `{"data":{"variant":{"id":"v1"},"matched_by":"qr_url"}}`,
		func() { _ = variantsResolveCmd.RunE(variantsResolveCmd, nil) })

	if seen.method != "GET" || seen.path != "/pcms/variants/resolve" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	if want := "code=https%3A%2F%2Fx.vn%2Fq%3Fa%3D1%26b%3D2%2B3"; seen.query != want {
		t.Errorf("query = %q, want %q", seen.query, want)
	}
}
