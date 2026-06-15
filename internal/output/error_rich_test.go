package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func richDetail() ErrorDetail {
	return ErrorDetail{
		Code:           "E9426",
		Message:        "Failed to update product variants",
		RequestID:      "req_abc123",
		HTTPStatus:     400,
		Meaning:        "The server rejected creating a product variant.",
		Next:           "Check the new variant's fields and retry.",
		CapabilityNote: true,
		RawBody:        `{"error":{"code":"E9426","message":"Failed to update product variants"}}`,
	}
}

func TestRenderErrorRich_Table_StdoutCarriesDiagnosis(t *testing.T) {
	var stdout, stderr bytes.Buffer
	RenderErrorRich(&stdout, &stderr, "table", richDetail())

	out := stdout.String()
	for _, want := range []string{
		"E9426", "HTTP 400",
		"The server rejected creating a product variant.",
		capabilityBrake,
		"Check the new variant's fields and retry.",
		`{"error":{"code":"E9426"`,
		"req_abc123",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n--- stdout ---\n%s", want, out)
		}
	}
	// Concise line must still reach stderr for humans/scripts.
	if !strings.Contains(stderr.String(), "code=E9426") {
		t.Errorf("stderr missing concise line, got %q", stderr.String())
	}
}

func TestRenderErrorRich_Table_NoBrakeWhenNotCapability(t *testing.T) {
	d := richDetail()
	d.CapabilityNote = false
	var stdout, stderr bytes.Buffer
	RenderErrorRich(&stdout, &stderr, "table", d)
	if strings.Contains(stdout.String(), capabilityBrake) {
		t.Errorf("brake should not appear when CapabilityNote is false:\n%s", stdout.String())
	}
}

func TestRenderErrorRich_JSON_EnrichedOnStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	RenderErrorRich(&stdout, &stderr, "json", richDetail())

	var got struct {
		Error struct {
			Code           string `json:"code"`
			Meaning        string `json:"meaning"`
			Next           string `json:"next"`
			CapabilityNote bool   `json:"capability_note"`
			Raw            string `json:"raw"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if got.Error.Code != "E9426" || got.Error.Meaning == "" || !got.Error.CapabilityNote || got.Error.Raw == "" {
		t.Errorf("enriched JSON fields missing: %+v", got.Error)
	}
}

func TestRenderErrorRich_Quiet_StdoutEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	RenderErrorRich(&stdout, &stderr, "quiet", richDetail())
	if stdout.Len() != 0 {
		t.Errorf("quiet mode must not write to stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "code=E9426") {
		t.Errorf("quiet mode should still write the concise stderr line, got %q", stderr.String())
	}
}
