package api

import (
	"testing"
)

func strPtr(s string) *string { return &s }

func TestResolveTenant(t *testing.T) {
	tests := []struct {
		name          string
		flag          *string
		noTenant      bool
		envVal        string
		defaultTenant string
		wantTenant    *string
		wantGlobal    bool
	}{
		{
			name:       "--tenant flag takes highest priority",
			flag:       strPtr("acme"),
			noTenant:   true,
			envVal:     "env-tenant",
			wantTenant: strPtr("acme"),
			wantGlobal: false,
		},
		{
			name:          "--no-tenant forces global mode over env and default",
			flag:          nil,
			noTenant:      true,
			envVal:        "env-tenant",
			defaultTenant: "cfg-tenant",
			wantTenant:    nil,
			wantGlobal:    true,
		},
		{
			name:          "env var used when no flag and no --no-tenant",
			flag:          nil,
			noTenant:      false,
			envVal:        "env-tenant",
			defaultTenant: "cfg-tenant",
			wantTenant:    strPtr("env-tenant"),
			wantGlobal:    false,
		},
		{
			name:          "config default_tenant used when no flag, no --no-tenant, no env",
			flag:          nil,
			noTenant:      false,
			envVal:        "",
			defaultTenant: "cfg-tenant",
			wantTenant:    strPtr("cfg-tenant"),
			wantGlobal:    false,
		},
		{
			name:          "global mode when nothing is set",
			flag:          nil,
			noTenant:      false,
			envVal:        "",
			defaultTenant: "",
			wantTenant:    nil,
			wantGlobal:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotGlobal := ResolveTenant(tt.flag, tt.noTenant, tt.envVal, tt.defaultTenant)

			if gotGlobal != tt.wantGlobal {
				t.Errorf("isGlobal = %v, want %v", gotGlobal, tt.wantGlobal)
			}

			if tt.wantTenant == nil {
				if got != nil {
					t.Errorf("tenant = %q, want nil", *got)
				}
			} else {
				if got == nil {
					t.Errorf("tenant = nil, want %q", *tt.wantTenant)
				} else if *got != *tt.wantTenant {
					t.Errorf("tenant = %q, want %q", *got, *tt.wantTenant)
				}
			}
		})
	}
}
