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

// A partial result is one JSON document carrying both the failure and the rows
// that survived it. Printed under a clean envelope those rows are
// indistinguishable from a complete answer; thrown away, the caller must refetch
// what the CLI already holds.
func TestWritePartialCarriesBothTheErrorAndTheRows(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := WritePartial(&stdout, &stderr, []brand{{ID: "a1"}}, Meta{Total: Ptr(1)},
		ErrorDetail{Code: "NOT_FOUND", Message: "2 of the requested ids were not returned: b2, c3"})
	if err != nil {
		t.Fatalf("WritePartial: %v", err)
	}

	// One document, not two: json.load on a pair raises "Extra data", which is
	// the failure the single-shape contract exists to remove.
	dec := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var got struct {
		Error map[string]any `json:"error"`
		Data  []brand        `json:"data"`
		Meta  Meta           `json:"meta"`
	}
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if dec.More() {
		t.Fatal("stdout holds more than one JSON document")
	}

	if got.Error["code"] != "NOT_FOUND" {
		t.Errorf("error key lost: %v", got.Error)
	}
	if len(got.Data) != 1 || got.Data[0].ID != "a1" {
		t.Errorf("the rows that were fetched were discarded: %v", got.Data)
	}
	if got.Meta.Total == nil || *got.Meta.Total != 1 {
		t.Errorf("meta lost: %+v", got.Meta)
	}
	if !strings.Contains(stderr.String(), "b2, c3") {
		t.Errorf("stderr lost the diagnosis: %s", stderr.String())
	}
}

// The error key is the whole test for completeness. A caller must not have to
// consult the exit code, or read stderr, to know whether it holds everything.
func TestWriteOmitsTheErrorKeyOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, []brand{{ID: "a1"}}, Meta{Total: Ptr(1)}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if strings.Contains(buf.String(), "error") {
		t.Errorf("a successful envelope carries an error key:\n%s", buf.String())
	}
}

// The error reads the same whether or not rows came back with it — both paths
// build it from one function.
func TestPartialAndBareErrorsAgreeOnTheErrorObject(t *testing.T) {
	d := ErrorDetail{Code: "E9426", Message: "boom", RequestID: "req_7", CapabilityNote: true}

	var bareOut, bareErr bytes.Buffer
	RenderError(&bareOut, &bareErr, d)
	var partOut, partErr bytes.Buffer
	_ = WritePartial(&partOut, &partErr, []brand{}, Meta{}, d)

	var bare, part struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(bareOut.Bytes(), &bare); err != nil {
		t.Fatalf("bare: %v", err)
	}
	if err := json.Unmarshal(partOut.Bytes(), &part); err != nil {
		t.Fatalf("partial: %v", err)
	}
	if len(bare.Error) != len(part.Error) || bare.Error["code"] != part.Error["code"] ||
		bare.Error["capability_note"] != part.Error["capability_note"] {
		t.Errorf("the two error shapes disagree:\n bare: %v\n part: %v", bare.Error, part.Error)
	}
}
