package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/spf13/viper"
)

type recordedCall struct {
	method  string
	path    string
	query   string
	tenant  string
	idemKey string
	body    map[string]any
	hasBody bool
	rawBody string
}

// callAgainst runs a command's RunE against a stub API that answers with reply,
// and returns what the API saw and what the command printed to stdout. The three
// commands here exit through os.Exit on a local validation failure, which a test
// cannot observe, so only the paths that reach the API run.
func callAgainst(t *testing.T, reply string, run func()) (recordedCall, string) {
	t.Helper()
	withConfig(t, "")

	var seen recordedCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.method = r.Method
		seen.path = r.URL.Path
		seen.query = r.URL.RawQuery
		seen.tenant = r.Header.Get("X-Tenant-Code")
		seen.idemKey = r.Header.Get("Idempotency-Key")
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			seen.hasBody = true
			seen.rawBody = string(raw)
			_ = json.Unmarshal(raw, &seen.body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("CAPIGO_API_KEY", "csk_test_key")
	t.Setenv("CAPIGO_API_URL", srv.URL)
	viper.Reset()
	_ = viper.BindEnv("api_key", "CAPIGO_API_KEY")
	_ = viper.BindEnv("api_url", "CAPIGO_API_URL")
	t.Cleanup(viper.Reset)

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	run()
	_ = w.Close()
	os.Stdout = origStdout
	printed, _ := io.ReadAll(r)

	return seen, string(printed)
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sameKeys(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestMembersInviteSendsOnlyTheIdentifiersGiven(t *testing.T) {
	memberInviteTenant, memberInviteEmail, memberInviteMobile = "acme", "tram@acme.vn", ""
	t.Cleanup(func() { memberInviteTenant, memberInviteEmail, memberInviteMobile = "", "", "" })

	seen, printed := callAgainst(t,
		`{"data":{"id":"inv-1","status":"pending","token":"tok-secret"}}`,
		func() { _ = membersInviteCmd.RunE(membersInviteCmd, nil) })

	if seen.method != "POST" || seen.path != "/members/invitations" {
		t.Errorf("call = %s %s, want POST /members/invitations", seen.method, seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
	if seen.idemKey != "" {
		t.Errorf("Idempotency-Key = %q, want none when the flag is not given", seen.idemKey)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"invitee_email"}) {
		t.Errorf("body keys = %v, want only invitee_email", got)
	}
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(printed), &out); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, printed)
	}
	if out.Data.Token != "tok-secret" {
		t.Errorf("token = %q; the creating answer's token must reach the caller", out.Data.Token)
	}
}

func TestMembersInviteSendsTheKeyAndBothIdentifiers(t *testing.T) {
	memberInviteTenant, memberInviteEmail, memberInviteMobile = "acme", "tram@acme.vn", "0912345678"
	memberInviteIdempotencyKey = "onboard-001"
	_ = membersInviteCmd.Flags().Set("idempotency-key", "onboard-001")
	t.Cleanup(func() {
		memberInviteTenant, memberInviteEmail, memberInviteMobile, memberInviteIdempotencyKey = "", "", "", ""
		membersInviteCmd.Flags().Lookup("idempotency-key").Changed = false
	})

	seen, _ := callAgainst(t, `{"data":{"id":"inv-1","status":"pending"}}`,
		func() { _ = membersInviteCmd.RunE(membersInviteCmd, nil) })

	if seen.idemKey != "onboard-001" {
		t.Errorf("Idempotency-Key = %q, want onboard-001", seen.idemKey)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"invitee_email", "invitee_mobile"}) {
		t.Errorf("body keys = %v, want invitee_email and invitee_mobile", got)
	}
}

func TestJoinRequestsApproveSendsTheNameAndOnlyTheOptionalsGiven(t *testing.T) {
	joinRequestApproveTenant, joinRequestApproveDisplayName = "acme", "Tram Nguyen"
	joinRequestApproveJobTitle = "Staff"
	t.Cleanup(func() {
		joinRequestApproveTenant, joinRequestApproveDisplayName, joinRequestApproveJobTitle = "", "", ""
	})

	seen, printed := callAgainst(t,
		`{"data":{"id":"req-1","status":"approved","member_id":"m-1"}}`,
		func() { _ = joinRequestsApproveCmd.RunE(joinRequestsApproveCmd, []string{"req-1"}) })

	if seen.method != "POST" || seen.path != "/join-requests/req-1/actions/approve" {
		t.Errorf("call = %s %s, want POST /join-requests/req-1/actions/approve", seen.method, seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"display_name", "job_title"}) {
		t.Errorf("body keys = %v, want display_name and job_title", got)
	}
	if !json.Valid([]byte(printed)) {
		t.Errorf("stdout is not JSON: %s", printed)
	}
}

func TestJoinRequestsRejectSendsNoBody(t *testing.T) {
	joinRequestRejectTenant = "acme"
	t.Cleanup(func() { joinRequestRejectTenant = "" })

	seen, _ := callAgainst(t, `{"data":{"id":"req-1","status":"rejected"}}`,
		func() { _ = joinRequestsRejectCmd.RunE(joinRequestsRejectCmd, []string{"req-1"}) })

	if seen.method != "POST" || seen.path != "/join-requests/req-1/actions/reject" {
		t.Errorf("call = %s %s, want POST /join-requests/req-1/actions/reject", seen.method, seen.path)
	}
	if seen.hasBody {
		t.Errorf("reject sent a body %v; the API refuses a field and there is none to send", seen.body)
	}
}

// callAgainstArray is callAgainst for a command whose body is a JSON array, which
// callAgainst cannot decode into a map: only rawBody is filled.
func callAgainstArray(t *testing.T, reply string, run func()) (recordedCall, string) {
	t.Helper()
	return callAgainst(t, reply, run)
}
