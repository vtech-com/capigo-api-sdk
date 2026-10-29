package credentials

import (
	"errors"
	"os"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// setEnv sets or, with present false, removes a variable for the test.
func setEnv(t *testing.T, name, value string, present bool) {
	t.Helper()
	t.Setenv(name, value)
	if !present {
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error is %T, want *api.APIError", err)
	}
	return apiErr.Code
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name                 string
		envKey               string
		envPresent           bool
		noConfig             string
		profileKey, viperKey string
		wantKey              string
		wantSource           Source
		wantCode             string
	}{
		{"absent: viper beats profile", "", false, "", "csk_file", "csk_viper", "csk_viper", SourceEnv, ""},
		{"absent: profile", "", false, "", "csk_file", "", "csk_file", SourceConfig, ""},
		{"absent: nothing", "", false, "", "", "", "", SourceNone, "AUTH_MISSING_HEADER"},
		{"env key ignores viper and profile", "csk_env", true, "", "csk_file", "csk_viper", "csk_env", SourceEnv, ""},
		{"empty env key never falls back", "", true, "", "csk_file", "csk_viper", "", SourceEnv, "AUTH_EMPTY_ENV_KEY"},
		{"no config, no env key", "", false, "1", "", "csk_viper", "", SourceNone, "AUTH_MISSING_HEADER"},
		{"no config other than 1 is absent", "", false, "true", "csk_file", "", "csk_file", SourceConfig, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setEnv(t, EnvKey, tc.envKey, tc.envPresent)
			setEnv(t, "CAPIGO_NO_CONFIG", tc.noConfig, tc.noConfig != "")

			key, source, err := Resolve(tc.profileKey, tc.viperKey)
			if tc.wantCode != "" {
				if got := codeOf(t, err); got != tc.wantCode {
					t.Errorf("code = %q, want %q", got, tc.wantCode)
				}
				if api.ExitCodeFor(err) != 2 {
					t.Errorf("exit = %d, want 2", api.ExitCodeFor(err))
				}
			} else if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if key != tc.wantKey || source != tc.wantSource {
				t.Errorf("Resolve = (%q, %q), want (%q, %q)", key, source, tc.wantKey, tc.wantSource)
			}
		})
	}
}

func TestStoreKeyRefusal(t *testing.T) {
	setEnv(t, EnvKey, "", true)
	setEnv(t, "CAPIGO_NO_CONFIG", "1", true)
	if got := StoreKeyRefusal(); got == nil || got.Code != "KEY_MANAGED_BY_ENV" {
		t.Errorf("both apply: refusal = %+v, want KEY_MANAGED_BY_ENV", got)
	}

	setEnv(t, EnvKey, "", false)
	if got := StoreKeyRefusal(); got == nil || got.Code != "CONFIG_DISABLED" {
		t.Errorf("no config only: refusal = %+v, want CONFIG_DISABLED", got)
	}
	if api.ExitCodeFor(ConfigDisabled()) != 1 || api.ExitCodeFor(KeyManagedByEnv()) != 1 {
		t.Error("refusals must exit 1")
	}

	setEnv(t, "CAPIGO_NO_CONFIG", "", false)
	if got := StoreKeyRefusal(); got != nil {
		t.Errorf("absent: refusal = %+v, want nil", got)
	}
}
