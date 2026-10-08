package cmd

import (
	"testing"

	"github.com/spf13/pflag"
)

func resetMemberUpdate(t *testing.T) {
	t.Helper()
	membersUpdateCmd.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	memberUpdatePositions, memberUpdatePermissions = nil, nil
}

func TestMembersUpdateSendsOnlyTheFlagsGiven(t *testing.T) {
	resetMemberUpdate(t)
	t.Cleanup(func() { resetMemberUpdate(t) })
	_ = membersUpdateCmd.Flags().Set("tenant", "acme")
	_ = membersUpdateCmd.Flags().Set("member-code", " EMP-001 ")
	_ = membersUpdateCmd.Flags().Set("job-title", "")

	seen, printed := callAgainst(t, `{"data":{"id":"u1","role":"member"}}`,
		func() { _ = membersUpdateCmd.RunE(membersUpdateCmd, []string{"u1"}) })

	if seen.method != "PATCH" || seen.path != "/members/u1" {
		t.Errorf("call = %s %s, want PATCH /members/u1", seen.method, seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"job_title", "member_code"}) {
		t.Errorf("body keys = %v, want job_title and member_code only", got)
	}
	if seen.body["member_code"] != "EMP-001" {
		t.Errorf("member_code = %v, want the trimmed value", seen.body["member_code"])
	}
	if v, ok := seen.body["job_title"]; !ok || v != "" {
		t.Errorf("job_title = %v, want an empty string so the API clears it", v)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestMembersUpdateSetsAndClearsLists(t *testing.T) {
	resetMemberUpdate(t)
	t.Cleanup(func() { resetMemberUpdate(t) })
	_ = membersUpdateCmd.Flags().Set("tenant", "acme")
	memberUpdatePermissions = []string{"wms_admin", "catalog_view"}
	memberUpdateClearPositions = true
	t.Cleanup(func() { memberUpdateClearPositions = false })

	seen, _ := callAgainst(t, `{"data":{"id":"u1"}}`,
		func() { _ = membersUpdateCmd.RunE(membersUpdateCmd, []string{"u1"}) })

	perms, _ := seen.body["permissions"].([]any)
	if len(perms) != 2 {
		t.Errorf("permissions = %v, want the two keys", seen.body["permissions"])
	}
	pos, ok := seen.body["positions"].([]any)
	if !ok || len(pos) != 0 {
		t.Errorf("positions = %v, want an empty array from --clear-positions", seen.body["positions"])
	}
}
