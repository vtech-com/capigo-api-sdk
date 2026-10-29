package api

import (
	"strings"
	"testing"
)

func TestLookupError_KnownWriteCodesCarryBrake(t *testing.T) {
	// Every write-side code must carry a next step and the capability brake.
	// Meaning is deliberately NOT required here — for codes with a descriptive
	// server message it is left empty so the server message is the single source.
	for _, code := range []string{
		"E9426", "E9445", "E9446", "VALIDATION_ERROR", "TASK_ALREADY_ASSIGNED",
		"E0602", "E0601", "E9649", "E9680", "E9675", "E9674",
		"SUBTASK_NOT_PROMOTABLE", "DESTINATION_UNAVAILABLE",
	} {
		info, ok := LookupError(code)
		if !ok {
			t.Errorf("%s should be in the catalog", code)
			continue
		}
		if info.Next == "" {
			t.Errorf("%s must have a next step: %+v", code, info)
		}
		if !info.CapabilityNote {
			t.Errorf("%s is a write-side error and must set CapabilityNote", code)
		}
	}
}

func TestLookupError_GenericCodesCarryMeaning(t *testing.T) {
	// E9426/E9427 fall to the backend's generic fallback message, so the catalog
	// must supply a Meaning to fill that void. Codes with a good server message
	// must NOT duplicate it.
	for _, code := range []string{"E9426", "E9427"} {
		info, _ := LookupError(code)
		if info.Meaning == "" {
			t.Errorf("%s has a generic server message and must carry a Meaning", code)
		}
	}
	for _, code := range []string{"E9445", "E9446", "E9443", "E9425"} {
		info, _ := LookupError(code)
		if info.Meaning != "" {
			t.Errorf("%s has a descriptive server message; Meaning should be empty to avoid drift, got %q", code, info.Meaning)
		}
	}
}

func TestLookupError_NonWriteCodesNoBrake(t *testing.T) {
	for _, code := range []string{"E9425", "E0102", "E0004", "E9103", "E9647", "WAREHOUSE_CODE_REQUIRED"} {
		info, ok := LookupError(code)
		if !ok {
			t.Errorf("%s should be in the catalog", code)
			continue
		}
		if info.CapabilityNote {
			t.Errorf("%s must not set CapabilityNote (would be a misleading brake)", code)
		}
	}
}

func TestLookupError_AuthCodesCarryNextWithoutBrake(t *testing.T) {
	// Public API auth codes must each carry a next step and must NOT set the
	// capability brake (auth is never about a missing feature).
	for _, code := range []string{
		"AUTH_INVALID_KEY", "AUTH_INVALID_KEY_PREFIX", "AUTH_MISSING_HEADER",
		"AUTH_INVALID_FORMAT", "AUTH_TENANT_MISMATCH", "AUTH_INTERNAL_ERROR",
	} {
		info, ok := LookupError(code)
		if !ok {
			t.Errorf("%s should be in the catalog", code)
			continue
		}
		if info.Next == "" {
			t.Errorf("%s must have a next step", code)
		}
		if info.CapabilityNote {
			t.Errorf("%s is an auth error and must not set CapabilityNote", code)
		}
		if info.Meaning != "" {
			t.Errorf("%s has a descriptive server message; Meaning should be empty, got %q", code, info.Meaning)
		}
	}
}

func TestLookupError_AuthRetryGuidanceDistinguishesTransient(t *testing.T) {
	// The whole point of cataloging these: a rejected key is deterministic
	// (retry is futile) while an auth-service 500 is transient (retry is right).
	// Guard the wording so the two never collapse into "just retry".
	invalid, _ := LookupError("AUTH_INVALID_KEY")
	if !strings.Contains(invalid.Next, "NOT help") {
		t.Errorf("AUTH_INVALID_KEY must warn that retry will not help, got %q", invalid.Next)
	}
	internal, _ := LookupError("AUTH_INTERNAL_ERROR")
	if !strings.Contains(internal.Next, "transient") {
		t.Errorf("AUTH_INTERNAL_ERROR must flag the failure as transient/retryable, got %q", internal.Next)
	}
}

// A wms update is answered 412 after it has been written. An agent told only
// "Precondition Failed" retries with the token it already spent and reads the
// E0602 that follows as proof nothing was saved.
func TestLookupError_PreconditionFailedSaysTheWriteMayHaveLanded(t *testing.T) {
	info, ok := LookupError("Precondition Failed")
	if !ok {
		t.Fatal("a 412 must be in the catalog")
	}
	if !strings.Contains(info.Meaning, "already written") {
		t.Errorf("Meaning must say the change may already be written, got %q", info.Meaning)
	}
	if !strings.Contains(info.Next, "Do not retry") || !strings.Contains(info.Next, "get") {
		t.Errorf("Next must stop the retry and send the caller to get, got %q", info.Next)
	}
}

func TestLookupError_Unknown(t *testing.T) {
	if _, ok := LookupError("E0000_NOT_REAL"); ok {
		t.Error("unknown code should not be found")
	}
}

// managedNextViolation names what a managed-mode next step must never say, or
// returns "". Under a managed key the caller cannot log in or swap the key, so
// a step that asks for one sends a person hunting for a secret that changes
// nothing — and names of the environment's internals do not belong to the CLI.
func managedNextViolation(next string) string {
	lower := strings.ToLower(next)
	for _, banned := range []string{
		"auth login", "csk_", "--key", "their key", "user's key",
		"capigo_api_key", "capigo_no_config", "unset",
		"openclaw", "tấm", "feishu", "lark", "plugin", "vps",
	} {
		if strings.Contains(lower, banned) {
			return banned
		}
	}
	// "do not ask the user for a key" is the instruction; any other asking is the
	// request the managed text exists to prevent.
	if strings.Contains(strings.ReplaceAll(lower, "do not ask the user", ""), "ask the user") {
		return "ask the user"
	}
	for _, word := range strings.FieldsFunc(lower, func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if word == "host" {
			return "host"
		}
	}
	return ""
}

func TestManagedNextNeverPointsAtLogin(t *testing.T) {
	for code := range errorCatalog {
		info, _ := LookupErrorFor(code, true)
		if v := managedNextViolation(info.Next); v != "" {
			t.Errorf("%s: managed next contains %q: %s", code, v, info.Next)
		}
	}

	// The codes a managed key meets must each carry their own managed step, and
	// the plain lookup must still return the step it always has.
	for _, code := range []string{
		"E0004", "AUTH_INVALID_KEY", "AUTH_INVALID_KEY_PREFIX", "AUTH_MISSING_HEADER",
		"AUTH_INVALID_FORMAT", "AUTH_TENANT_MISMATCH",
	} {
		info, ok := LookupError(code)
		if !ok || info.ManagedNext == "" {
			t.Errorf("%s has no ManagedNext", code)
			continue
		}
		if !strings.Contains(info.Next, "auth login") && code != "AUTH_TENANT_MISMATCH" {
			t.Errorf("%s: plain next lost its login step: %s", code, info.Next)
		}
		managed, _ := LookupErrorFor(code, true)
		if managed.Next != info.ManagedNext {
			t.Errorf("%s: LookupErrorFor(managed) next = %q, want ManagedNext", code, managed.Next)
		}
		if plain, _ := LookupErrorFor(code, false); plain != info {
			t.Errorf("%s: LookupErrorFor(unmanaged) = %+v, want %+v", code, plain, info)
		}
	}

	for _, code := range []string{"AUTH_EMPTY_ENV_KEY", "KEY_MANAGED_BY_ENV", "CONFIG_DISABLED", "EMPTY_ENV_API_URL"} {
		if info, ok := LookupError(code); !ok || info.Next == "" {
			t.Errorf("%s needs a catalog entry with a next step", code)
		}
	}

	// Pointing the CLI elsewhere is the redirect the managed URL rules exist to stop.
	if info, _ := LookupErrorFor("EMPTY_ENV_API_URL", true); strings.Contains(info.Next, "--api-url") {
		t.Errorf("EMPTY_ENV_API_URL next names --api-url: %s", info.Next)
	}
}

// The checker itself: the managed texts' own negation passes, a request fails.
func TestManagedNextViolationChecker(t *testing.T) {
	if v := managedNextViolation("Logging in here changes nothing — do not ask the user for a key."); v != "" {
		t.Errorf("negated instruction flagged as %q", v)
	}
	for _, bad := range []string{
		"Ask the user for their key.",
		"Run capigo auth login --key csk_x.",
		"Unset CAPIGO_API_KEY and retry.",
		"Ask the host operator.",
	} {
		if managedNextViolation(bad) == "" {
			t.Errorf("%q passed the check", bad)
		}
	}
}
