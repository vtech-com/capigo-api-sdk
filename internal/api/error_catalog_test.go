package api

import "testing"

func TestLookupError_KnownWriteCodesCarryBrake(t *testing.T) {
	for _, code := range []string{"E9426", "E9445", "E9446", "VALIDATION_ERROR"} {
		info, ok := LookupError(code)
		if !ok {
			t.Errorf("%s should be in the catalog", code)
			continue
		}
		if info.Meaning == "" || info.Next == "" {
			t.Errorf("%s must have a meaning and a next step: %+v", code, info)
		}
		if !info.CapabilityNote {
			t.Errorf("%s is a write-side error and must set CapabilityNote", code)
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
