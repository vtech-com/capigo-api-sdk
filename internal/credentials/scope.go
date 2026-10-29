package credentials

import (
	"os"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// In a managed environment the key belongs to the caller the host injected it
// for, so where it is sent and which tenant it names must come from the same
// place the key did. viper's AutomaticEnv with an empty prefix lets an
// unprefixed API_URL or TENANT beat the CAPIGO_* variable, and the config file
// is shared by every caller — either would let whoever can write the host's
// environment or the file redirect every caller's key. Managed mode therefore
// reads only the flag and the CAPIGO_* variable, directly.

const (
	// EnvAPIURL is the variable an environment sets the base URL through.
	EnvAPIURL = "CAPIGO_API_URL"
	// EnvTenant is the variable an environment sets the tenant through.
	EnvTenant = "CAPIGO_TENANT"
)

// BaseURL returns the base URL to send requests to. flag is --api-url,
// viperURL is api_url as viper resolves it, profileURL the stored api_url and
// def the default host. Outside managed mode the precedence is the one the CLI
// has always had: flag, viper, profile, default. In managed mode viperURL and
// profileURL are ignored: flag, then CAPIGO_API_URL, then def — and a present
// but empty CAPIGO_API_URL is a local error rather than a fallback.
func BaseURL(flag, viperURL, profileURL, def string) (string, error) {
	if !Managed() {
		switch {
		case flag != "":
			return flag, nil
		case viperURL != "":
			return viperURL, nil
		case profileURL != "":
			return profileURL, nil
		}
		return def, nil
	}

	if flag != "" {
		return flag, nil
	}
	u, present := os.LookupEnv(EnvAPIURL)
	if !present {
		return def, nil
	}
	if u == "" {
		// The environment said "this URL" and gave none. Sending the key to the
		// default host instead would be a guess the environment did not make.
		return "", &api.APIError{
			Code:           "EMPTY_ENV_API_URL",
			Message:        "CAPIGO_API_URL is set but empty — the environment supplied no API URL, so no request was sent and no other URL was used in its place",
			LocalDiagnosis: true,
		}
	}
	return u, nil
}

// Tenant returns the tenant to send, or nil, and where it came from. flag is
// --tenant, viperTenant is tenant as viper resolves it and profileDefault the
// stored default_tenant. Outside managed mode the precedence is unchanged:
// flag, viper, profile. In managed mode only flag and CAPIGO_TENANT count; an
// empty CAPIGO_TENANT is no tenant.
func Tenant(flag, viperTenant, profileDefault string) (*string, string) {
	if Managed() {
		// The env value takes viper's slot, so ResolveTenant keeps its order.
		viperTenant, _ = os.LookupEnv(EnvTenant)
		profileDefault = ""
	}
	tenant := api.ResolveTenant(flag, viperTenant, profileDefault)
	if tenant == nil {
		return nil, ""
	}
	return tenant, TenantSource(flag, viperTenant)
}

// TenantSource names where a resolved tenant came from: "flag", "env" or
// "config". In managed mode a tenant not given by flag can only have come from
// CAPIGO_TENANT, so the answer is never "config" there.
func TenantSource(flag, viperTenant string) string {
	switch {
	case flag != "":
		return "flag"
	case Managed(), viperTenant != "":
		return "env"
	}
	return "config"
}
