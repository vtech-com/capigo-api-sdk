package cmd

// Black-box tests for a key the environment manages. Each test runs the built
// binary with an explicit environment, a temporary HOME and a fake API that
// records every request, because the behaviour under test is decided by which
// environment variables are present — state the in-process tests share with
// the developer's own shell.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// CAPIGO_ENVKEY_BIN runs these tests against another binary; with
// CAPIGO_ENVKEY_UPDATE_GOLDEN=1 the unchanged-behaviour goldens are rewritten
// from it. Goldens are only ever captured from the baseline binary:
//
//	CAPIGO_ENVKEY_BIN=/tmp/capigo-base CAPIGO_ENVKEY_UPDATE_GOLDEN=1 \
//	  go test ./cmd -run TestUnchangedWithoutEnvManagement
const (
	envkeyBinEnv    = "CAPIGO_ENVKEY_BIN"
	envkeyUpdateEnv = "CAPIGO_ENVKEY_UPDATE_GOLDEN"
	envkeyGoldenDir = "testdata/envkey"
	serverURLMarker = "{{API_URL}}"
)

var (
	envkeyBuildOnce sync.Once
	envkeyBuildDir  string
	envkeyBuiltBin  string
	envkeyBuildErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if envkeyBuildDir != "" {
		_ = os.RemoveAll(envkeyBuildDir)
	}
	os.Exit(code)
}

// envkeyBinary builds the CLI once per test run.
func envkeyBinary(t *testing.T) string {
	t.Helper()
	if p := os.Getenv(envkeyBinEnv); p != "" {
		return p
	}
	envkeyBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "capigo-envkey-")
		if err != nil {
			envkeyBuildErr = err
			return
		}
		envkeyBuildDir = dir
		envkeyBuiltBin = filepath.Join(dir, "capigo")
		build := exec.Command("go", "build", "-o", envkeyBuiltBin, ".")
		build.Dir = ".."
		if out, err := build.CombinedOutput(); err != nil {
			envkeyBuildErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if envkeyBuildErr != nil {
		t.Fatalf("build capigo: %v", envkeyBuildErr)
	}
	return envkeyBuiltBin
}

type recordedRequest struct {
	Path   string
	Auth   string
	Tenant string
}

// fakeAPI answers like the Public API and records what reached it.
type fakeAPI struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []recordedRequest
	// failStatus, when set by failWith, answers every request with it and failCode.
	failStatus int
	failCode   string
	// failFromPage, when set, answers that page and later with failStatus.
	failFromPage int
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.reqs = append(f.reqs, recordedRequest{
			Path:   r.URL.Path,
			Auth:   r.Header.Get("Authorization"),
			Tenant: r.Header.Get("X-Tenant-Code"),
		})
		status, code := f.failStatus, f.failCode
		if page, _ := strconv.Atoi(r.URL.Query().Get("page")); f.failFromPage > 0 && page < f.failFromPage {
			status = 0
		}
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case status != 0:
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":"rejected by the fake API"}}`, code)
		case r.Header.Get("Authorization") == "Bearer csk_bad":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"AUTH_INVALID_KEY","message":"Invalid API key"}}`))
		case r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"ok":true,"timestamp":"2026-10-27T00:00:00Z"}`))
		case r.URL.Path == "/pcms/products":
			_, _ = w.Write([]byte(`{"data":[{"id":"p1"}],"meta":{"total":2,"page":1,"limit":1,"has_more":true}}`))
		case r.URL.Path == "/tenants":
			_, _ = w.Write([]byte(`{"data":[{"tenant_code":"acme","name":"Acme"},{"tenant_code":"demo","name":"Demo"}]}`))
		default:
			_, _ = w.Write([]byte(`{"data":[],"meta":{"total":0,"limit":20,"offset":0}}`))
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) failWith(status int, code string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStatus, f.failCode = status, code
}

func (f *fakeAPI) failFrom(page, status int, code string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failFromPage, f.failStatus, f.failCode = page, status, code
}

func (f *fakeAPI) requests() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.reqs...)
}

type cliResult struct {
	stdout string
	stderr string
	code   int
}

// errorField returns one field of the error object on stdout, "" when absent.
func (r cliResult) errorField(t *testing.T, key string) string {
	t.Helper()
	var doc struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil || doc.Error == nil {
		t.Fatalf("stdout is not an error object: %v\n%s", err, r.stdout)
	}
	v, ok := doc.Error[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(v)
}

func (r cliResult) hasErrorField(t *testing.T, key string) bool {
	t.Helper()
	var doc struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil {
		return false
	}
	_, ok := doc.Error[key]
	return ok
}

// runCapigo runs the binary with exactly env — nothing is inherited.
func runCapigo(t *testing.T, env []string, args ...string) cliResult {
	t.Helper()
	cmd := exec.Command(envkeyBinary(t), args...)
	cmd.Env = env
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run capigo %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return cliResult{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

// baseEnv is HOME plus the fake API's URL, followed by extra KEY=VALUE pairs.
func baseEnv(home string, f *fakeAPI, extra ...string) []string {
	env := []string{"CAPIGO_API_URL=" + f.srv.URL}
	if home != "" {
		env = append(env, "HOME="+home)
	}
	return append(env, extra...)
}

const storedProfile = `{"version":1,"active_profile":"default","profiles":{"default":{"api_key":"csk_file","api_url":""}}}`

// homeWithConfig returns a HOME whose config file holds contents, and the
// file's path. Its mtime is moved into the past so a rewrite is visible.
func homeWithConfig(t *testing.T, contents string) (string, string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".capigo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	return home, path
}

type fileState struct {
	data  []byte
	mtime time.Time
}

func snapshot(t *testing.T, path string) fileState {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileState{data: data, mtime: info.ModTime()}
}

func assertUntouched(t *testing.T, path string, before fileState) {
	t.Helper()
	after := snapshot(t, path)
	if !bytes.Equal(before.data, after.data) {
		t.Errorf("config file was rewritten:\nbefore %s\nafter  %s", before.data, after.data)
	}
	if !before.mtime.Equal(after.mtime) {
		t.Errorf("config file mtime moved: %v -> %v", before.mtime, after.mtime)
	}
}

func assertNoConfigDir(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(home, ".capigo")); !os.IsNotExist(err) {
		t.Errorf("%s/.capigo exists after the command (stat err %v)", home, err)
	}
}

func assertRefused(t *testing.T, r cliResult, exit int, code, keySource string) {
	t.Helper()
	if r.code != exit {
		t.Errorf("exit = %d, want %d\nstdout: %s", r.code, exit, r.stdout)
	}
	if got := r.errorField(t, "code"); got != code {
		t.Errorf("code = %q, want %q", got, code)
	}
	if got := r.errorField(t, "key_source"); got != keySource {
		t.Errorf("key_source = %q, want %q", got, keySource)
	}
}

func assertNoRequests(t *testing.T, f *fakeAPI) {
	t.Helper()
	if reqs := f.requests(); len(reqs) != 0 {
		t.Errorf("%d request(s) reached the server, want none: %+v", len(reqs), reqs)
	}
}

func assertSentKey(t *testing.T, f *fakeAPI, key string) {
	t.Helper()
	reqs := f.requests()
	if len(reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	for _, r := range reqs {
		if r.Auth != "Bearer "+key {
			t.Errorf("%s sent Authorization %q, want %q", r.Path, r.Auth, "Bearer "+key)
		}
	}
}

// A1, A2
func TestLoginRefusedWhenEnvKeyPresent(t *testing.T) {
	for name, value := range map[string]string{"set": "csk_x", "empty": "", "sentinel": "csk_unlinked"} {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)

			bare := t.TempDir()
			r := runCapigo(t, baseEnv(bare, f, "CAPIGO_API_KEY="+value), "auth", "login", "--key", "csk_y")
			assertRefused(t, r, 1, "KEY_MANAGED_BY_ENV", "env")
			assertNoConfigDir(t, bare)

			home, path := homeWithConfig(t, storedProfile)
			before := snapshot(t, path)
			r = runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY="+value), "auth", "login", "--key", "csk_y")
			assertRefused(t, r, 1, "KEY_MANAGED_BY_ENV", "env")
			assertUntouched(t, path, before)
			assertNoRequests(t, f)
		})
	}
}

// A3
func TestConfigSetAPIKeyRefusedWhenEnvKeyPresent(t *testing.T) {
	f := newFakeAPI(t)
	home, path := homeWithConfig(t, storedProfile)
	before := snapshot(t, path)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x"), "config", "set", "api_key", "csk_y")
	assertRefused(t, r, 1, "KEY_MANAGED_BY_ENV", "env")
	assertUntouched(t, path, before)
	assertNoRequests(t, f)
}

// A4
func TestOtherWritesAllowedUnderEnvKey(t *testing.T) {
	withTenant := `{"version":1,"active_profile":"default","profiles":{"default":{"api_key":"csk_file","api_url":"","default_tenant":"acme"}}}`
	cases := []struct {
		name     string
		contents string
		args     []string
		check    func(p map[string]any) error
	}{
		{"config set api_url", storedProfile, []string{"config", "set", "api_url", "https://example.test/api/v1"},
			func(p map[string]any) error { return wantField(p, "api_url", "https://example.test/api/v1") }},
		{"set-default-tenant", storedProfile, []string{"config", "set-default-tenant", "acme"},
			func(p map[string]any) error { return wantField(p, "default_tenant", "acme") }},
		{"unset-default-tenant", withTenant, []string{"config", "unset-default-tenant"},
			func(p map[string]any) error { return wantField(p, "default_tenant", "") }},
		{"auth logout", storedProfile, []string{"auth", "logout"},
			func(p map[string]any) error { return wantField(p, "api_key", "") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t)
			home, path := homeWithConfig(t, tc.contents)

			r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x"), tc.args...)
			if r.code != 0 {
				t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
			}
			if err := tc.check(readProfile(t, path)); err != nil {
				t.Error(err)
			}
		})
	}
}

func readProfile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Profiles map[string]map[string]any `json:"profiles"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v\n%s", err, data)
	}
	return cfg.Profiles["default"]
}

func wantField(p map[string]any, key, want string) error {
	got, _ := p[key].(string)
	if got != want {
		return fmt.Errorf("profile %s = %q, want %q", key, got, want)
	}
	return nil
}

// A5
func TestEnvKeyTenantsListWritesNothing(t *testing.T) {
	f := newFakeAPI(t)
	home, path := homeWithConfig(t, storedProfile)
	before := snapshot(t, path)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x"), "tenants", "list")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	assertUntouched(t, path, before)
	assertSentKey(t, f, "csk_x")
}

// B1
func TestEmptyEnvKeyDoesNotFallBack(t *testing.T) {
	f := newFakeAPI(t)
	home, _ := homeWithConfig(t, storedProfile)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY="), "health")
	assertRefused(t, r, 2, "AUTH_EMPTY_ENV_KEY", "env")
	assertNoRequests(t, f)
}

// B2: under a host, API_KEY must not let a caller replace the injected key.
func TestEnvKeyBeatsViperAutomaticEnv(t *testing.T) {
	f := newFakeAPI(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f, "CAPIGO_API_KEY=csk_x", "API_KEY=csk_other"), "health")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	assertSentKey(t, f, "csk_x")
}

// C1
func TestNoConfigRefusesWrites(t *testing.T) {
	commands := map[string][]string{
		"auth login":           {"auth", "login", "--key", "csk_y"},
		"auth logout":          {"auth", "logout"},
		"config set":           {"config", "set", "api_url", "https://example.test/api/v1"},
		"set-default-tenant":   {"config", "set-default-tenant", "acme"},
		"unset-default-tenant": {"config", "unset-default-tenant"},
	}
	for name, args := range commands {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)

			bare := t.TempDir()
			r := runCapigo(t, baseEnv(bare, f, "CAPIGO_NO_CONFIG=1"), args...)
			assertRefused(t, r, 1, "CONFIG_DISABLED", "none")
			assertNoConfigDir(t, bare)

			home, path := homeWithConfig(t, storedProfile)
			before := snapshot(t, path)
			r = runCapigo(t, baseEnv(home, f, "CAPIGO_NO_CONFIG=1"), args...)
			assertRefused(t, r, 1, "CONFIG_DISABLED", "none")
			assertUntouched(t, path, before)
			assertNoRequests(t, f)
		})
	}
}

// C2
func TestNoConfigGetReportsDisabled(t *testing.T) {
	f := newFakeAPI(t)
	home, path := homeWithConfig(t, storedProfile)
	before := snapshot(t, path)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_NO_CONFIG=1"), "config", "get", "default_tenant")
	assertRefused(t, r, 1, "CONFIG_DISABLED", "none")
	assertUntouched(t, path, before)
}

// C3
func TestNoConfigTenantsListWritesNothing(t *testing.T) {
	f := newFakeAPI(t)
	home, path := homeWithConfig(t, storedProfile)
	before := snapshot(t, path)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_NO_CONFIG=1", "CAPIGO_API_KEY=csk_x"), "tenants", "list")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	assertUntouched(t, path, before)
	assertSentKey(t, f, "csk_x")
}

// C4: a corrupt file proves it is never read, not only that its key is unused.
func TestNoConfigIgnoresStoredKey(t *testing.T) {
	for name, contents := range map[string]string{"stored key": storedProfile, "corrupt file": "{not json"} {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			home, path := homeWithConfig(t, contents)
			before := snapshot(t, path)

			r := runCapigo(t, baseEnv(home, f, "CAPIGO_NO_CONFIG=1"), "health")
			assertRefused(t, r, 2, "AUTH_MISSING_HEADER", "none")
			assertNoRequests(t, f)
			assertUntouched(t, path, before)
		})
	}
}

// C5
func TestNoConfigNeedsNoHome(t *testing.T) {
	f := newFakeAPI(t)

	r := runCapigo(t, baseEnv("", f, "CAPIGO_NO_CONFIG=1", "CAPIGO_API_KEY=csk_x"), "health")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", r.code, r.stdout, r.stderr)
	}
	assertSentKey(t, f, "csk_x")
}

// C6
func TestNoConfigOnlyExactOne(t *testing.T) {
	for _, value := range []string{"0", "", "true"} {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			f := newFakeAPI(t)
			home, _ := homeWithConfig(t, storedProfile)

			r := runCapigo(t, baseEnv(home, f, "CAPIGO_NO_CONFIG="+value), "health")
			if r.code != 0 {
				t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
			}
			assertSentKey(t, f, "csk_file")
		})
	}
}

// D1
func TestManagedAuthErrorsSteerToEnvironment(t *testing.T) {
	cases := []struct {
		status int
		code   string
		exit   int
	}{
		{401, "AUTH_INVALID_KEY", 2},
		{401, "AUTH_INVALID_KEY_PREFIX", 2},
		{401, "AUTH_INVALID_FORMAT", 2},
		{401, "E0004", 2},
		{403, "AUTH_TENANT_MISMATCH", 3},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			f := newFakeAPI(t)
			f.failWith(tc.status, tc.code)

			r := runCapigo(t, baseEnv(t.TempDir(), f, "CAPIGO_API_KEY=csk_x"), "boards", "list", "--tenant", "demo")
			assertRefused(t, r, tc.exit, tc.code, "env")

			managed, ok := api.LookupErrorFor(tc.code, true)
			if !ok {
				t.Fatalf("%s has no catalog entry", tc.code)
			}
			plain, _ := api.LookupError(tc.code)
			if managed.Next == plain.Next {
				t.Fatalf("%s has no managed next step", tc.code)
			}
			if got := r.errorField(t, "next"); got != managed.Next {
				t.Errorf("next = %q\nwant the managed text %q", got, managed.Next)
			}
		})
	}
}

// A key rejected mid-sweep: the partial document still steers to the environment.
func TestManagedAuthErrorMidSweepSteersToEnvironment(t *testing.T) {
	f := newFakeAPI(t)
	f.failFrom(2, 401, "AUTH_INVALID_KEY")

	r := runCapigo(t, baseEnv(t.TempDir(), f, "CAPIGO_API_KEY=csk_x"), "products", "list", "--all", "--tenant", "demo")
	if got := len(f.requests()); got != 2 {
		t.Fatalf("%d requests, want 2 (page 1 served, page 2 rejected)", got)
	}
	assertRefused(t, r, 2, "AUTH_INVALID_KEY", "env")
	managed, _ := api.LookupErrorFor("AUTH_INVALID_KEY", true)
	if got := r.errorField(t, "next"); got != managed.Next {
		t.Errorf("next = %q\nwant the managed text %q", got, managed.Next)
	}
}

// R4
func TestEnvKeyWorksOnBareHome(t *testing.T) {
	f := newFakeAPI(t)
	home := t.TempDir()
	env := baseEnv(home, f, "CAPIGO_API_KEY=csk_x")

	if r := runCapigo(t, env, "tenants", "list"); r.code != 0 {
		t.Fatalf("tenants list exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	if r := runCapigo(t, env, "boards", "list", "--tenant", "demo"); r.code != 0 {
		t.Fatalf("boards list exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	assertSentKey(t, f, "csk_x")
	assertNoConfigDir(t, home)
}

// U7: the one deliberate change with both variables absent.
func TestBareHomeViperKeyNoLongerFailsOnProfile(t *testing.T) {
	f := newFakeAPI(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f, "API_KEY=csk_z"), "boards", "list", "--tenant", "demo")
	if r.code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s", r.code, r.stdout)
	}
	assertSentKey(t, f, "csk_z")
}

// U1–U6: with both variables absent, stdout is byte-equal to the baseline's.
func TestUnchangedWithoutEnvManagement(t *testing.T) {
	update := os.Getenv(envkeyUpdateEnv) == "1"
	if update && os.Getenv(envkeyBinEnv) == "" {
		t.Fatalf("%s=1 needs %s naming the baseline binary", envkeyUpdateEnv, envkeyBinEnv)
	}

	cases := []struct {
		name     string
		golden   string
		contents string // "" = bare HOME
		args     []string
		exit     int
		after    func(t *testing.T, f *fakeAPI, path string)
	}{
		{
			name: "U1 login writes file", golden: "U1-login.golden",
			args: []string{"auth", "login", "--key", "csk_file"}, exit: 0,
			after: func(t *testing.T, f *fakeAPI, path string) {
				if err := wantField(readProfile(t, path), "api_key", "csk_file"); err != nil {
					t.Error(err)
				}
				assertNoRequests(t, f)
			},
		},
		{
			name: "U2 health uses file key", golden: "U2-health.golden", contents: storedProfile,
			args: []string{"health"}, exit: 0,
			after: func(t *testing.T, f *fakeAPI, _ string) { assertSentKey(t, f, "csk_file") },
		},
		{
			name: "U3 tenants list merges known_tenants", golden: "U3-tenants-list.golden", contents: storedProfile,
			args: []string{"tenants", "list"}, exit: 0,
			after: func(t *testing.T, _ *fakeAPI, path string) {
				known, _ := readProfile(t, path)["known_tenants"].([]any)
				if fmt.Sprint(known) != "[acme demo]" {
					t.Errorf("known_tenants = %v, want [acme demo]", known)
				}
			},
		},
		{
			name: "U4 rejected key", golden: "U4-invalid-key.golden",
			contents: `{"version":1,"active_profile":"default","profiles":{"default":{"api_key":"csk_bad","api_url":""}}}`,
			args:     []string{"health"}, exit: 2,
		},
		{
			name: "U5 no key configured", golden: "U5-missing-key.golden",
			args: []string{"health"}, exit: 2,
			after: func(t *testing.T, f *fakeAPI, _ string) { assertNoRequests(t, f) },
		},
		{
			name: "U6 config set api_key", golden: "U6-config-set-api-key.golden", contents: storedProfile,
			args: []string{"config", "set", "api_key", "csk_y"}, exit: 5,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t)
			home := t.TempDir()
			path := filepath.Join(home, ".capigo", "config.json")
			if tc.contents != "" {
				home, path = homeWithConfig(t, tc.contents)
			}

			r := runCapigo(t, baseEnv(home, f), tc.args...)
			if r.code != tc.exit {
				t.Errorf("exit = %d, want %d\nstdout: %s", r.code, tc.exit, r.stdout)
			}
			if r.hasErrorField(t, "key_source") {
				t.Error("key_source present with both variables absent")
			}
			got := strings.ReplaceAll(r.stdout, f.srv.URL, serverURLMarker)

			goldenPath := filepath.Join(envkeyGoldenDir, tc.golden)
			if update {
				if err := os.MkdirAll(envkeyGoldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got != string(want) {
				t.Errorf("stdout differs from the baseline golden %s\ngot:\n%s\nwant:\n%s", goldenPath, got, want)
			}
			if tc.after != nil {
				tc.after(t, f, path)
			}
		})
	}
}
