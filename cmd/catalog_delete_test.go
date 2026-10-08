package cmd

import "testing"

func TestStructureDeleteCommandsCallTheirOwnPath(t *testing.T) {
	for _, c := range []struct {
		name, path string
	}{
		{"brands", "/pcms/brands/b1"},
		{"categories", "/pcms/categories/b1"},
		{"product-types", "/pcms/product-types/b1"},
		{"units", "/pcms/units/b1"},
	} {
		var run func(args []string)
		var setTenant func()
		switch c.name {
		case "brands":
			run = func(a []string) { _ = brandsDeleteCmd.RunE(brandsDeleteCmd, a) }
			setTenant = func() { _ = brandsDeleteCmd.Flags().Set("tenant", "acme") }
		case "categories":
			run = func(a []string) { _ = categoriesDeleteCmd.RunE(categoriesDeleteCmd, a) }
			setTenant = func() { _ = categoriesDeleteCmd.Flags().Set("tenant", "acme") }
		case "product-types":
			run = func(a []string) { _ = productTypesDeleteCmd.RunE(productTypesDeleteCmd, a) }
			setTenant = func() { _ = productTypesDeleteCmd.Flags().Set("tenant", "acme") }
		default:
			run = func(a []string) { _ = unitsDeleteCmd.RunE(unitsDeleteCmd, a) }
			setTenant = func() { _ = unitsDeleteCmd.Flags().Set("tenant", "acme") }
		}
		setTenant()

		seen, printed := callAgainst(t, `{"data":{"id":"b1"}}`, func() { run([]string{"b1"}) })

		if seen.method != "DELETE" || seen.path != c.path || seen.tenant != "acme" || seen.hasBody {
			t.Errorf("%s: call = %s %s tenant=%q hasBody=%v, want DELETE %s", c.name, seen.method, seen.path, seen.tenant, seen.hasBody, c.path)
		}
		if printed == "" {
			t.Errorf("%s: nothing printed", c.name)
		}
	}
}

func TestCategoriesListSendsParentFilters(t *testing.T) {
	for _, c := range []struct {
		parent string
		root   bool
		want   string
	}{
		{"p1", false, "parent_id=p1"},
		{"", true, "parent_id=null"},
	} {
		categoryListTenant, categoryListParent, categoryListRoot = "acme", c.parent, c.root
		seen, _ := callAgainst(t, `{"data":[],"meta":{"page":1,"limit":20,"total":0,"has_more":false}}`,
			func() { _ = categoriesListCmd.RunE(categoriesListCmd, nil) })
		categoryListTenant, categoryListParent, categoryListRoot = "", "", false

		if got := seen.query; len(got) < len(c.want) || !containsParam(got, c.want) {
			t.Errorf("query = %q, want it to carry %s", got, c.want)
		}
	}
}

func containsParam(query, want string) bool {
	for _, p := range splitAmp(query) {
		if p == want {
			return true
		}
	}
	return false
}

func splitAmp(q string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(q); i++ {
		if i == len(q) || q[i] == '&' {
			out = append(out, q[start:i])
			start = i + 1
		}
	}
	return out
}
