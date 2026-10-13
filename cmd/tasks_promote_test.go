package cmd

import (
	"encoding/json"
	"testing"
)

func promoteMeta(t *testing.T, printed string) map[string]any {
	t.Helper()
	var doc struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(printed), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, printed)
	}
	return doc.Meta
}

func TestTasksPromoteSendsTheListOnlyWhenGiven(t *testing.T) {
	const list = "9ab2c744-16fe-4d09-8a52-3ef0b7c61d84"

	taskPromoteTenant, taskPromoteCode, taskPromoteList = "acme", "ACME-7", ""
	seen, _ := callAgainst(t, `{"data":{"code":"ACME-7","parent":null,"board_list_id":null}}`,
		func() { _ = tasksPromoteCmd.RunE(tasksPromoteCmd, nil) })
	if seen.method != "POST" || seen.path != "/mission/tasks/code/ACME-7/actions/promote" || seen.tenant != "acme" {
		t.Errorf("call = %s %s tenant=%q", seen.method, seen.path, seen.tenant)
	}
	if seen.hasBody {
		t.Errorf("no --list must send no body, got %s", seen.rawBody)
	}

	taskPromoteCode, taskPromoteList = "", list
	seen, printed := callAgainst(t, `{"data":{"id":"t1","parent":null,"board_list_id":"`+list+`"}}`,
		func() { _ = tasksPromoteCmd.RunE(tasksPromoteCmd, []string{"t1"}) })
	taskPromoteTenant, taskPromoteList = "", ""

	if seen.path != "/mission/tasks/t1/actions/promote" || seen.body["board_list_id"] != list {
		t.Errorf("call = %s body=%v", seen.path, seen.body)
	}
	if _, flagged := promoteMeta(t, printed)["list_applied"]; flagged {
		t.Error("a task that landed in the named list must not be flagged")
	}
}

// A task that was already top-level answers 200 unchanged and ignores the list.
// The answer looks like a success, and a caller who named a list would report a
// placement that never happened.
func TestTasksPromoteSaysWhenTheListWasIgnored(t *testing.T) {
	taskPromoteTenant, taskPromoteCode, taskPromoteList = "acme", "", "9ab2c744-16fe-4d09-8a52-3ef0b7c61d84"
	_, printed := callAgainst(t, `{"data":{"id":"t1","parent":null,"board_list_id":"11111111-2222-4333-8444-555555555555"}}`,
		func() { _ = tasksPromoteCmd.RunE(tasksPromoteCmd, []string{"t1"}) })
	taskPromoteTenant, taskPromoteList = "", ""

	if promoteMeta(t, printed)["list_applied"] != false {
		t.Errorf("meta.list_applied must be false when the task is not in the list sent:\n%s", printed)
	}
}
