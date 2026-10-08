package cmd

import (
	"strings"
	"testing"
)

func TestParseOptionFlag(t *testing.T) {
	got, err := parseOptionFlag(" Color = Red, Blue ,, ")
	if err != nil {
		t.Fatal(err)
	}
	values := got["values"].([]string)
	if got["name"] != "Color" || strings.Join(values, "|") != "Red|Blue" {
		t.Errorf("parseOptionFlag = %v", got)
	}
	for _, bad := range []string{"Color", "=Red", "Color=", "Color= , "} {
		if _, err := parseOptionFlag(bad); err == nil {
			t.Errorf("parseOptionFlag(%q) accepted a bad option", bad)
		}
	}
	if _, err := parseOptionFlag("N=" + strings.Repeat("v,", 11)); err == nil {
		t.Error("an option of 11 values was accepted")
	}
}

func TestProductsOptionsSendsOptionsAndStrategy(t *testing.T) {
	productOptionsTenant, productOptionsStrategy = "acme", "replace_all"
	productOptionsOptions = []string{"Color=Red,Blue", "Size=S,M"}
	t.Cleanup(func() { productOptionsTenant, productOptionsStrategy, productOptionsOptions = "", "", nil })

	seen, printed := callAgainst(t, `{"data":{"id":"p1"}}`,
		func() { _ = productsOptionsCmd.RunE(productsOptionsCmd, []string{"p1"}) })

	if seen.method != "PUT" || seen.path != "/pcms/products/p1/options" || seen.tenant != "acme" {
		t.Errorf("call = %s %s tenant=%q", seen.method, seen.path, seen.tenant)
	}
	if seen.body["strategy"] != "replace_all" {
		t.Errorf("strategy = %v", seen.body["strategy"])
	}
	opts, _ := seen.body["options"].([]any)
	if len(opts) != 2 {
		t.Errorf("options = %v, want two", seen.body["options"])
	}
	if _, has := seen.body["overrides"]; has {
		t.Error("overrides sent although none was given")
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestProductsOptionsNoOptionsSendsAnEmptyArray(t *testing.T) {
	productOptionsTenant, productOptionsStrategy, productOptionsNone = "acme", "keep_as_manual", true
	t.Cleanup(func() { productOptionsTenant, productOptionsStrategy, productOptionsNone = "", "", false })

	seen, _ := callAgainst(t, `{"data":{"id":"p1"}}`,
		func() { _ = productsOptionsCmd.RunE(productsOptionsCmd, []string{"p1"}) })

	opts, ok := seen.body["options"].([]any)
	if !ok || len(opts) != 0 {
		t.Errorf("options = %v, want an empty array", seen.body["options"])
	}
}

func TestProductsVariantsAppendsDeleteItems(t *testing.T) {
	productVariantsTenant, productVariantsProductID = "acme", "p1"
	productVariantsDelete = []string{"v1", " v2 "}
	t.Cleanup(func() { productVariantsTenant, productVariantsProductID, productVariantsDelete = "", "", nil })

	seen, _ := callAgainstArray(t, `{"data":{"id":"p1"}}`,
		func() { _ = productsVariantsCmd.RunE(productsVariantsCmd, nil) })

	if seen.method != "PUT" || seen.path != "/pcms/products/p1/variants" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	if seen.rawBody != `[{"_delete":true,"variant_id":"v1"},{"_delete":true,"variant_id":"v2"}]` {
		t.Errorf("body = %s", seen.rawBody)
	}
}

func TestVariantsListSendsProductAndStatus(t *testing.T) {
	variantListTenant, variantListProductID, variantListStatus = "acme", "p1", "inactive"
	t.Cleanup(func() { variantListTenant, variantListProductID, variantListStatus = "", "", "" })

	seen, _ := callAgainst(t, `{"data":[],"meta":{"page":1,"limit":20,"total":0,"has_more":false}}`,
		func() { _ = variantsListCmd.RunE(variantsListCmd, nil) })

	for _, want := range []string{"product_id=p1", "status=inactive"} {
		if !strings.Contains(seen.query, want) {
			t.Errorf("query = %q, want it to carry %s", seen.query, want)
		}
	}
}
