package api

import (
	"testing"
)

func strPtr(s string) *string { return &s }

func TestResolveTenant(t *testing.T) {
	tests := []struct {
		name          string
		tenantFlag    string
		envVal        string
		defaultTenant string
		wantTenant    *string
	}{
		{
			name:       "--tenant flag takes highest priority",
			tenantFlag: "acme",
			envVal:     "env-tenant",
			wantTenant: strPtr("acme"),
		},
		{
			name:          "env var used when no flag",
			tenantFlag:    "",
			envVal:        "env-tenant",
			defaultTenant: "cfg-tenant",
			wantTenant:    strPtr("env-tenant"),
		},
		{
			name:          "config default_tenant used when no flag and no env",
			tenantFlag:    "",
			envVal:        "",
			defaultTenant: "cfg-tenant",
			wantTenant:    strPtr("cfg-tenant"),
		},
		{
			name:          "nil returned when nothing is set",
			tenantFlag:    "",
			envVal:        "",
			defaultTenant: "",
			wantTenant:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveTenant(tt.tenantFlag, tt.envVal, tt.defaultTenant)

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
