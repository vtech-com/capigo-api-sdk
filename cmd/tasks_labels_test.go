package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
)

// withDefaultProfile swaps in a config holding an empty "default" profile.
// tasks create and update read the profile directly, which callAgainst's empty
// config does not have. Call it inside the run func, after callAgainst's own
// withConfig.
func withDefaultProfile(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"active_profile":"default","profiles":{"default":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := config.ConfigPath
	config.ConfigPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { config.ConfigPath = orig })
}

func TestTasksCreateSendsLabelNames(t *testing.T) {
	taskCreateTenant, taskCreateTitle = "acme", "Fix login"
	taskCreateLabels = []string{"urgent", "q3 plan, v2"}
	t.Cleanup(func() { taskCreateTenant, taskCreateTitle, taskCreateLabels = "", "", nil })

	seen, _ := callAgainst(t, `{"data":{"id":"t1","labels":[]}}`,
		func() { withDefaultProfile(t); _ = tasksCreateCmd.RunE(tasksCreateCmd, nil) })
	if seen.path != "/mission/tasks" {
		t.Fatalf("path = %s", seen.path)
	}
	if got := seen.body["label_names"]; !reflect.DeepEqual(got, []any{"urgent", "q3 plan, v2"}) {
		t.Errorf("label_names = %v", got)
	}

	taskCreateLabels = nil
	seen, _ = callAgainst(t, `{"data":{"id":"t1"}}`,
		func() { withDefaultProfile(t); _ = tasksCreateCmd.RunE(tasksCreateCmd, nil) })
	if _, sent := seen.body["label_names"]; sent {
		t.Error("label_names must be omitted when --label is not given")
	}
}

func TestTasksUpdateSendsLabelAddAndRemove(t *testing.T) {
	taskUpdateTenant = "acme"
	taskUpdateLabels = []string{"urgent"}
	taskUpdateRemoveLabels = []string{"later"}
	t.Cleanup(func() { taskUpdateTenant, taskUpdateLabels, taskUpdateRemoveLabels = "", nil, nil })

	seen, _ := callAgainst(t, `{"data":{"id":"t1"}}`,
		func() { withDefaultProfile(t); _ = tasksUpdateCmd.RunE(tasksUpdateCmd, []string{"t1"}) })
	if seen.method != "PATCH" || seen.path != "/mission/tasks/t1" {
		t.Fatalf("call = %s %s", seen.method, seen.path)
	}
	if !reflect.DeepEqual(seen.body["label_names"], []any{"urgent"}) ||
		!reflect.DeepEqual(seen.body["label_remove_names"], []any{"later"}) {
		t.Errorf("body = %v", seen.body)
	}
}

func TestTasksListBuildsLabelFilter(t *testing.T) {
	got := tasksListPath(taskListFilters{labels: []string{"a", "b"}})
	if got != "/mission/tasks?filters%5Blabels%5D%5B%24all%5D=a%2Cb" {
		t.Errorf("path = %s", got)
	}
	if got := tasksListPath(taskListFilters{}); got != "/mission/tasks" {
		t.Errorf("no labels must add no param, got %s", got)
	}
}

func TestTasksListRefusesLabelInBoardMode(t *testing.T) {
	for name, f := range map[string]taskListFilters{
		"unassigned": {labels: []string{"a"}, unassigned: true},
		"date field": {labels: []string{"a"}, dateField: "due_date", dateTo: "2026-09-30T23:59:59+07:00"},
	} {
		e := validateTaskListFlags(f)
		if e == nil || api.ExitCodeFor(e) != 5 || !strings.Contains(e.Message, "--label") {
			t.Errorf("%s: got %v, want exit 5 naming --label", name, e)
		}
	}
	if e := validateTaskListFlags(taskListFilters{labels: []string{"a"}, scope: "mine"}); e != nil {
		t.Errorf("--label with scope mine must be accepted: %v", e)
	}
}
