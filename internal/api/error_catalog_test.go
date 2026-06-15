package api

import "testing"

func TestLookupError_KnownWriteCodesCarryBrake(t *testing.T) {
	// Every write-side code must carry a next step and the capability brake.
	// Meaning is deliberately NOT required here — for codes with a descriptive
	// server message it is left empty so the server message is the single source.
	for _, code := range []string{"E9426", "E9445", "E9446", "VALIDATION_ERROR"} {
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
	for _, code := range []string{"E9425", "E0102", "E0004", "E9103"} {
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

func TestLookupError_Unknown(t *testing.T) {
	if _, ok := LookupError("E0000_NOT_REAL"); ok {
		t.Error("unknown code should not be found")
	}
}
