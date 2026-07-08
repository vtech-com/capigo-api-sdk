package cmd

import (
	"net/url"
	"strings"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

func commentStrPtr(s string) *string { return &s }

func TestToOutputComment(t *testing.T) {
	c := api.TaskComment{
		ID:          "msg_1",
		Author:      api.CommentAuthor{ID: "u1", Name: "Trâm", Type: "user"},
		Kind:        "comment",
		Content:     commentStrPtr("hello"),
		Attachments: []api.CommentAttachment{{ID: "a1"}, {ID: "a2"}},
		CreatedAt:   "2026-06-01T08:00:00Z",
	}
	got := toOutputComment(c)
	if got.ID != "msg_1" || got.Author != "Trâm" || got.Kind != "comment" {
		t.Errorf("unexpected mapping: %+v", got)
	}
	if got.Content != "hello" {
		t.Errorf("content = %q, want hello", got.Content)
	}
	if got.Created != "2026-06-01T08:00:00Z" {
		t.Errorf("created = %q", got.Created)
	}
	if got.Attachments != 2 {
		t.Errorf("attachments = %d, want 2", got.Attachments)
	}
}

func TestToOutputComment_NilContent(t *testing.T) {
	c := api.TaskComment{
		ID:      "m",
		Author:  api.CommentAuthor{Name: "System", Type: "user"},
		Kind:    "activity",
		Content: nil,
	}
	got := toOutputComment(c)
	if got.Content != "" {
		t.Errorf("nil content should map to empty string, got %q", got.Content)
	}
}

func TestFlattenForTable(t *testing.T) {
	// Collapses newlines/tabs/runs of whitespace into single spaces.
	if got := flattenForTable("a\nb\t c", 100); got != "a b c" {
		t.Errorf("flatten = %q, want 'a b c'", got)
	}
	// Truncates to max runes with an ellipsis.
	got := flattenForTable(strings.Repeat("x", 50), 10)
	if r := []rune(got); len(r) != 10 || !strings.HasSuffix(got, "…") {
		t.Errorf("truncate = %q (len %d runes), want 10 runes ending in …", got, len([]rune(got)))
	}
}

func TestValidateCommentParams(t *testing.T) {
	// Valid combinations (including empty = use server default) return nil.
	for _, c := range []struct {
		typeFlag, sortFlag string
		limit              int
	}{
		{"", "", 0},
		{"comment", "asc", 20},
		{"activity", "desc", 50},
	} {
		if e := validateCommentParams(c.typeFlag, c.sortFlag, c.limit); e != nil {
			t.Errorf("validateCommentParams(%q,%q,%d) = %v, want nil", c.typeFlag, c.sortFlag, c.limit, e)
		}
	}

	// Invalid values return a VALIDATION_ERROR (HTTP 400 → exit 5).
	for _, c := range []struct {
		name               string
		typeFlag, sortFlag string
		limit              int
	}{
		{"bad type", "foo", "", 0},
		{"bad sort", "", "sideways", 0},
		{"limit over cap", "", "", 51},
	} {
		e := validateCommentParams(c.typeFlag, c.sortFlag, c.limit)
		if e == nil {
			t.Errorf("%s: expected error, got nil", c.name)
			continue
		}
		if e.Code != "VALIDATION_ERROR" || e.HTTPStatus != 400 {
			t.Errorf("%s: got %+v, want VALIDATION_ERROR/400", c.name, e)
		}
		if api.ExitCodeFor(e) != 5 {
			t.Errorf("%s: exit code = %d, want 5", c.name, api.ExitCodeFor(e))
		}
	}
}

func TestCommentsPath(t *testing.T) {
	// No flags → bare path, no query string.
	if got := commentsPath("t1", "", "", 0, 0); got != "/mission/tasks/t1/comments" {
		t.Errorf("bare path = %q", got)
	}

	// All flags set → every param present with the right value.
	got := commentsPath("t1", "activity", "asc", 2, 30)
	base, query, found := strings.Cut(got, "?")
	if !found || base != "/mission/tasks/t1/comments" {
		t.Fatalf("path = %q, want base + query", got)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parse query %q: %v", query, err)
	}
	for k, want := range map[string]string{"type": "activity", "sort": "asc", "page": "2", "limit": "30"} {
		if q.Get(k) != want {
			t.Errorf("query %s = %q, want %q", k, q.Get(k), want)
		}
	}

	// Zero/empty values are omitted from the query.
	got = commentsPath("t1", "comment", "", 0, 0)
	if strings.Contains(got, "sort=") || strings.Contains(got, "page=") || strings.Contains(got, "limit=") {
		t.Errorf("zero/empty flags should be omitted, got %q", got)
	}
}

// TestTasksListPath is a regression test for the tasks list filter gap: the
// backend (query-parser.ts ALLOWED_FILTER_COLUMNS) accepts filters on status,
// priority, assignee_id, owner_id, board_id, board_list_id, due_date, and
// created_at, but the CLI used to expose only --status. It now covers every
// allowed column.
func TestTasksListPath(t *testing.T) {
	// No flags → bare path, no query string.
	if got := tasksListPath(taskListFilters{}); got != "/mission/tasks" {
		t.Errorf("bare path = %q", got)
	}

	got := tasksListPath(taskListFilters{
		query:         "invoice",
		status:        "Doing",
		priority:      "high",
		assigneeID:    "u1",
		ownerID:       "u2",
		boardID:       "b1",
		boardListID:   "bl1",
		dueAfter:      "2026-07-01",
		dueBefore:     "2026-07-31",
		createdAfter:  "2026-06-01T00:00:00Z",
		createdBefore: "2026-06-30T00:00:00Z",
		parentTaskID:  "p1",
		page:          2,
		limit:         30,
	})
	base, query, found := strings.Cut(got, "?")
	if !found || base != "/mission/tasks" {
		t.Fatalf("path = %q, want base + query", got)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("parse query %q: %v", query, err)
	}
	want := map[string]string{
		"q":                           "invoice",
		"filters[status][$eq]":        "Doing",
		"filters[priority][$eq]":      "high",
		"filters[assignee_id][$eq]":   "u1",
		"filters[owner_id][$eq]":      "u2",
		"filters[board_id][$eq]":      "b1",
		"filters[board_list_id][$eq]": "bl1",
		"filters[due_date][$gte]":     "2026-07-01",
		"filters[due_date][$lte]":     "2026-07-31",
		"filters[created_at][$gte]":   "2026-06-01T00:00:00Z",
		"filters[created_at][$lte]":   "2026-06-30T00:00:00Z",
		"parent_task_id":              "p1",
		"page":                        "2",
		"limit":                       "30",
	}
	for k, wantVal := range want {
		if got := q.Get(k); got != wantVal {
			t.Errorf("query %s = %q, want %q", k, got, wantVal)
		}
	}

	// Zero/empty values are omitted from the query.
	if got := tasksListPath(taskListFilters{status: "Doing"}); strings.Contains(got, "priority") ||
		strings.Contains(got, "assignee_id") || strings.Contains(got, "page=") {
		t.Errorf("zero/empty flags should be omitted, got %q", got)
	}
}
