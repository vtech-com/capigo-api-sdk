package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCreateSubtasksRequest_Shape ensures the batch subtasks body carries
// tenant_code and a subtasks array, and that unset optional item fields are
// omitted (only title is required per item).
func TestCreateSubtasksRequest_Shape(t *testing.T) {
	desc := "detail"
	req := CreateSubtasksRequest{
		TenantCode: "acme",
		Subtasks: []SubtaskItem{
			{Title: "A"},
			{Title: "B", Description: &desc},
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)

	if !strings.Contains(s, `"tenant_code":"acme"`) {
		t.Errorf("missing tenant_code: %s", s)
	}
	if !strings.Contains(s, `"subtasks":[`) {
		t.Errorf("missing subtasks array: %s", s)
	}
	// First item has only title (optional fields omitted).
	if !strings.Contains(s, `{"title":"A"}`) {
		t.Errorf("first item should omit unset optional fields: %s", s)
	}
	if !strings.Contains(s, `"description":"detail"`) {
		t.Errorf("second item should carry description: %s", s)
	}
}

// TestCreateTaskWithSubtasksRequest_Shape ensures the atomic create body nests
// the parent task under "task" and the subtasks under "subtasks", with
// tenant_code at the envelope level.
func TestCreateTaskWithSubtasksRequest_Shape(t *testing.T) {
	req := CreateTaskWithSubtasksRequest{
		TenantCode: "acme",
		Task:       CreateTaskWithSubtasksTask{Title: "Parent"},
		Subtasks:   []SubtaskItem{{Title: "Child"}},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["tenant_code"] != "acme" {
		t.Errorf("tenant_code = %v, want acme", m["tenant_code"])
	}
	task, ok := m["task"].(map[string]any)
	if !ok || task["title"] != "Parent" {
		t.Errorf("task object malformed: %v", m["task"])
	}
	subs, ok := m["subtasks"].([]any)
	if !ok || len(subs) != 1 {
		t.Errorf("subtasks array malformed: %v", m["subtasks"])
	}
}
