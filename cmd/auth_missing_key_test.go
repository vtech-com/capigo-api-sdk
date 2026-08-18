package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
)

// withConfig points the config store at a temp file for the duration of a test,
// writing contents when it is non-empty (an empty string means "no config file
// at all" — the state of a machine where auth login has never run).
func withConfig(t *testing.T, contents string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if contents != "" {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	orig := config.ConfigPath
	config.ConfigPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { config.ConfigPath = orig })

	// The key otherwise leaks in from the developer's own environment.
	t.Setenv("CAPIGO_API_KEY", "")
	viper.Reset()
	_ = viper.BindEnv("api_key", "CAPIGO_API_KEY")
	_ = viper.BindEnv("api_url", "CAPIGO_API_URL")
	t.Cleanup(viper.Reset)
}

// The first run of a fresh install must name the situation the caller is in —
// no key — rather than the shape of a config file they have never seen.
func TestBuildClient_NoConfigReportsMissingKey(t *testing.T) {
	cases := map[string]string{
		"no config file at all":       "",
		"profile exists, key cleared": `{"active_profile":"default","profiles":{"default":{}}}`,
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			withConfig(t, contents)

			_, _, err := buildClient()
			if err == nil {
				t.Fatal("buildClient succeeded with no API key configured")
			}
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error is %T, want *api.APIError", err)
			}
			if apiErr.Code != "AUTH_MISSING_HEADER" {
				t.Errorf("code = %q, want AUTH_MISSING_HEADER", apiErr.Code)
			}
			if !apiErr.LocalDiagnosis {
				t.Error("LocalDiagnosis = false; the catalog interpretation would be withheld")
			}
			if got := api.ExitCodeFor(err); got != 2 {
				t.Errorf("exit code = %d, want 2 (auth)", got)
			}
		})
	}
}

// A key in the environment is a complete credential on its own: a machine that
// has never run auth login must still work when CAPIGO_API_KEY is set.
func TestBuildClient_EnvKeyNeedsNoConfigFile(t *testing.T) {
	withConfig(t, "")
	t.Setenv("CAPIGO_API_KEY", "csk_from_env")

	client, _, err := buildClient()
	if err != nil {
		t.Fatalf("buildClient: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}

// An active_profile naming a profile that was never written is a real config
// fault, and keeps its own message — it is not the same thing as "no key".
func TestBuildClient_MissingNamedProfileKeepsItsOwnError(t *testing.T) {
	withConfig(t, `{"active_profile":"staging","profiles":{"default":{"api_key":"csk_x"}}}`)

	_, _, err := buildClient()
	if err == nil {
		t.Fatal("buildClient succeeded with a dangling active_profile")
	}
	if !strings.Contains(err.Error(), `profile "staging" not found`) {
		t.Errorf("error = %q, want it to name the missing profile", err)
	}
}

// The diagnostic block is what an agent reads. A locally-detected error whose
// code is a real catalog code must still carry the catalog's Next step.
func TestRenderCLIError_LocalDiagnosisCarriesCatalogNext(t *testing.T) {
	stdout := captureStdout(t, func() {
		renderCLIError(&api.APIError{
			Code:           "AUTH_MISSING_HEADER",
			Message:        "no API key configured",
			HTTPStatus:     401,
			LocalDiagnosis: true,
		})
	})

	var doc struct {
		Error struct {
			Code string `json:"code"`
			Next string `json:"next"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if doc.Error.Code != "AUTH_MISSING_HEADER" {
		t.Errorf("code = %q, want AUTH_MISSING_HEADER", doc.Error.Code)
	}
	if !strings.Contains(doc.Error.Next, "auth login") {
		t.Errorf("next = %q, want the auth login instruction", doc.Error.Next)
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	_ = w.Close()
	os.Stdout = orig
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}
