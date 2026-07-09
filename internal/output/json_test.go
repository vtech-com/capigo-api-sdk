package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type brand struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// A caller distinguishes "no rows" from "no such key". A nil slice marshals to
// null, which reads as neither.
func TestWriteEmitsAnEmptyArrayForNoRows(t *testing.T) {
	var buf bytes.Buffer
	var none []brand
	if err := Write(&buf, none, Meta{Total: Ptr(0)}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !strings.Contains(buf.String(), `"data": []`) {
		t.Errorf("nil slice must emit [], got:\n%s", buf.String())
	}
}

// The pagination fields are pointers precisely so that a zero survives. An
// omitempty int would drop "total": 0 — the answer to "how many are there?"
// on an empty tenant.
func TestWriteKeepsZeroTotal(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, []brand{}, Meta{Total: Ptr(0), Page: Ptr(1)}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var got struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := got.Meta["total"]; !ok {
		t.Errorf(`meta.total dropped when zero: %v`, got.Meta)
	}
}

// A single item is an object at .data, not a one-element array, and not the
// top level.
func TestWritePutsASingleItemAtData(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, brand{ID: "b1", Name: "Nike"}, Meta{Tenant: "acme", TenantSource: "flag"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	var got struct {
		Data brand `json:"data"`
		Meta Meta  `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Data.ID != "b1" || got.Meta.Tenant != "acme" || got.Meta.TenantSource != "flag" {
		t.Errorf("got %+v", got)
	}
	// A single-item read has nothing to page.
	if got.Meta.Total != nil || got.Meta.Page != nil {
		t.Errorf("single item carries pagination meta: %+v", got.Meta)
	}
}

// A tenantless command must not claim a tenant it never resolved.
func TestWriteOmitsTenantWhenThereIsNone(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, brand{ID: "b1"}, Meta{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), "tenant") {
		t.Errorf("empty meta must not mention a tenant:\n%s", buf.String())
	}
}

// stdout stays parseable when the command fails: a caller that reads stdout
// unconditionally gets a diagnosis, not a parse error on top of an API error.
func TestRenderErrorPutsJSONOnStdoutAndASummaryOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	RenderError(&stdout, &stderr, ErrorDetail{
		Code: "E9426", Message: "create product variant failed",
		RequestID: "req_7", HTTPStatus: 400, CapabilityNote: true,
	})

	var got struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got.Error["code"] != "E9426" || got.Error["request_id"] != "req_7" {
		t.Errorf("error object lost fields: %v", got.Error)
	}
	// The brake is the sentence, not a boolean. `"capability_note": true` tells
	// a reader nothing; the sentence is what stops the wrong conclusion.
	note, _ := got.Error["capability_note"].(string)
	if !strings.Contains(note, "does NOT mean this operation is unsupported") {
		t.Errorf("capability_note is not the brake sentence: %v", got.Error["capability_note"])
	}
	if !strings.Contains(stderr.String(), "Error: create product variant failed") {
		t.Errorf("stderr summary missing:\n%s", stderr.String())
	}
}

// Without the brake, the note must be absent rather than false: a caller
// scanning for the key should not find one that says nothing.
func TestRenderErrorOmitsTheBrakeWhenItDoesNotApply(t *testing.T) {
	var stdout, stderr bytes.Buffer
	RenderError(&stdout, &stderr, ErrorDetail{Code: "VALIDATION_ERROR", Message: "--name is required"})

	var got struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if got.Error["capability_note"] != nil {
		t.Errorf("client-side error carries the capability brake: %v", got.Error["capability_note"])
	}
}

// stdout carries one JSON document. A command that has already printed its
// envelope and then fails must not append an error object to it: json.load on
// the pair raises "Extra data", which is precisely the failure the single-shape
// contract exists to remove.
func TestRenderErrorSummaryWritesNothingToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	_ = Write(&stdout, []brand{{ID: "a1"}}, Meta{Total: Ptr(5)})
	before := stdout.Len()

	RenderErrorSummary(&stderr, ErrorDetail{Code: "BOOM", Message: "page 2 failed"})

	if stdout.Len() != before {
		t.Errorf("stdout grew after the envelope was written:\n%s", stdout.String())
	}
	var one any
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := dec.Decode(&one); err != nil {
		t.Fatalf("envelope does not decode: %v", err)
	}
	if dec.More() {
		t.Error("stdout holds more than one JSON document")
	}
	if !strings.Contains(stderr.String(), "page 2 failed") {
		t.Errorf("stderr lost the diagnosis: %s", stderr.String())
	}
}
