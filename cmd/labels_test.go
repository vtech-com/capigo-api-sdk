package cmd

import "testing"

func TestLabelsListHitsEndpointWithTenant(t *testing.T) {
	labelListTenant = "acme"
	t.Cleanup(func() { labelListTenant = "" })

	seen, out := callAgainst(t, `{"data":[{"name":"urgent","color":"rose"}]}`, func() {
		if err := labelsListCmd.RunE(labelsListCmd, nil); err != nil {
			t.Errorf("RunE: %v", err)
		}
	})

	if seen.method != "GET" || seen.path != "/mission/labels" {
		t.Errorf("called %s %s, want GET /mission/labels", seen.method, seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
	if out == "" {
		t.Error("nothing printed")
	}
}
