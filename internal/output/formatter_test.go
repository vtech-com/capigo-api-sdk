package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

var fakeTasks = []Task{
	{Code: "TASK-1", ID: "task_001", Title: "Fix login bug", Status: "open", Assignee: "Alice", TenantCode: "acme"},
	{Code: "TASK-2", ID: "task_002", Title: "Update menu", Status: "done", Assignee: "Bob", TenantCode: "globex"},
}

func TestRender_TableMode_SingleTenant(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{GlobalMode: false, ResourceKind: "task"}
	if err := Render(&buf, "table", fakeTasks, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Code", "ID", "Title", "Status", "Assignee", "TASK-1", "task_001", "Fix login bug", "task_002", "Update menu"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Tenant") {
		t.Errorf("Tenant column should be absent in single-tenant mode\ngot:\n%s", out)
	}
}

func TestRender_TableMode_GlobalMode_TenantColumn(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{GlobalMode: true, ResourceKind: "task"}
	if err := Render(&buf, "table", fakeTasks, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Tenant") {
		t.Errorf("expected Tenant column in global mode\ngot:\n%s", out)
	}
	// Tenant column must be first — it appears before the ID column in the line
	tenantPos := strings.Index(out, "Tenant")
	idPos := strings.Index(out, "ID")
	if tenantPos == -1 || idPos == -1 || tenantPos > idPos {
		t.Errorf("Tenant column must appear before ID column in global mode\ngot:\n%s", out)
	}
	if !strings.Contains(out, "acme") || !strings.Contains(out, "globex") {
		t.Errorf("expected tenant values in table\ngot:\n%s", out)
	}
}

func TestRender_JSONMode(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{GlobalMode: false, ResourceKind: "task"}
	if err := Render(&buf, "json", fakeTasks, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot:\n%s", err, buf.String())
	}
	if len(decoded) != 2 {
		t.Errorf("expected 2 items, got %d", len(decoded))
	}
}

func TestRender_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{GlobalMode: false, ResourceKind: "task"}
	if err := Render(&buf, "quiet", fakeTasks, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d\ngot:\n%s", len(lines), buf.String())
	}
	if lines[0] != "task_001" || lines[1] != "task_002" {
		t.Errorf("unexpected IDs: %v", lines)
	}
}

func TestRender_EmptySlice_Table(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{GlobalMode: false, ResourceKind: "task"}
	if err := Render(&buf, "table", []Task{}, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	// Headers must still appear even with no rows.
	for _, want := range []string{"ID", "Title", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("empty table should still show header %q\ngot:\n%s", want, out)
		}
	}
}

func TestRender_EmptySlice_JSON(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "task"}
	if err := Render(&buf, "json", []Task{}, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("expected '[]' for empty JSON slice, got: %q", buf.String())
	}
}

func TestRender_EmptySlice_Quiet(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "task"}
	if err := Render(&buf, "quiet", []Task{}, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("quiet mode with empty slice should print nothing, got: %q", buf.String())
	}
}

func TestRender_SingleStruct(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "task"}
	single := Task{ID: "task_single", Title: "Solo", Status: "open", Assignee: "Eve"}
	if err := Render(&buf, "table", single, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "task_single") {
		t.Errorf("expected single struct row in table\ngot:\n%s", buf.String())
	}
}

func TestRenderError_JSON(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, "json", "NOT_FOUND", "Task not found", "req_abc123")

	var wrapper map[string]map[string]string
	if err := json.Unmarshal(buf.Bytes(), &wrapper); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot:\n%s", err, buf.String())
	}
	errObj, ok := wrapper["error"]
	if !ok {
		t.Fatalf("expected 'error' key in JSON output\ngot:\n%s", buf.String())
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected code=NOT_FOUND, got %q", errObj["code"])
	}
	if errObj["message"] != "Task not found" {
		t.Errorf("expected message='Task not found', got %q", errObj["message"])
	}
	if errObj["request_id"] != "req_abc123" {
		t.Errorf("expected request_id=req_abc123, got %q", errObj["request_id"])
	}
}

func TestRenderError_Table(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, "table", "NOT_FOUND", "Task not found", "req_abc123")
	out := buf.String()
	if !strings.Contains(out, "Error:") {
		t.Errorf("expected 'Error:' prefix\ngot: %q", out)
	}
	if !strings.Contains(out, "NOT_FOUND") {
		t.Errorf("expected code in output\ngot: %q", out)
	}
	if !strings.Contains(out, "req_abc123") {
		t.Errorf("expected request_id in output\ngot: %q", out)
	}
}

func TestRenderError_Quiet(t *testing.T) {
	var buf bytes.Buffer
	RenderError(&buf, "quiet", "AUTH_ERROR", "Invalid key", "req_xyz")
	out := buf.String()
	if !strings.Contains(out, "Error:") {
		t.Errorf("expected 'Error:' prefix in quiet mode\ngot: %q", out)
	}
}

func TestRender_UnknownKind(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "nonexistent"}
	err := Render(&buf, "table", []Task{}, opts)
	if err == nil {
		t.Fatal("expected error for unknown resource kind")
	}
}

func TestRender_AllResourceKinds(t *testing.T) {
	tests := []struct {
		kind string
		data any
	}{
		{"tenant", []Tenant{{Code: "acme", Name: "ACME Corp"}}},
		{"member", []Member{{ID: "m1", Name: "Alice", Email: "alice@acme.com", Role: "owner"}}},
		{"board", []Board{{ID: "b1", Title: "Sprint 1", TenantCode: "acme"}}},
		{"task", []Task{{ID: "t1", Title: "Task 1", Status: "open"}}},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			for _, mode := range []string{"table", "json", "quiet"} {
				var buf bytes.Buffer
				opts := RenderOpts{ResourceKind: tc.kind}
				if err := Render(&buf, mode, tc.data, opts); err != nil {
					t.Errorf("mode=%s kind=%s: unexpected error: %v", mode, tc.kind, err)
				}
			}
		})
	}
}
