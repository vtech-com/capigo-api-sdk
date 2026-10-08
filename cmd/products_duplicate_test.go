package cmd

import "testing"

func TestProductsDuplicateSendsNameAndKey(t *testing.T) {
	productDuplicateTenant, productDuplicateIdempotencyKey = "acme", "dup-1"
	_ = productsDuplicateCmd.Flags().Set("name", "  Red tee (copy) ")
	_ = productsDuplicateCmd.Flags().Set("idempotency-key", "dup-1")
	t.Cleanup(func() {
		productDuplicateTenant, productDuplicateIdempotencyKey, productDuplicateName = "", "", ""
		productsDuplicateCmd.Flags().Lookup("name").Changed = false
		productsDuplicateCmd.Flags().Lookup("idempotency-key").Changed = false
	})

	seen, printed := callAgainst(t, `{"data":{"id":"p2","status":"DRAFT"}}`,
		func() { _ = productsDuplicateCmd.RunE(productsDuplicateCmd, []string{"p1"}) })

	if seen.method != "POST" || seen.path != "/pcms/products/p1/actions/duplicate" || seen.tenant != "acme" {
		t.Errorf("call = %s %s tenant=%q", seen.method, seen.path, seen.tenant)
	}
	if seen.idemKey != "dup-1" {
		t.Errorf("Idempotency-Key = %q, want dup-1", seen.idemKey)
	}
	if seen.body["name"] != "Red tee (copy)" {
		t.Errorf("name = %v, want the trimmed name", seen.body["name"])
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestProductsDuplicateOmitsTheBodyWithoutAName(t *testing.T) {
	productDuplicateTenant = "acme"
	t.Cleanup(func() { productDuplicateTenant = "" })

	seen, _ := callAgainst(t, `{"data":{"id":"p2"}}`,
		func() { _ = productsDuplicateCmd.RunE(productsDuplicateCmd, []string{"p1"}) })

	if seen.hasBody {
		t.Errorf("a body was sent without --name: %v", seen.body)
	}
	if seen.idemKey != "" {
		t.Errorf("Idempotency-Key = %q, want none when the flag is not given", seen.idemKey)
	}
}
