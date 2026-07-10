package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The CLI is a transport, not a second definition of the platform's schemas.
//
// It once decoded every response into a Go struct and marshalled it again. That
// silently dropped every field the struct did not declare and invented every
// field it declared that the API did not send: `tasks get` discarded `parent`
// and emitted a `parent_task_id` no response ever carried, and `variants list`
// truncated nineteen fields to five, so `manufacturer_code` was readable through
// `variants get` and absent through `variants list`.
//
// Neither was announced. A caller cannot see a field that was never printed.
//
// So no command may decode a response into a typed model. Reading a value the
// command's own logic needs — an id, a tenant code — is done through a narrow
// local struct in cmd/meta.go, and never decides what the caller sees.
func TestNoCommandDecodesAResponseIntoATypedModel(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	// Request bodies are ours to model: we build them.
	banned := []string{
		"api.Envelope[",
		"Data api.",
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, phrase := range banned {
			if strings.Contains(string(src), phrase) {
				t.Errorf("%s decodes a response into a typed model (%q); use api.RawEnvelope and print the bytes", f, phrase)
			}
		}
	}
}

// A list that came back empty must print [], not null: a caller must not have to
// tell "no rows" apart from "no such key".
func TestRawListOfNothingIsAnEmptyArray(t *testing.T) {
	if got := string(rawList(nil)); got != "[]" {
		t.Errorf("rawList(nil) = %s, want []", got)
	}
	if got := string(rawItem(nil)); got != "null" {
		t.Errorf("rawItem(nil) = %s, want null", got)
	}
	if got := string(rawList([]byte(`[{"id":"a"}]`))); got != `[{"id":"a"}]` {
		t.Errorf("rawList must not touch a real array, got %s", got)
	}
}
