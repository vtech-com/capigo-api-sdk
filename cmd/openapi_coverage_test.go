// Package cmd — openapi_coverage_test.go
//
// Guard test: OpenAPI query params ⟶ cobra flags
//
// This test prevents the class of bugs where an OpenAPI endpoint supports a
// query parameter but the corresponding CLI command never exposes a flag for
// it (example: GET /mission/tasks has "q" but tasksListCmd had no --query).
//
// For each list command, we:
//  1. Parse api/openapi.json and extract every in:query parameter for the
//     path's GET operation.
//  2. Normalise the param name → cobra flag name (underscores→dashes).
//  3. Apply a small alias map for deliberate rename conventions (e.g. q→query).
//  4. Assert the cobra command registers a flag for every param, OR the param
//     is listed in intentionallyUnexposed with a documented rationale.
//
// If a param is added to the OpenAPI spec in the future, this test will
// immediately catch any command that does not expose it.
package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// paramToFlagName converts an OpenAPI query parameter name to the expected
// cobra flag name: underscores become dashes.
func paramToFlagName(param string) string {
	return strings.ReplaceAll(param, "_", "-")
}

// aliasMap maps OpenAPI param names to the cobra flag name actually registered
// when the naming convention differs from the underscore→dash transform.
// Document the reason for each alias.
var aliasMap = map[string]string{
	// GET /mission/tasks and GET /pcms/* use "q" as the search param;
	// all list commands expose it as --query/-q for discoverability.
	"q": "query",
	// GET /mission/tasks uses "filters" (deepObject) for status filter;
	// the CLI exposes it as --status (a flat, user-friendly flag).
	"filters": "status",
}

// intentionallyUnexposed lists params that the CLI deliberately does not expose
// as flags, with a documented rationale. Params in this list are skipped by
// the test rather than failing it. Any omission here must be a conscious,
// reviewed decision — not an accident.
var intentionallyUnexposed = map[string]map[string]string{
	// No intentionally unexposed params at this time.
	// Add entries here if a param is deliberately not surfaced:
	//   "/some/path": {"param_name": "reason why it is not exposed"},
}

// commandPathMapping maps each list command variable to its OpenAPI path.
// The path must match exactly what appears as a key in openapi.json "paths".
var commandPathMapping = []struct {
	name string
	cmd  interface{ HasFlags() bool } // cobra.Command satisfies this
	path string
}{
	{name: "boardsListCmd", cmd: boardsListCmd, path: "/mission/boards"},
	{name: "tasksListCmd", cmd: tasksListCmd, path: "/mission/tasks"},
	{name: "productsListCmd", cmd: productsListCmd, path: "/pcms/products"},
	{name: "brandsListCmd", cmd: brandsListCmd, path: "/pcms/brands"},
	{name: "categoriesListCmd", cmd: categoriesListCmd, path: "/pcms/categories"},
	{name: "productTypesListCmd", cmd: productTypesListCmd, path: "/pcms/product-types"},
	{name: "unitsListCmd", cmd: unitsListCmd, path: "/pcms/units"},
	{name: "variantsListCmd", cmd: variantsListCmd, path: "/pcms/variants"},
}

// openAPISpec is a minimal subset of the OpenAPI 3.0 schema needed for this test.
type openAPISpec struct {
	Paths map[string]openAPIPathItem `json:"paths"`
}

type openAPIPathItem struct {
	Get *openAPIOperation `json:"get"`
}

type openAPIOperation struct {
	Parameters []openAPIParameter `json:"parameters"`
}

type openAPIParameter struct {
	Name string `json:"name"`
	In   string `json:"in"`
}

func TestOpenAPICoverage(t *testing.T) {
	// Load openapi.json relative to this test file (cmd/ → ../api/openapi.json).
	data, err := os.ReadFile("../api/openapi.json")
	if err != nil {
		t.Fatalf("read api/openapi.json: %v", err)
	}

	var spec openAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse api/openapi.json: %v", err)
	}

	for _, entry := range commandPathMapping {
		entry := entry // capture
		t.Run(entry.name, func(t *testing.T) {
			pathItem, ok := spec.Paths[entry.path]
			if !ok {
				t.Fatalf("path %q not found in openapi.json", entry.path)
			}
			if pathItem.Get == nil {
				t.Fatalf("path %q has no GET operation in openapi.json", entry.path)
			}

			unexposedForPath := intentionallyUnexposed[entry.path]

			// We need to access cobra flags; use the concrete *cobra.Command type.
			// The interface trick above just helps ensure the variable is a command;
			// cast back to *cobra.Command via the actual vars.
			var flagLookup func(name string) bool
			switch entry.name {
			case "boardsListCmd":
				flagLookup = func(n string) bool { return boardsListCmd.Flags().Lookup(n) != nil }
			case "tasksListCmd":
				flagLookup = func(n string) bool { return tasksListCmd.Flags().Lookup(n) != nil }
			case "productsListCmd":
				flagLookup = func(n string) bool { return productsListCmd.Flags().Lookup(n) != nil }
			case "brandsListCmd":
				flagLookup = func(n string) bool { return brandsListCmd.Flags().Lookup(n) != nil }
			case "categoriesListCmd":
				flagLookup = func(n string) bool { return categoriesListCmd.Flags().Lookup(n) != nil }
			case "productTypesListCmd":
				flagLookup = func(n string) bool { return productTypesListCmd.Flags().Lookup(n) != nil }
			case "unitsListCmd":
				flagLookup = func(n string) bool { return unitsListCmd.Flags().Lookup(n) != nil }
			case "variantsListCmd":
				flagLookup = func(n string) bool { return variantsListCmd.Flags().Lookup(n) != nil }
			default:
				t.Fatalf("unknown command name %q in test mapping", entry.name)
			}

			for _, param := range pathItem.Get.Parameters {
				if param.In != "query" {
					continue // only check query params
				}

				paramName := param.Name

				// Check intentionally unexposed allowlist first.
				if reason, skip := unexposedForPath[paramName]; skip {
					t.Logf("skipping %q on %s: %s", paramName, entry.path, reason)
					continue
				}

				// Determine the expected flag name.
				flagName := paramToFlagName(paramName)
				if alias, ok := aliasMap[paramName]; ok {
					flagName = alias
				}

				if !flagLookup(flagName) {
					t.Errorf(
						"%s (%s GET): OpenAPI param %q expects cobra flag --%s but it is not registered; "+
							"add the flag or add %q to intentionallyUnexposed",
						entry.name, entry.path, paramName, flagName, paramName,
					)
				}
			}
		})
	}
}
