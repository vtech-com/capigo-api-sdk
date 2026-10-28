package cmd

// Black-box tests for where a managed key is sent, and with which tenant. In a
// managed environment the base URL comes only from --api-url, CAPIGO_API_URL or
// the default host, and the tenant only from --tenant or CAPIGO_TENANT: an
// unprefixed API_URL or TENANT, or a value stored in the config file, would let
// whoever can write the host's environment or the file redirect every caller's
// key. Every environment here routes the default host through a recording
// proxy, so a fallback to it is observed and never reaches the real API.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const evilURLProfile = `{"version":1,"active_profile":"default","profiles":{"default":{"api_key":"csk_file","api_url":"%s"}}}`

const otherTenantProfile = `{"version":1,"active_profile":"default","profiles":{"default":{"api_key":"csk_file","api_url":"","default_tenant":"other"}}}`

// metaField returns one field of meta on stdout, "" when absent.
func (r cliResult) metaField(t *testing.T, key string) string {
	t.Helper()
	var doc struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil {
		t.Fatalf("stdout is not a JSON document: %v\n%s", err, r.stdout)
	}
	v, ok := doc.Meta[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(v)
}

func assertExit(t *testing.T, r cliResult, want int) {
	t.Helper()
	if r.code != want {
		t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", r.code, want, r.stdout, r.stderr)
	}
}

func assertNoProxyTraffic(t *testing.T, p *recordingProxy) {
	t.Helper()
	if seen := p.seen(); len(seen) != 0 {
		t.Errorf("the proxy saw %v, want nothing", seen)
	}
}

// assertTenantNeverSent fails when any request carried tenant, or stdout names it.
func assertTenantNeverSent(t *testing.T, f *fakeAPI, r cliResult, tenant string) {
	t.Helper()
	for _, req := range f.requests() {
		if req.Tenant == tenant {
			t.Errorf("%s sent X-Tenant-Code %q", req.Path, req.Tenant)
		}
	}
	if strings.Contains(r.stdout, tenant) {
		t.Errorf("stdout names tenant %q:\n%s", tenant, r.stdout)
	}
}

// L1
func TestManagedURLIgnoresUnprefixedAPIURL(t *testing.T) {
	good, evil, p := newFakeAPI(t), newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), good, "CAPIGO_API_KEY=csk_x", "API_URL="+evil.srv.URL, p.env()), "health")
	assertExit(t, r, 0)
	assertSentKey(t, good, "csk_x")
	assertNoRequests(t, evil)
	assertNoProxyTraffic(t, p)
}

// L2
func TestManagedURLIgnoresStoredAPIURL(t *testing.T) {
	evil, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, fmt.Sprintf(evilURLProfile, evil.srv.URL))

	runCapigo(t, homeEnv(home, "CAPIGO_API_KEY=csk_x", p.env()), "health")
	assertNoRequests(t, evil)
	if seen := p.seen(); len(seen) != 1 || seen[0] != "CONNECT platform.capigo.app:443" {
		t.Errorf("the proxy saw %v, want [CONNECT platform.capigo.app:443]", seen)
	}
}

// L3
func TestNoConfigURLIgnoresUnprefixedAPIURL(t *testing.T) {
	good, evil, p := newFakeAPI(t), newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), good,
		"CAPIGO_NO_CONFIG=1", "CAPIGO_API_KEY=csk_x", "API_URL="+evil.srv.URL, p.env()), "health")
	assertExit(t, r, 0)
	assertSentKey(t, good, "csk_x")
	assertNoRequests(t, evil)
	assertNoProxyTraffic(t, p)
}

// L4
func TestManagedEmptyEnvURLRefused(t *testing.T) {
	evil, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, fmt.Sprintf(evilURLProfile, evil.srv.URL))

	r := runCapigo(t, homeEnv(home, "CAPIGO_API_KEY=csk_x", "CAPIGO_API_URL=", p.env()), "health")
	assertRefused(t, r, 1, "EMPTY_ENV_API_URL", "env")
	assertNoRequests(t, evil)
	assertNoProxyTraffic(t, p)
}

// L5
func TestManagedURLFlagWins(t *testing.T) {
	good, other, evil, p := newFakeAPI(t), newFakeAPI(t), newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), other, "CAPIGO_API_KEY=csk_x", "API_URL="+evil.srv.URL, p.env()),
		"--api-url", good.srv.URL, "health")
	assertExit(t, r, 0)
	assertSentKey(t, good, "csk_x")
	assertNoRequests(t, other)
	assertNoRequests(t, evil)
	assertNoProxyTraffic(t, p)
}

// T1
func TestManagedTenantIgnoresUnprefixedTENANT(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f, "CAPIGO_API_KEY=csk_x", "TENANT=evil", p.env()), "boards", "list")
	assertExit(t, r, 0)
	assertSentTenant(t, f, "")
	assertTenantNeverSent(t, f, r, "evil")
}

// T2
func TestManagedTenantPrefersCAPIGOTenant(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f,
		"CAPIGO_API_KEY=csk_x", "TENANT=evil", "CAPIGO_TENANT=acme", p.env()), "boards", "list")
	assertExit(t, r, 0)
	assertSentTenant(t, f, "acme")
	if got := r.metaField(t, "tenant_source"); got != "env" {
		t.Errorf("meta.tenant_source = %q, want %q", got, "env")
	}
	assertTenantNeverSent(t, f, r, "evil")
}

// T3
func TestManagedTenantIgnoresStoredDefault(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, otherTenantProfile)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x", p.env()), "boards", "list")
	assertExit(t, r, 0)
	assertSentKey(t, f, "csk_x")
	assertTenantNeverSent(t, f, r, "other")
	if strings.Contains(r.stdout, "tenant_source") {
		t.Errorf("meta names a tenant_source with no tenant resolved:\n%s", r.stdout)
	}
}

// T4: an empty CAPIGO_TENANT is no tenant; it does not fall back to the stored one.
func TestManagedEmptyEnvTenantIsNone(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, otherTenantProfile)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x", "CAPIGO_TENANT=", p.env()), "boards", "list")
	assertExit(t, r, 0)
	assertSentTenant(t, f, "")
	assertTenantNeverSent(t, f, r, "other")
}

// T5
func TestNoConfigTenantIgnoresUnprefixedTENANT(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f,
		"CAPIGO_NO_CONFIG=1", "CAPIGO_API_KEY=csk_x", "TENANT=evil", p.env()), "boards", "list")
	assertExit(t, r, 0)
	assertSentTenant(t, f, "")
	assertTenantNeverSent(t, f, r, "evil")
}

// T6
func TestManagedTenantFlagWins(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)

	r := runCapigo(t, baseEnv(t.TempDir(), f, "CAPIGO_API_KEY=csk_x", "CAPIGO_TENANT=acme", p.env()),
		"boards", "list", "--tenant", "demo")
	assertExit(t, r, 0)
	assertSentTenant(t, f, "demo")
	if got := r.metaField(t, "tenant_source"); got != "flag" {
		t.Errorf("meta.tenant_source = %q, want %q", got, "flag")
	}
}

// A tenant-scoped command with no managed tenant fails locally, and its repair
// does not point at the stored default it would not apply. Without a managed
// key the stored default resolves, and the old wording stands when none does.
func TestManagedMissingTenantHint(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, otherTenantProfile)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x", "CAPIGO_TENANT=", p.env()), "brands", "list")
	assertExit(t, r, 5)
	assertNoRequests(t, f)
	if strings.Contains(r.stdout, "other") {
		t.Errorf("stdout names the stored tenant:\n%s", r.stdout)
	}
	if msg := r.errorField(t, "message"); strings.Contains(msg, "set a default") {
		t.Errorf("managed message suggests a stored default: %s", msg)
	}

	keyOnly, _ := homeWithConfig(t, storedProfile)
	unmanaged := runCapigo(t, baseEnv(keyOnly, f), "brands", "list")
	assertExit(t, unmanaged, 5)
	want := "brands commands: a tenant is required; pass --tenant <code> or set a default"
	if msg := unmanaged.errorField(t, "message"); msg != want {
		t.Errorf("unmanaged message = %q, want %q", msg, want)
	}
}

// M1: the config file is not read at all, so a corrupt one cannot fail the call.
func TestManagedIgnoresCorruptConfig(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)
	home, path := homeWithConfig(t, "{not json")
	before := snapshot(t, path)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x", p.env()), "health")
	assertExit(t, r, 0)
	assertSentKey(t, f, "csk_x")
	assertUntouched(t, path, before)
}

// M2
func TestManagedIgnoresDanglingProfile(t *testing.T) {
	f, p := newFakeAPI(t), newRecordingProxy(t)
	home, _ := homeWithConfig(t, `{"version":1,"active_profile":"ghost","profiles":{"default":{"api_key":"csk_file","api_url":""}}}`)

	r := runCapigo(t, baseEnv(home, f, "CAPIGO_API_KEY=csk_x", p.env()), "boards", "list", "--tenant", "demo")
	assertExit(t, r, 0)
	assertSentKey(t, f, "csk_x")
	assertSentTenant(t, f, "demo")
}
