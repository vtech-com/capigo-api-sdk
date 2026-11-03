package cmd

import "testing"

func TestVatPoliciesCallTheirOwnPaths(t *testing.T) {
	const reply = `{"data":{"id":"v1","name":"Standard","rate":8}}`
	vatPolicyCreateTenant, vatPolicyCreateName, vatPolicyCreateRate = "acme", "Standard", 8
	vatPolicyUpdateTenant, vatPolicyUpdateName = "acme", "Zero"
	vatPolicyReplTenant, vatPolicyReplName, vatPolicyReplRate = "acme", "Standard", 0
	vatPolicyDelTenant, vatPolicyGetTenant = "acme", "acme"
	for _, c := range []struct {
		name         string
		cmd          func() error
		method, path string
		body         string
	}{
		{"create", func() error {
			_ = vatPoliciesCreateCmd.Flags().Set("rate", "8")
			return vatPoliciesCreateCmd.RunE(vatPoliciesCreateCmd, nil)
		}, "POST", "/pcms/vat-policies", `{"name":"Standard","rate":8}`},
		{"update", func() error {
			_ = vatPoliciesUpdateCmd.Flags().Set("name", "Zero")
			return vatPoliciesUpdateCmd.RunE(vatPoliciesUpdateCmd, []string{"v1"})
		}, "PATCH", "/pcms/vat-policies/v1", `{"name":"Zero"}`},
		{"replace", func() error {
			_ = vatPoliciesReplaceCmd.Flags().Set("rate", "0")
			return vatPoliciesReplaceCmd.RunE(vatPoliciesReplaceCmd, []string{"v1"})
		}, "PUT", "/pcms/vat-policies/v1", `{"name":"Standard","rate":0}`},
		{"get", func() error { return vatPoliciesGetCmd.RunE(vatPoliciesGetCmd, []string{"v1"}) }, "GET", "/pcms/vat-policies/v1", ""},
		{"delete", func() error { return vatPoliciesDeleteCmd.RunE(vatPoliciesDeleteCmd, []string{"v1"}) }, "DELETE", "/pcms/vat-policies/v1", ""},
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

func TestVatPoliciesUpdateSendsZeroRate(t *testing.T) {
	vatPolicyUpdateTenant, vatPolicyUpdateRate = "acme", 0
	_ = vatPoliciesUpdateCmd.Flags().Set("rate", "0")
	t.Cleanup(func() { vatPoliciesUpdateCmd.Flags().Lookup("rate").Changed = false })
	vatPoliciesUpdateCmd.Flags().Lookup("name").Changed = false
	seen, _ := callAgainst(t, `{"data":{"id":"v1","name":"Z","rate":0}}`,
		func() { _ = vatPoliciesUpdateCmd.RunE(vatPoliciesUpdateCmd, []string{"v1"}) })
	if seen.rawBody != `{"rate":0}` {
		t.Errorf("body = %q, want {\"rate\":0}", seen.rawBody)
	}
}

func TestVatPoliciesListNotPaged(t *testing.T) {
	vatPolicyListTenant = "acme"
	seen, printed := callAgainst(t, `{"data":[{"id":"v1","name":"S","rate":8}]}`,
		func() { _ = vatPoliciesListCmd.RunE(vatPoliciesListCmd, nil) })
	if seen.method != "GET" || seen.path != "/pcms/vat-policies" || seen.query != "" || printed == "" {
		t.Errorf("got %s %s ?%s printed=%q", seen.method, seen.path, seen.query, printed)
	}
}

func TestVatPolicyIDReachesProductAndTypeBodies(t *testing.T) {
	const reply = `{"data":{"id":"x"}}`
	productTypeCreateTenant, productTypeCreateName, productTypeCreateVATPolicyID = "acme", "T", "v1"
	productTypeUpdateTenant, productTypeUpdateVATPolicyID, productTypeUpdateClearVATPolicy = "acme", "", true
	productTypeReplaceTenant, productTypeReplaceName, productTypeReplaceNoDescription = "acme", "T", true
	productTypeReplaceVATPolicyID, productTypeReplaceNoVATPolicy = "v1", false
	productCreateTenant, productCreateName, productCreateUnitID, productCreateVATPolicyID = "acme", "P", "u1", "v1"
	productUpdateTenant, productUpdateVATPolicyID, productUpdateClearVATPolicy = "acme", "", true
	t.Cleanup(func() {
		productTypeCreateVATPolicyID, productTypeUpdateClearVATPolicy, productTypeReplaceVATPolicyID = "", false, ""
		productTypeReplaceNoDescription = false
		productCreateVATPolicyID, productUpdateClearVATPolicy = "", false
	})
	for _, c := range []struct {
		name, method, body string
		run                func() error
	}{
		{"types create", "POST", `{"name":"T","vat_policy_id":"v1"}`, func() error { return productTypesCreateCmd.RunE(productTypesCreateCmd, nil) }},
		{"types update clear", "PATCH", `{"vat_policy_id":null}`, func() error { return productTypesUpdateCmd.RunE(productTypesUpdateCmd, []string{"t1"}) }},
		{"types replace", "PUT", `{"name":"T","description":null,"vat_policy_id":"v1"}`, func() error {
			_ = productTypesReplaceCmd.Flags().Set("no-description", "true")
			return productTypesReplaceCmd.RunE(productTypesReplaceCmd, []string{"t1"})
		}},
		{"products create", "POST", `{"name":"P","unit_id":"u1","vat_policy_id":"v1"}`, func() error { return productsCreateCmd.RunE(productsCreateCmd, nil) }},
		{"products update clear", "PUT", `{"vat_policy_id":null}`, func() error { return productsUpdateCmd.RunE(productsUpdateCmd, []string{"p1"}) }},
	} {
		seen, _ := callAgainst(t, reply, func() { _ = c.run() })
		if seen.method != c.method || seen.rawBody != c.body {
			t.Errorf("%s: got %s %q, want %s %q", c.name, seen.method, seen.rawBody, c.method, c.body)
		}
	}
}
