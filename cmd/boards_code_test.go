package cmd

import "testing"

func TestBoardsGetByCodeHitsCodePath(t *testing.T) {
	boardGetCode, boardGetTenant = "ACMEC-457", "acme"
	t.Cleanup(func() { boardGetCode, boardGetTenant = "", "" })

	seen, _ := callAgainst(t, `{"data":{"id":"b1","code":"ACMEC-457"}}`, func() {
		if err := boardsGetCmd.RunE(boardsGetCmd, nil); err != nil {
			t.Errorf("RunE: %v", err)
		}
	})

	if seen.path != "/mission/boards/code/ACMEC-457" {
		t.Errorf("path = %q", seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
}

func TestBoardAddressErrors(t *testing.T) {
	acme := "acme"
	cases := []struct {
		name, id, code string
		tenant         *string
		wantErr        bool
	}{
		{"id only", "b1", "", nil, false},
		{"code with tenant", "", "ACMEC-1", &acme, false},
		{"both", "b1", "ACMEC-1", &acme, true},
		{"neither", "", "", &acme, true},
		{"code without tenant", "", "ACMEC-1", nil, true},
	}
	for _, c := range cases {
		if got := boardAddressError(c.id, c.code, c.tenant) != ""; got != c.wantErr {
			t.Errorf("%s: error=%v, want %v", c.name, got, c.wantErr)
		}
	}
}

func TestBoardPath(t *testing.T) {
	if got := boardPath("", "A/B"); got != "/mission/boards/code/A%2FB" {
		t.Errorf("code path = %q", got)
	}
	if got := boardPath("b1", ""); got != "/mission/boards/b1" {
		t.Errorf("id path = %q", got)
	}
}

func TestBoardShapeErrorNeedsNoConfig(t *testing.T) {
	for _, c := range []struct {
		id, code string
		wantErr  bool
	}{{"b1", "", false}, {"", "X", false}, {"b1", "X", true}, {"", "", true}} {
		if got := boardShapeError(c.id, c.code) != ""; got != c.wantErr {
			t.Errorf("id=%q code=%q: error=%v, want %v", c.id, c.code, got, c.wantErr)
		}
	}
}
