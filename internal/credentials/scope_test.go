package credentials

import (
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

const defURL = "https://default.test/api/v1"

// managedEnv sets the variables that decide managed mode and the CAPIGO_* scope
// variables; a nil value leaves the variable absent.
func managedEnv(t *testing.T, key, noConfig, url, tenant *string) {
	t.Helper()
	for name, v := range map[string]*string{
		EnvKey: key, "CAPIGO_NO_CONFIG": noConfig, EnvAPIURL: url, EnvTenant: tenant,
	} {
		if v == nil {
			setEnv(t, name, "", false)
		} else {
			setEnv(t, name, *v, true)
		}
	}
}

func str(s string) *string { return &s }

func TestBaseURL(t *testing.T) {
	cases := []struct {
		name                    string
		key, noConfig, envURL   *string
		flag, viperURL, profile string
		want                    string
		wantCode                string
	}{
		// Unmanaged: exactly the precedence buildClient always had.
		{name: "absent: flag wins", flag: "F", viperURL: "V", profile: "P", want: "F"},
		{name: "absent: viper beats profile", viperURL: "V", profile: "P", want: "V"},
		{name: "absent: profile", profile: "P", want: "P"},
		{name: "absent: default", want: defURL},
		{name: "absent: CAPIGO_API_URL is read through viper only", envURL: str("E"), profile: "P", want: "P"},
		{name: "absent: empty CAPIGO_API_URL is not an error", envURL: str(""), want: defURL},
		{name: "no config other than 1 is absent", noConfig: str("true"), profile: "P", want: "P"},

		// Managed: viper and profile never count.
		{name: "managed: flag wins", key: str("csk_x"), envURL: str("E"), flag: "F", viperURL: "V", profile: "P", want: "F"},
		{name: "managed: env beats viper and profile", key: str("csk_x"), envURL: str("E"), viperURL: "V", profile: "P", want: "E"},
		{name: "managed: default, never viper or profile", key: str("csk_x"), viperURL: "V", profile: "P", want: defURL},
		{name: "managed: empty env is refused", key: str("csk_x"), envURL: str(""), viperURL: "V", profile: "P", wantCode: "EMPTY_ENV_API_URL"},
		{name: "managed: flag beats an empty env", key: str("csk_x"), envURL: str(""), flag: "F", want: "F"},
		{name: "managed: empty key still managed", key: str(""), viperURL: "V", profile: "P", want: defURL},
		{name: "no config: default", noConfig: str("1"), viperURL: "V", profile: "P", want: defURL},
		{name: "no config: env", noConfig: str("1"), envURL: str("E"), viperURL: "V", want: "E"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			managedEnv(t, tc.key, tc.noConfig, tc.envURL, nil)

			got, err := BaseURL(tc.flag, tc.viperURL, tc.profile, defURL)
			if tc.wantCode != "" {
				if code := codeOf(t, err); code != tc.wantCode {
					t.Errorf("code = %q, want %q", code, tc.wantCode)
				}
				if exit := api.ExitCodeFor(err); exit != 1 {
					t.Errorf("exit = %d, want 1", exit)
				}
				if got != "" {
					t.Errorf("url = %q alongside an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("url = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTenant(t *testing.T) {
	cases := []struct {
		name                       string
		key, noConfig, envTenant   *string
		flag, viperTenant, profile string
		want, wantSource           string // want "" = nil
	}{
		// Unmanaged: unchanged precedence and labels.
		{name: "absent: flag", flag: "F", viperTenant: "V", profile: "P", want: "F", wantSource: "flag"},
		{name: "absent: viper", viperTenant: "V", profile: "P", want: "V", wantSource: "env"},
		{name: "absent: profile", profile: "P", want: "P", wantSource: "config"},
		{name: "absent: none"},
		{name: "absent: CAPIGO_TENANT is read through viper only", envTenant: str("E"), profile: "P", want: "P", wantSource: "config"},

		// Managed: viper and profile never count.
		{name: "managed: flag", key: str("csk_x"), envTenant: str("E"), flag: "F", viperTenant: "V", profile: "P", want: "F", wantSource: "flag"},
		{name: "managed: env", key: str("csk_x"), envTenant: str("E"), viperTenant: "V", profile: "P", want: "E", wantSource: "env"},
		{name: "managed: no env is none", key: str("csk_x"), viperTenant: "V", profile: "P"},
		{name: "managed: empty env is none", key: str("csk_x"), envTenant: str(""), viperTenant: "V", profile: "P"},
		{name: "no config: env", noConfig: str("1"), envTenant: str("E"), viperTenant: "V", want: "E", wantSource: "env"},
		{name: "no config: none", noConfig: str("1"), viperTenant: "V", profile: "P"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			managedEnv(t, tc.key, tc.noConfig, nil, tc.envTenant)

			got, source := Tenant(tc.flag, tc.viperTenant, tc.profile)
			if tc.want == "" {
				if got != nil {
					t.Errorf("tenant = %q, want none", *got)
				}
				if source != "" {
					t.Errorf("source = %q with no tenant", source)
				}
				return
			}
			if got == nil || *got != tc.want {
				t.Fatalf("tenant = %v, want %q", got, tc.want)
			}
			if source != tc.wantSource {
				t.Errorf("source = %q, want %q", source, tc.wantSource)
			}
		})
	}
}

func TestTenantSource(t *testing.T) {
	managedEnv(t, nil, nil, nil, nil)
	for _, tc := range []struct{ flag, viper, want string }{
		{"F", "V", "flag"}, {"", "V", "env"}, {"", "", "config"},
	} {
		if got := TenantSource(tc.flag, tc.viper); got != tc.want {
			t.Errorf("unmanaged TenantSource(%q, %q) = %q, want %q", tc.flag, tc.viper, got, tc.want)
		}
	}

	managedEnv(t, str("csk_x"), nil, nil, nil)
	if got := TenantSource("", ""); got != "env" {
		t.Errorf("managed TenantSource = %q, want env: a managed tenant is never config", got)
	}
}
