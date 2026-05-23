package api

// Endpoint-level tenant rules — cmd/ reads these instead of hard-coding logic.
const (
	// PCMSRequiresTenant signals that /pcms/* endpoints require a tenant header.
	// cmd/ must reject calls to these endpoints when no tenant is resolved.
	PCMSRequiresTenant = true

	// CreateTaskUsesBodyField signals that POST /mission/tasks carries the tenant
	// as a body field (tenant_code) rather than the X-Tenant-Code header.
	// cmd/ must embed the tenant in CreateTaskRequest and reject when global.
	CreateTaskUsesBodyField = true
)

// ResolveTenant implements the tenant precedence rule from §3.4:
//
//	--tenant flag  >  --no-tenant flag  >  $CAPIGO_TENANT env  >  config default_tenant  >  global mode
//
// Returns (tenant *string, isGlobal bool).
// When isGlobal is true, tenant is nil and no X-Tenant-Code header should be sent.
func ResolveTenant(flag *string, noTenant bool, envVal, defaultTenant string) (tenant *string, isGlobal bool) {
	// Highest priority: explicit --tenant flag.
	if flag != nil && *flag != "" {
		return flag, false
	}

	// --no-tenant forces global mode, overriding everything below.
	if noTenant {
		return nil, true
	}

	// Environment variable.
	if envVal != "" {
		return &envVal, false
	}

	// Config default_tenant.
	if defaultTenant != "" {
		return &defaultTenant, false
	}

	// Fall through to global mode.
	return nil, true
}
