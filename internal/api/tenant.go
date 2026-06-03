package api

// CreateTaskUsesBodyField signals that POST /mission/tasks carries the tenant
// as a body field (tenant_code) rather than the X-Tenant-Code header.
// cmd/ must embed the tenant in CreateTaskRequest and reject when nil.
const CreateTaskUsesBodyField = true

// ResolveTenant implements the tenant precedence rule:
//
//	tenantFlag  >  $CAPIGO_TENANT env  >  config default_tenant
//
// Returns the resolved tenant string, or nil if none is set.
func ResolveTenant(tenantFlag, envVal, defaultTenant string) *string {
	// Highest priority: explicit per-command --tenant flag.
	if tenantFlag != "" {
		s := tenantFlag
		return &s
	}

	// Environment variable.
	if envVal != "" {
		s := envVal
		return &s
	}

	// Config default_tenant.
	if defaultTenant != "" {
		s := defaultTenant
		return &s
	}

	return nil
}
