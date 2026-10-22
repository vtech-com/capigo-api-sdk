package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeSubtaskItemsKeepsTheAssignee(t *testing.T) {
	const id = "6f1c9a3d-8b2e-4f01-9c77-1a3d5e7f9b21"
	items, err := decodeSubtaskItems("--from-json", []byte(`[
		{"title":"A","assignee":"`+id+`"},
		{"title":"B","assignee_id":"`+id+`"},
		{"title":"C","assignee":"`+id+`","assignee_id":"`+id+`"},
		{"title":"D"}]`))
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range items[:3] {
		if it.AssigneeID == nil || *it.AssigneeID != id {
			t.Errorf("item %d: assignee_id = %v, want %s", i, it.AssigneeID, id)
		}
	}
	if items[3].AssigneeID != nil {
		t.Errorf("item 3: assignee_id = %v, want none", *items[3].AssigneeID)
	}
	body, _ := json.Marshal(items[0])
	if !strings.Contains(string(body), `"assignee_id":"`+id+`"`) || strings.Contains(string(body), `"assignee":`) {
		t.Errorf("wire item = %s, want assignee_id only", body)
	}
}

func TestDecodeSubtaskItemsRefusesWhatItWouldDrop(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`[{"title":"A","asignee":"x"}]`, `unknown key "asignee"`},
		{`[{"title":"A","assignee":"x","assignee_id":"y"}]`, "differ"},
		{`{"title":"A"}`, "must be a JSON array"},
	} {
		_, err := decodeSubtaskItems("--subtasks-json", []byte(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "--subtasks-json") {
			t.Errorf("%s: err = %v, want one naming the flag and %q", c.in, err, c.want)
		}
	}
}
