package cmd

import (
	"os"

	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// derefTenant returns the tenant code or "" when no tenant is resolved.
func derefTenant(t *string) string {
	if t == nil {
		return ""
	}
	return *t
}

// tenantNote explains where an implicitly resolved tenant came from —
// "from CAPIGO_TENANT" or "from config default_tenant" — so output can make
// a silently defaulted tenant visible. Returns "" when the tenant was given
// explicitly via --tenant or none was resolved at all.
func tenantNote(tenant *string, tenantFlag string) string {
	if tenant == nil || tenantFlag != "" {
		return ""
	}
	if viper.GetString("tenant") != "" {
		return "from CAPIGO_TENANT"
	}
	return "from config default_tenant"
}

// echoTenant prints the resolved tenant scope to stdout in table mode so the
// tenant a write actually ran against is always visible on the stream agents
// read. Call it via defer right after resolveTenant in write commands: error
// paths exit through os.Exit and skip the defer, so the line only annotates
// success output.
func echoTenant(tenant *string, tenantFlag string) {
	if outputMode != "table" || tenant == nil {
		return
	}
	output.WriteTenantLine(os.Stdout, *tenant, tenantNote(tenant, tenantFlag))
}
