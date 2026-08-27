// Package cmd — openapi_path_coverage_test.go
//
// Guard test: every OpenAPI operation is either wrapped by a CLI command or
// listed as deliberately unwrapped, with a reason.
//
// The unit is the operation — METHOD + path — not the path alone. A path-only
// guard cannot see a verb appear on a path it already knows: prod added
// `GET /mission/tasks/{id}/subtasks` beside the POST the CLI already called,
// and the old test, which tracked "paths not method+path pairs" by design,
// stayed green while a whole capability went unwrapped.
//
// The test asserts:
//   - every operation in the spec is in exactly one of the two maps
//   - the two maps are disjoint
//   - neither map names an operation the spec does not declare (no stale entries)
//
// When the spec grows, this test fails until someone decides what the growth
// means. The failure is the decision, not noise to be silenced.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// implementedOps is every OpenAPI operation the CLI actually calls. Derived by
// reading cmd/*.go and internal/api/client.go for the literal methods and paths
// passed to client.Do.
var implementedOps = map[string]string{
	"GET /health": "health",

	"GET /tenants": "tenants list",

	"GET /members":      "members list",
	"GET /members/{id}": "members get",

	"GET /mission/boards":                       "boards list",
	"GET /mission/boards/{id}":                  "boards get",
	"POST /mission/boards":                      "boards create",
	"PATCH /mission/boards/{id}":                "boards update",
	"POST /mission/boards/{id}/lists":           "boards lists create",
	"PATCH /mission/boards/{id}/lists/{listId}": "boards lists update",

	"GET /mission/tasks":                                                   "tasks list",
	"POST /mission/tasks":                                                  "tasks create",
	"GET /mission/tasks/{id}":                                              "tasks get",
	"PATCH /mission/tasks/{id}":                                            "tasks update",
	"GET /mission/tasks/{id}/comments":                                     "tasks comments",
	"POST /mission/tasks/{id}/comments":                                    "tasks comments create",
	"GET /mission/tasks/{id}/subtasks":                                     "tasks subtasks list",
	"POST /mission/tasks/{id}/subtasks":                                    "tasks subtasks create (batch)",
	"POST /mission/tasks/with-subtasks":                                    "tasks create --subtasks-json (atomic parent + subtasks)",
	"GET /mission/tasks/{id}/attachments/{attachmentId}/download":          "tasks attachments download",
	"GET /mission/tasks/{id}/comments/attachments/{attachmentId}/download": "tasks comments attachments download",

	// The same five capabilities, addressed by a task's code instead of its id.
	// One flag on each existing command, not five more commands.
	"GET /mission/tasks/code/{code}":                                              "tasks get --code",
	"PATCH /mission/tasks/code/{code}":                                            "tasks update --code",
	"GET /mission/tasks/code/{code}/comments":                                     "tasks comments --code",
	"POST /mission/tasks/code/{code}/comments":                                    "tasks comments create --code",
	"GET /mission/tasks/code/{code}/subtasks":                                     "tasks subtasks list --code",
	"POST /mission/tasks/code/{code}/subtasks":                                    "tasks subtasks create --code",
	"GET /mission/tasks/code/{code}/attachments/{attachmentId}/download":          "tasks attachments download --code",
	"GET /mission/tasks/code/{code}/comments/attachments/{attachmentId}/download": "tasks comments attachments download --code",

	"GET /pcms/products":               "products list",
	"POST /pcms/products":              "products create",
	"GET /pcms/products/{id}":          "products get",
	"PUT /pcms/products/{id}":          "products update — note this PUT takes a partial body, unlike the ref-data PUTs",
	"PUT /pcms/products/{id}/variants": "products variants (upsert)",
	"GET /pcms/variants":               "variants list",
	"GET /pcms/variants/{id}":          "variants get <id>",
	"GET /pcms/variants/sku/{sku}":     "variants get --sku <sku> — the same record, addressed by its natural key",

	"GET /pcms/brands":        "brands list",
	"POST /pcms/brands":       "brands create",
	"GET /pcms/brands/{id}":   "brands get",
	"PATCH /pcms/brands/{id}": "brands update",
	"PUT /pcms/brands/{id}":   "brands replace",

	"GET /pcms/categories":        "categories list",
	"POST /pcms/categories":       "categories create",
	"GET /pcms/categories/{id}":   "categories get",
	"PATCH /pcms/categories/{id}": "categories update",
	"PUT /pcms/categories/{id}":   "categories replace",

	"GET /pcms/product-types":        "product-types list",
	"POST /pcms/product-types":       "product-types create",
	"GET /pcms/product-types/{id}":   "product-types get",
	"PATCH /pcms/product-types/{id}": "product-types update",
	"PUT /pcms/product-types/{id}":   "product-types replace",

	"GET /pcms/units":        "units list",
	"POST /pcms/units":       "units create",
	"GET /pcms/units/{id}":   "units get",
	"PATCH /pcms/units/{id}": "units update",
	"PUT /pcms/units/{id}":   "units replace",
}

// unimplementedOps is every operation the CLI deliberately does not wrap, each
// with a reason. Add here — rather than to implementedOps — when an operation is
// left without a command on purpose.
var unimplementedOps = map[string]string{

	// WMS: a module addressed by code rather than by id, and no longer read-only —
	// prod now publishes the full write path (create, update, preview, validate, and
	// state-machine actions) for every document type. Held out deliberately — the API
	// surface is not settled yet, and a CLI that wraps an unsettled surface teaches
	// its callers a shape that will move under them. Revisit when the module stabilises.
	"GET /wms/warehouses":                 "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/warehouses/{code}":          "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/inbound-receipts":           "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/inbound-receipts/{code}":    "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/outbound-shipments":         "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/outbound-shipments/{code}":  "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/internal-transfers":         "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/internal-transfers/{code}":  "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/locations":                  "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/locations/{code}":           "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/warehouse-transfers":        "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",
	"GET /wms/warehouse-transfers/{code}": "unwrapped: the WMS API is not yet stable; wrapping it would freeze a moving surface",

	// WMS writes. Beyond the stability argument above, these are not single calls but
	// a stateful workflow — preview and validate feed a create, and the documents then
	// move through /actions/{action}. Wrapping a workflow whose states are still
	// moving is how a CLI ends up with commands it cannot later remove. Left for a
	// dedicated design pass once the module settles, not folded into a spec sync.
	"POST /wms/inbound-receipts":                            "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/inbound-receipts/preview":                    "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/inbound-receipts/validate":                   "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"PATCH /wms/inbound-receipts/{code}":                    "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/inbound-receipts/{code}/actions/{action}":    "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/outbound-shipments":                          "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/outbound-shipments/preview":                  "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/outbound-shipments/validate":                 "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"PATCH /wms/outbound-shipments/{code}":                  "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/outbound-shipments/{code}/actions/{action}":  "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/internal-transfers":                          "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/internal-transfers/preview":                  "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/internal-transfers/validate":                 "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"PATCH /wms/internal-transfers/{code}":                  "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/warehouse-transfers":                         "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/warehouse-transfers/preview":                 "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/warehouse-transfers/validate":                "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"PATCH /wms/warehouse-transfers/{code}":                 "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
	"POST /wms/warehouse-transfers/{code}/actions/{action}": "unwrapped: WMS write path is an unsettled stateful workflow; needs its own design pass",
}

// openAPIPathOnlySpec is the minimal subset of OpenAPI 3.0 needed to enumerate
// operations: a path, and the methods hanging off it.
type openAPIPathOnlySpec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
}

func specOperations(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../api/openapi.json")
	if err != nil {
		t.Fatalf("read api/openapi.json: %v", err)
	}
	var spec openAPIPathOnlySpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse api/openapi.json: %v", err)
	}
	ops := map[string]bool{}
	for path, item := range spec.Paths {
		for method := range item {
			if httpMethods[method] {
				ops[fmt.Sprintf("%s %s", strings.ToUpper(method), path)] = true
			}
		}
	}
	if len(ops) == 0 {
		t.Fatal("openapi.json declares no operations; this guard would pass vacuously")
	}
	return ops
}

func TestOpenAPIPathCoverage(t *testing.T) {
	ops := specOperations(t)

	for op := range implementedOps {
		if _, dup := unimplementedOps[op]; dup {
			t.Errorf("%q is in both implementedOps and unimplementedOps; remove it from one", op)
		}
	}

	// No stale entries: an operation the API dropped must not linger in either
	// map, claiming a command wraps something that is gone.
	for _, m := range []struct {
		name string
		set  map[string]string
	}{{"implementedOps", implementedOps}, {"unimplementedOps", unimplementedOps}} {
		for op := range m.set {
			if !ops[op] {
				t.Errorf("%s lists %q, which openapi.json does not declare; remove the stale entry", m.name, op)
			}
		}
	}

	var uncovered []string
	for op := range ops {
		_, impl := implementedOps[op]
		_, unimpl := unimplementedOps[op]
		if !impl && !unimpl {
			uncovered = append(uncovered, op)
		}
	}
	sort.Strings(uncovered)
	for _, op := range uncovered {
		t.Errorf("openapi.json declares %q, which is in neither implementedOps nor unimplementedOps; "+
			"wrap it in a CLI command and list it in implementedOps, OR add it to unimplementedOps with a reason", op)
	}
}

// A reason that says nothing is a reason nobody revisits. Every entry in
// unimplementedOps carries one, and "TODO" is not one.
func TestUnimplementedOpsCarryAReason(t *testing.T) {
	for op, reason := range unimplementedOps {
		if len(reason) < 25 {
			t.Errorf("%q is listed as unimplemented without a real reason: %q", op, reason)
		}
	}
}
