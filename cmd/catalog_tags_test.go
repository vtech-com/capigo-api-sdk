package cmd

import (
	"strings"
	"testing"
)

func TestCatalogTagCommandsCallTheirOwnPaths(t *testing.T) {
	productTagsTenant, variantTagsTenant, tagListTenant = "acme", "acme", "acme"
	sub := func(group string, verb string) func() error {
		parent := productsTagsCmd
		if group == "variants" {
			parent = variantsTagsCmd
		}
		for _, c := range parent.Commands() {
			if c.Name() == verb {
				return func() error { return c.RunE(c, []string{"p1", "Sale 50"}) }
			}
		}
		t.Fatalf("no %s tags %s command", group, verb)
		return nil
	}
	for _, c := range []struct {
		name         string
		run          func() error
		method, path string
	}{
		{"tags list", func() error { return tagsListCmd.RunE(tagsListCmd, nil) }, "GET", "/pcms/tags"},
		{"products tags add", sub("products", "add"), "POST", "/pcms/products/p1/tags/Sale 50"},
		{"products tags remove", sub("products", "remove"), "DELETE", "/pcms/products/p1/tags/Sale 50"},
		{"variants tags add", sub("variants", "add"), "POST", "/pcms/variants/p1/tags/Sale 50"},
		{"variants tags remove", sub("variants", "remove"), "DELETE", "/pcms/variants/p1/tags/Sale 50"},
	} {
		seen, printed := callAgainst(t, `{"data":{"id":"p1","tags":[]}}`, func() { _ = c.run() })
		if seen.method != c.method || seen.path != c.path || seen.tenant != "acme" || seen.rawBody != "" {
			t.Errorf("%s: got %s %s tenant=%q body=%q, want %s %s", c.name, seen.method, seen.path, seen.tenant, seen.rawBody, c.method, c.path)
		}
		if printed == "" {
			t.Errorf("%s: nothing printed", c.name)
		}
	}
}

func TestListTagFilterReachesTheQuery(t *testing.T) {
	variantListTenant, variantListTags = "acme", "Hè,Sale 50"
	defer func() { variantListTags = "" }()
	seen, _ := callAgainst(t, `{"data":[]}`, func() { _ = variantsListCmd.RunE(variantsListCmd, nil) })
	if !strings.Contains(seen.query, "tags=H%C3%A8%2CSale+50") {
		t.Errorf("variants list query = %q, want the tags filter", seen.query)
	}
}
