package cmd

import "testing"

func TestPropertyDefinitionsCallTheirOwnPaths(t *testing.T) {
	const reply = `{"data":{"id":"d1","key":"material","label":"Fabric"}}`
	propDefCreateTenant, propDefCreateLabel = "acme", "Fabric"
	propDefUpdateTenant, propDefUpdateLabel = "acme", "Fabric"
	propDefReplTenant, propDefReplLabel = "acme", "Fabric"
	propDefDelTenant, propDefGetTenant = "acme", "acme"
	for _, c := range []struct {
		name         string
		cmd          func() error
		method, path string
		body         string
	}{
		{"create", func() error { return propertyDefinitionsCreateCmd.RunE(propertyDefinitionsCreateCmd, nil) }, "POST", "/pcms/property-definitions", `{"label":"Fabric"}`},
		{"update", func() error { return propertyDefinitionsUpdateCmd.RunE(propertyDefinitionsUpdateCmd, []string{"d1"}) }, "PATCH", "/pcms/property-definitions/d1", `{"label":"Fabric"}`},
		{"replace", func() error { return propertyDefinitionsReplaceCmd.RunE(propertyDefinitionsReplaceCmd, []string{"d1"}) }, "PUT", "/pcms/property-definitions/d1", `{"label":"Fabric"}`},
		{"get", func() error { return propertyDefinitionsGetCmd.RunE(propertyDefinitionsGetCmd, []string{"d1"}) }, "GET", "/pcms/property-definitions/d1", ""},
		{"delete", func() error { return propertyDefinitionsDeleteCmd.RunE(propertyDefinitionsDeleteCmd, []string{"d1"}) }, "DELETE", "/pcms/property-definitions/d1", ""},
	} {
		seen, printed := callAgainst(t, reply, func() { _ = c.cmd() })
		if seen.method != c.method || seen.path != c.path || seen.tenant != "acme" || seen.rawBody != c.body {
			t.Errorf("%s: got %s %s tenant=%q body=%q, want %s %s body=%q", c.name, seen.method, seen.path, seen.tenant, seen.rawBody, c.method, c.path, c.body)
		}
		if printed == "" {
			t.Errorf("%s: nothing printed", c.name)
		}
	}
}

func TestPropertyDefinitionsListNotPaged(t *testing.T) {
	propDefListTenant = "acme"
	seen, printed := callAgainst(t, `{"data":[{"id":"d1","key":"k","label":"L"}]}`,
		func() { _ = propertyDefinitionsListCmd.RunE(propertyDefinitionsListCmd, nil) })
	if seen.method != "GET" || seen.path != "/pcms/property-definitions" || seen.query != "" || printed == "" {
		t.Errorf("got %s %s ?%s printed=%q", seen.method, seen.path, seen.query, printed)
	}
}

func TestPropertyDefinitionsMoveBody(t *testing.T) {
	propDefMoveTenant = "acme"
	for _, c := range []struct {
		after string
		top   bool
		want  string
	}{
		{"", true, `{"after_id":null}`},
		{"a1", false, `{"after_id":"a1"}`},
	} {
		propDefMoveAfterID, propDefMoveTop = c.after, c.top
		_ = propertyDefinitionsMoveCmd.Flags().Set("after-id", c.after)
		if c.after == "" {
			propertyDefinitionsMoveCmd.Flags().Lookup("after-id").Changed = false
		}
		seen, _ := callAgainst(t, `{"data":{"id":"d1","key":"k","label":"L"}}`,
			func() { _ = propertyDefinitionsMoveCmd.RunE(propertyDefinitionsMoveCmd, []string{"d1"}) })
		if seen.path != "/pcms/property-definitions/d1/actions/move" || seen.rawBody != c.want {
			t.Errorf("got %s %q, want body %s", seen.path, seen.rawBody, c.want)
		}
	}
	propDefMoveAfterID, propDefMoveTop = "", false
	propertyDefinitionsMoveCmd.Flags().Lookup("after-id").Changed = false
}
