package cmd

import (
	"strings"
	"testing"
)

func TestMembersPermissionsCallTheirOwnRoute(t *testing.T) {
	for _, tc := range []struct {
		change string
		cmd    func() error
	}{
		{"add", func() error { return membersPermissionsAddCmd.RunE(membersPermissionsAddCmd, []string{" u1 "}) }},
		{"remove", func() error { return membersPermissionsRemoveCmd.RunE(membersPermissionsRemoveCmd, []string{" u1 "}) }},
	} {
		t.Run(tc.change, func(t *testing.T) {
			c := membersPermissionsAddCmd
			if tc.change == "remove" {
				c = membersPermissionsRemoveCmd
			}
			setFlags(t, c, "tenant", "acme", "permission", " catalog_view ", "permission", "po:view")

			seen, printed := callAgainst(t, `{"data":{"id":"u1","permissions":["catalog_view","po:view"]}}`,
				func() { _ = tc.cmd() })

			if want := "/members/u1/permissions/" + tc.change; seen.method != "POST" || seen.path != want {
				t.Errorf("call = %s %s, want POST %s", seen.method, seen.path, want)
			}
			if seen.tenant != "acme" {
				t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
			}
			if got := keysOf(seen.body); !sameKeys(got, []string{"permissions"}) {
				t.Errorf("body keys = %v, want permissions only", got)
			}
			perms, _ := seen.body["permissions"].([]any)
			if len(perms) != 2 || perms[0] != "catalog_view" || perms[1] != "po:view" {
				t.Errorf("permissions = %v, want the two trimmed keys in order", seen.body["permissions"])
			}
			data, meta := printedData(t, printed)
			if data["id"] != "u1" || meta["tenant"] != "acme" {
				t.Errorf("printed data=%v meta=%v, want the member under the acme tenant", data, meta)
			}
		})
	}
}

func TestPermissionsListFiltersInTheCLI(t *testing.T) {
	setFlags(t, permissionsListCmd, "module", "wms", "scope-type", "warehouse")

	seen, printed := callAgainst(t, `{"data":[
		{"key":"wms_admin","module":"wms","scope_type":"tenant","implies":[]},
		{"key":"scope.view","module":"wms","scope_type":"warehouse","implies":[]},
		{"key":"catalog_view","module":"catalog","scope_type":"tenant","implies":[]}]}`,
		func() { _ = permissionsListCmd.RunE(permissionsListCmd, nil) })

	if seen.method != "GET" || seen.path != "/permissions" || seen.query != "" {
		t.Errorf("call = %s %s?%s, want GET /permissions with no query", seen.method, seen.path, seen.query)
	}
	if seen.tenant != "" {
		t.Errorf("X-Tenant-Code = %q, want none: the catalog is not tenant-scoped", seen.tenant)
	}
	if !strings.Contains(printed, `"scope.view"`) || strings.Contains(printed, "wms_admin") || strings.Contains(printed, "catalog_view") {
		t.Errorf("printed %s, want only scope.view", printed)
	}
}
