// Package cmd — openapi_body_coverage_test.go
//
// Guard test: OpenAPI requestBody fields ⟶ cobra flags (or --from-json escape hatch)
//
// This test prevents the class of bugs where a write command's request-body
// field exists in the OpenAPI spec but is never wired to a CLI flag
// (example: tasks create had follower_ids in the spec but no --follower-id flag).
//
// For each write command (POST/PATCH/PUT) the test:
//  1. Reads api/openapi.json and extracts the requestBody JSON schema properties.
//  2. Converts snake_case field names to kebab-case cobra flag names, applying a
//     documented alias map for non-obvious renames.
//  3. For EACH body property asserts ONE of:
//     (a) The command registers a cobra flag whose name maps to the field, OR
//     (b) The command registers --from-json (a generic body mechanism that covers
//     ALL fields; all PCMS resource commands use this escape hatch). When
//     --from-json is present the per-field assertion is skipped entirely.
//  4. Skips fields listed in intentionallyUnexposedBodyFields, which documents
//     server-managed/derived fields that legitimately have no flag.
//
// Alias map (bodyFieldToFlagName): snake_case → kebab-case, with extra entries
// for cases where the CLI flag name intentionally differs from the naive transform:
//
//	follower_ids → follower-id  (plural field, singular repeatable flag)
//	tenant_code  → tenant       (sent in request body but exposed as --tenant)
//	board_id     → board        (shorter flag name for usability)
//	board_list_id → list        (short flag name; context makes it clear)
//	assignee_id  → assignee     (trailing _id dropped for usability)
//
// Verifying the guard catches real regressions:
// To confirm this test would fail if --follower-id were removed from tasks create,
// temporarily comment out the StringArrayVar line in cmd/tasks.go init() and run
// `go test ./cmd/ -run TestOpenAPIBodyCoverage`; the test fails with:
//
//	tasks create (POST /mission/tasks): body field "follower_ids" expects cobra flag
//	--follower-id but it is not registered; add the flag or add "follower_ids" to
//	intentionallyUnexposedBodyFields for this command.
//
// Restoring the line makes the test pass. (This was verified during development.)
package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// bodyFieldToFlagName converts an OpenAPI requestBody field name to the
// expected cobra flag name. The base transform is snake_case → kebab-case;
// the alias map handles non-obvious cases that differ from that transform.
func bodyFieldToFlagName(field string) string {
	// Apply alias map first; if no alias, fall back to underscore→dash.
	if alias, ok := bodyFieldAliasMap[field]; ok {
		return alias
	}
	return strings.ReplaceAll(field, "_", "-")
}

// bodyFieldAliasMap maps OpenAPI body field names to the cobra flag name
// actually registered when the naming convention differs from the naive
// snake_case→kebab-case transform. Document the reason for each alias.
var bodyFieldAliasMap = map[string]string{
	// POST /mission/tasks
	// follower_ids is a plural field; the CLI exposes it as a singular repeatable
	// flag --follower-id so callers write: --follower-id <uuid> --follower-id <uuid>
	"follower_ids": "follower-id",
	// tenant_code is required by the API body but the CLI exposes it as --tenant
	// (consistent with all other tenant-scoped commands).
	"tenant_code": "tenant",
	// board_id and board_list_id are shortened for usability; the _id suffix is
	// redundant in a CLI context where the argument is clearly an identifier.
	"board_id":      "board",
	"board_list_id": "list",
	// assignee_id → assignee: the _id suffix dropped for usability.
	"assignee_id": "assignee",
	// POST /mission/tasks/with-subtasks: the subtasks array is supplied as a JSON
	// file via --subtasks-json on `tasks create`.
	"subtasks": "subtasks-json",
}

// intentionallyUnexposedBodyFields lists body fields that the CLI deliberately
// does not expose as flags, keyed by "<METHOD> <path>". Each entry maps field
// name → rationale. Fields in this list are skipped rather than failing the test.
// Any omission here must be a conscious, reviewed decision — not an accident.
// Currently empty: every per-field-checked command (only tasks create, since all
// others use the --from-json escape hatch) exposes a flag for every body field.
// Add an entry here only for a field that is genuinely server-managed/derived on a
// command that does NOT use --from-json.
var intentionallyUnexposedBodyFields = map[string]map[string]string{
	// POST /mission/tasks/with-subtasks is driven by `tasks create --subtasks-json`:
	// the nested `task` object is built from the existing per-field create flags
	// (--title, --description, --priority, …), so there is no single `--task` flag.
	"POST /mission/tasks/with-subtasks": {
		"task": "parent-task fields come from the individual tasks-create flags, not a single --task flag",
	},
}

// writeCommandMapping maps each write cobra command to its OpenAPI path + method.
// The path must match exactly what appears as a key in openapi.json "paths".
// The hasFlagsFn is a closure that looks up a flag by name on the specific command.
type writeCommandEntry struct {
	// humanName is "resource action" for error messages.
	humanName string
	// path is the OpenAPI path key (e.g. "/mission/tasks").
	path string
	// method is the HTTP method: "post", "patch", or "put".
	method string
	// hasFlag returns true if the cobra command registers a flag with the given name.
	hasFlag func(name string) bool
}

// buildWriteCommandMapping constructs the mapping at test time so that the
// cobra commands are fully initialised (init() has run).
func buildWriteCommandMapping() []writeCommandEntry {
	return []writeCommandEntry{
		// tasks create: no --from-json; must have per-field flags for everything.
		{
			humanName: "tasks create",
			path:      "/mission/tasks",
			method:    "post",
			hasFlag:   func(n string) bool { return tasksCreateCmd.Flags().Lookup(n) != nil },
		},
		// tasks subtasks: registers --from-json (batch array), so per-field assertion
		// is skipped; --tenant covers tenant_code.
		{
			humanName: "tasks subtasks",
			path:      "/mission/tasks/{id}/subtasks",
			method:    "post",
			hasFlag:   func(n string) bool { return tasksSubtasksCmd.Flags().Lookup(n) != nil },
		},
		// tasks create --subtasks-json drives with-subtasks: tenant_code→--tenant,
		// subtasks→--subtasks-json (alias), and the nested task object is built from
		// the individual create flags (task is intentionallyUnexposed).
		{
			humanName: "tasks create --subtasks-json",
			path:      "/mission/tasks/with-subtasks",
			method:    "post",
			hasFlag:   func(n string) bool { return tasksCreateCmd.Flags().Lookup(n) != nil },
		},
		// tasks update: no --from-json; must have per-field flags for every PATCH body field.
		// Pre-staged from develop; endpoint not yet on prod.
		{
			humanName: "tasks update",
			path:      "/mission/tasks/{id}",
			method:    "patch",
			hasFlag:   func(n string) bool { return tasksUpdateCmd.Flags().Lookup(n) != nil },
		},
		// PCMS resource commands: all register --from-json, so per-field assertion is
		// skipped. Listed here so NEW spec fields still surface as a test failure when
		// --from-json is absent (i.e. if a command forgets --from-json).
		{
			humanName: "brands create",
			path:      "/pcms/brands",
			method:    "post",
			hasFlag:   func(n string) bool { return brandsCreateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "brands update",
			path:      "/pcms/brands/{id}",
			method:    "patch",
			hasFlag:   func(n string) bool { return brandsUpdateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "brands replace",
			path:      "/pcms/brands/{id}",
			method:    "put",
			hasFlag:   func(n string) bool { return brandsReplaceCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "categories create",
			path:      "/pcms/categories",
			method:    "post",
			hasFlag:   func(n string) bool { return categoriesCreateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "categories update",
			path:      "/pcms/categories/{id}",
			method:    "patch",
			hasFlag:   func(n string) bool { return categoriesUpdateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "categories replace",
			path:      "/pcms/categories/{id}",
			method:    "put",
			hasFlag:   func(n string) bool { return categoriesReplaceCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "product-types create",
			path:      "/pcms/product-types",
			method:    "post",
			hasFlag:   func(n string) bool { return productTypesCreateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "product-types update",
			path:      "/pcms/product-types/{id}",
			method:    "patch",
			hasFlag:   func(n string) bool { return productTypesUpdateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "product-types replace",
			path:      "/pcms/product-types/{id}",
			method:    "put",
			hasFlag:   func(n string) bool { return productTypesReplaceCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "units create",
			path:      "/pcms/units",
			method:    "post",
			hasFlag:   func(n string) bool { return unitsCreateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "units update",
			path:      "/pcms/units/{id}",
			method:    "patch",
			hasFlag:   func(n string) bool { return unitsUpdateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "units replace",
			path:      "/pcms/units/{id}",
			method:    "put",
			hasFlag:   func(n string) bool { return unitsReplaceCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "products create",
			path:      "/pcms/products",
			method:    "post",
			hasFlag:   func(n string) bool { return productsCreateCmd.Flags().Lookup(n) != nil },
		},
		{
			humanName: "products update",
			path:      "/pcms/products/{id}",
			method:    "put",
			hasFlag:   func(n string) bool { return productsUpdateCmd.Flags().Lookup(n) != nil },
		},
		// products variants: --from-json only (the body is a JSON array of mixed
		// create/update variant objects, not flag-representable). Listed for
		// completeness so the guard covers every write command; the --from-json
		// escape hatch short-circuits per-field assertion.
		{
			humanName: "products variants",
			path:      "/pcms/products/{id}/variants",
			method:    "put",
			hasFlag:   func(n string) bool { return productsVariantsCmd.Flags().Lookup(n) != nil },
		},
	}
}

// openAPIBodySpec is a minimal subset of the OpenAPI 3.0 schema for body coverage.
type openAPIBodySpec struct {
	Paths      map[string]openAPIBodyPathItem `json:"paths"`
	Components struct {
		Schemas map[string]openAPIBodySchema `json:"schemas"`
	} `json:"components"`
}

type openAPIBodyPathItem struct {
	Post  *openAPIBodyOperation `json:"post"`
	Patch *openAPIBodyOperation `json:"patch"`
	Put   *openAPIBodyOperation `json:"put"`
}

type openAPIBodyOperation struct {
	RequestBody *openAPIBodyRequestBody `json:"requestBody"`
}

type openAPIBodyRequestBody struct {
	Content map[string]openAPIBodyContent `json:"content"`
}

type openAPIBodyContent struct {
	Schema openAPIBodySchema `json:"schema"`
}

type openAPIBodySchema struct {
	Ref        string                       `json:"$ref"`
	Properties map[string]openAPIBodySchema `json:"properties"`
}

func TestOpenAPIBodyCoverage(t *testing.T) {
	data, err := os.ReadFile("../api/openapi.json")
	if err != nil {
		t.Fatalf("read api/openapi.json: %v", err)
	}

	var spec openAPIBodySpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("parse api/openapi.json: %v", err)
	}

	// resolveSchema dereferences a $ref to its schema from spec.components.schemas.
	resolveSchema := func(schema openAPIBodySchema) openAPIBodySchema {
		if schema.Ref == "" {
			return schema
		}
		// Ref format: "#/components/schemas/SomeName"
		const prefix = "#/components/schemas/"
		if strings.HasPrefix(schema.Ref, prefix) {
			name := strings.TrimPrefix(schema.Ref, prefix)
			if s, ok := spec.Components.Schemas[name]; ok {
				return s
			}
		}
		return schema
	}

	entries := buildWriteCommandMapping()

	for _, entry := range entries {
		entry := entry // capture
		t.Run(entry.humanName, func(t *testing.T) {
			pathItem, ok := spec.Paths[entry.path]
			if !ok {
				t.Fatalf("path %q not found in openapi.json", entry.path)
			}

			var op *openAPIBodyOperation
			switch entry.method {
			case "post":
				op = pathItem.Post
			case "patch":
				op = pathItem.Patch
			case "put":
				op = pathItem.Put
			}
			if op == nil {
				t.Fatalf("path %q has no %s operation in openapi.json", entry.path, strings.ToUpper(entry.method))
				return
			}
			if op.RequestBody == nil {
				// No request body for this operation; nothing to check.
				t.Logf("path %q %s has no requestBody; skipping", entry.path, strings.ToUpper(entry.method))
				return
			}

			content, ok := op.RequestBody.Content["application/json"]
			if !ok {
				t.Fatalf("path %q %s requestBody has no application/json content", entry.path, strings.ToUpper(entry.method))
			}

			schema := resolveSchema(content.Schema)

			// Escape hatch: if the command registers --from-json, it covers all fields
			// generically. Skip per-field assertion for this command.
			if entry.hasFlag("from-json") {
				t.Logf("%s registers --from-json: per-field assertion skipped (generic body mechanism covers all fields)", entry.humanName)
				return
			}

			// Per-field assertion: every body property must have a cobra flag.
			opKey := strings.ToUpper(entry.method) + " " + entry.path
			intentional := intentionallyUnexposedBodyFields[opKey]

			for field := range schema.Properties {
				if reason, skip := intentional[field]; skip {
					t.Logf("skipping %q on %s: %s", field, opKey, reason)
					continue
				}

				flagName := bodyFieldToFlagName(field)

				if !entry.hasFlag(flagName) {
					t.Errorf(
						"%s (%s %s): body field %q expects cobra flag --%s but it is not registered; "+
							"add the flag or add %q to intentionallyUnexposedBodyFields for this command",
						entry.humanName, strings.ToUpper(entry.method), entry.path,
						field, flagName, field,
					)
				}
			}
		})
	}
}
