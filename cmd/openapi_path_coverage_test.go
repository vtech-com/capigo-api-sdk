// Package cmd — openapi_path_coverage_test.go
//
// Guard test (SOFT): OpenAPI paths ⟶ implemented or intentionally skipped
//
// This test does NOT enforce a 1:1 mapping between spec paths and CLI commands —
// the CLI is a curated subset. Instead it asserts that every path in openapi.json
// is either:
//
//	(a) in implementedPaths — a path the CLI actually calls, or
//	(b) in unimplementedPaths — a path deliberately not wrapped (with a rationale).
//
// The test fails ONLY when a NEW spec path appears that is in neither set, turning
// `make update-spec` (which pulls new endpoints) into a visible, must-acknowledge
// signal. A developer who adds a new spec path must consciously decide whether to
// implement it or add it to the unimplementedPaths allowlist.
//
// Additional integrity checks:
//   - implementedPaths ∩ unimplementedPaths = ∅ (disjoint)
//   - Every path in implementedPaths actually exists in the spec (no stale entries)
//   - Every path in unimplementedPaths actually exists in the spec (no stale entries)
//
// Paths are OpenAPI path strings (e.g. "/mission/tasks", "/pcms/brands/{id}").
// A single path may have multiple methods (GET, POST, PATCH, PUT); we track paths
// not method+path pairs because the CLI maps a resource (e.g. "brands") to a group
// of methods — if we have brands create/update/replace we have all write methods.
package cmd

import (
	"encoding/json"
	"os"
	"testing"
)

// implementedPaths is every OpenAPI path the CLI actually calls.
// Derived by reading cmd/*.go and internal/api/client.go for literal paths passed to
// client.Do / built in client methods. Update this set when adding new commands.
var implementedPaths = map[string]string{
	// Auth / identity
	"/tenants": "tenants list — GET /tenants",

	// Members
	"/members":      "members list — GET /members",
	"/members/{id}": "members get — GET /members/{id} (pre-staged from develop; not yet on prod)",

	// Mission
	"/mission/boards":                                         "boards list — GET /mission/boards",
	"/mission/boards/{id}":                                    "boards get — GET /mission/boards/{id}",
	"/mission/tasks":                                          "tasks list + tasks create — GET + POST /mission/tasks",
	"/mission/tasks/{id}":                                     "tasks get + tasks update — GET + PATCH /mission/tasks/{id} (PATCH pre-staged from develop; not yet on prod)",
	"/mission/tasks/{id}/comments":                            "tasks comments — GET /mission/tasks/{id}/comments",
	"/mission/tasks/{id}/subtasks":                            "tasks subtasks — POST /mission/tasks/{id}/subtasks (batch create subtasks)",
	"/mission/tasks/with-subtasks":                            "tasks create --subtasks-json — POST /mission/tasks/with-subtasks (atomic parent+subtasks)",
	"/mission/tasks/{id}/attachments/{attachmentId}/download": "tasks attachments download — GET /mission/tasks/{id}/attachments/{attachmentId}/download",
	"/mission/tasks/{id}/comments/attachments/{attachmentId}/download": "tasks comments attachments download — GET /mission/tasks/{id}/comments/attachments/{attachmentId}/download",

	// PCMS products
	"/pcms/products":               "products list + products create — GET + POST /pcms/products",
	"/pcms/products/{id}":          "products get + products update — GET (pre-staged from develop; not yet on prod) + PUT /pcms/products/{id}",
	"/pcms/products/{id}/variants": "products variants — PUT /pcms/products/{id}/variants",
	"/pcms/variants":               "variants list — GET /pcms/variants",
	"/pcms/variants/{id}":          "variants get — GET /pcms/variants/{id} (pre-staged from develop; not yet on prod)",

	// PCMS ref data — brands
	"/pcms/brands":      "brands list + brands create — GET + POST /pcms/brands",
	"/pcms/brands/{id}": "brands get + brands update + brands replace — GET + PATCH + PUT /pcms/brands/{id}",

	// PCMS ref data — categories
	"/pcms/categories":      "categories list + categories create — GET + POST /pcms/categories",
	"/pcms/categories/{id}": "categories get + categories update + categories replace — GET + PATCH + PUT /pcms/categories/{id}",

	// PCMS ref data — product-types
	"/pcms/product-types":      "product-types list + product-types create — GET + POST /pcms/product-types",
	"/pcms/product-types/{id}": "product-types get + product-types update + product-types replace — GET + PATCH + PUT /pcms/product-types/{id}",

	// PCMS ref data — units
	"/pcms/units":      "units list + units create — GET + POST /pcms/units",
	"/pcms/units/{id}": "units get + units update + units replace — GET + PATCH + PUT /pcms/units/{id}",
}

// unimplementedPaths is every OpenAPI path the CLI deliberately does not wrap.
// Document a rationale for each entry. Update this set — rather than implementedPaths —
// when a path is intentionally left as a no-CLI-command.
// Currently empty: every spec path is wrapped by a CLI command. Add an entry here
// (rather than implementedPaths) when a path is intentionally left without a command.
var unimplementedPaths = map[string]string{}

// openAPIPathOnlySpec is the minimal subset of OpenAPI 3.0 needed for path enumeration.
type openAPIPathOnlySpec struct {
	Paths map[string]json.RawMessage `json:"paths"`
}

func TestOpenAPIPathCoverage(t *testing.T) {
	data, err := os.ReadFile("../api/openapi.json")
	if err != nil {
		t.Fatalf("read api/openapi.json: %v", err)
	}

	var spec openAPIPathOnlySpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse api/openapi.json: %v", err)
	}

	// Integrity check 1: implementedPaths and unimplementedPaths must be disjoint.
	for path := range implementedPaths {
		if _, inUnimpl := unimplementedPaths[path]; inUnimpl {
			t.Errorf("path %q appears in BOTH implementedPaths and unimplementedPaths; remove it from one", path)
		}
	}

	// Integrity check 2: every path in implementedPaths must exist in the spec.
	for path := range implementedPaths {
		if _, inSpec := spec.Paths[path]; !inSpec {
			t.Errorf("implementedPaths contains %q but that path does not exist in openapi.json; remove the stale entry", path)
		}
	}

	// Integrity check 3: every path in unimplementedPaths must exist in the spec.
	for path, rationale := range unimplementedPaths {
		if _, inSpec := spec.Paths[path]; !inSpec {
			t.Errorf("unimplementedPaths contains %q (rationale: %q) but that path does not exist in openapi.json; remove the stale entry", path, rationale)
		}
	}

	// Primary assertion: every spec path must be in implementedPaths OR unimplementedPaths.
	for path := range spec.Paths {
		_, implemented := implementedPaths[path]
		_, unimplemented := unimplementedPaths[path]
		if !implemented && !unimplemented {
			t.Errorf(
				"openapi.json contains path %q which is in neither implementedPaths nor unimplementedPaths; "+
					"add a CLI command and list it in implementedPaths, OR add it to unimplementedPaths with a rationale",
				path,
			)
		}
	}
}
