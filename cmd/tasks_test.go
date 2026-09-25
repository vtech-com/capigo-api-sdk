package cmd

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// The flags the command's own help promises have to exist, or the help lies.
func TestTasksCommentsCreateFileFlags(t *testing.T) {
	for _, name := range []string{"file", "content-type", "idempotency-key", "content", "attachments-json"} {
		if tasksCommentsCreateCmd.Flags().Lookup(name) == nil {
			t.Errorf("tasks comments create has no --%s flag", name)
		}
	}
}

// The detection is a guess, so the override has to reach every part it was given
// for — otherwise the flag that exists to correct a wrong guess cannot.
func TestReadUploadParts_UsesTheDeclaredType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.unknownext")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	parts, err := readUploadParts([]string{path}, "text/markdown")
	if err != nil {
		t.Fatalf("readUploadParts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(parts))
	}
	if parts[0].ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want the declared text/markdown", parts[0].ContentType)
	}
}

// readUploadParts reads each --file path and declares the media type it detects:
// the server checks that type against its allow-list, so a blank one would be
// refused as "no Content-Type" with nothing to fix.
func TestReadUploadParts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	parts, err := readUploadParts([]string{path}, "")
	if err != nil {
		t.Fatalf("readUploadParts: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("parts = %d, want 1", len(parts))
	}

	got := parts[0]
	if got.FieldName != "file" {
		t.Errorf("FieldName = %q, want file — the API looks for that part name", got.FieldName)
	}
	if got.FileName != "notes.txt" {
		t.Errorf("FileName = %q, want the base name", got.FileName)
	}
	if got.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", got.ContentType)
	}
	if string(got.Data) != "hello" {
		t.Errorf("Data = %q, want hello", string(got.Data))
	}
}

// A path that is not there is refused locally, before a request is built — the
// same rule tasks attachments upload follows.
func TestReadUploadParts_RefusesAMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.txt")

	if _, err := readUploadParts([]string{missing}, ""); err == nil {
		t.Error("readUploadParts accepted a path that does not exist")
	}
}

// A blank key is not a key: the API trims the header and reads the blank as
// absent, so passing one through unchanged would post the duplicate the flag
// exists to prevent. The exit itself goes through failValidation, which os.Exit
// makes untestable in-process; what is tested here is that a key nobody asked
// for stays empty and a real key keeps its characters.
func TestRequireUsableKey(t *testing.T) {
	if got := requireUsableKey(false, "  ignored  ", "idempotency-key"); got != "" {
		t.Errorf("a key that was not given = %q, want empty", got)
	}
	if got := requireUsableKey(true, "  comment-42  ", "idempotency-key"); got != "comment-42" {
		t.Errorf("a given key = %q, want it trimmed to comment-42", got)
	}
}

// A declared media type with no characters is the same class of mistake: sent
// as-is, the server trims it to nothing and refuses the file after the upload.
// The exit goes through failValidation; what is tested is that "not declared"
// stays empty (which is a detection) and a real type keeps its characters.
func TestRequireUsableMediaType(t *testing.T) {
	if got := requireUsableMediaType(false, "  ignored  ", "content-type"); got != "" {
		t.Errorf("a type that was not declared = %q, want empty so detection runs", got)
	}
	if got := requireUsableMediaType(true, " text/plain ", "content-type"); got != "text/plain" {
		t.Errorf("a declared type = %q, want it trimmed to text/plain", got)
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
	if got := commentsPath(taskPath("t1", ""), "", "", 0, 0); got != "/mission/tasks/t1/comments" {
		t.Errorf("bare path = %q", got)
	}

	// All flags set → every param present with the right value.
	got := commentsPath(taskPath("t1", ""), "activity", "asc", 2, 30)
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
	got = commentsPath(taskPath("t1", ""), "comment", "", 0, 0)
	if strings.Contains(got, "sort=") || strings.Contains(got, "page=") || strings.Contains(got, "limit=") {
		t.Errorf("zero/empty flags should be omitted, got %q", got)
	}

	// Addressed by code, the comments hang off the code route, not the id one.
	if got := commentsPath(taskPath("", "ACMEC-68"), "", "", 0, 0); got != "/mission/tasks/code/ACMEC-68/comments" {
		t.Errorf("by-code path = %q", got)
	}
}

// A code is a value someone typed. Unescaped, one containing a slash would
// address a different route entirely.
func TestTaskPathEscapesItsAddress(t *testing.T) {
	if got := taskPath("", "AC/ME-1"); got != "/mission/tasks/code/AC%2FME-1" {
		t.Errorf("code not escaped: %q", got)
	}
	if got := taskPath("a b", ""); got != "/mission/tasks/a%20b" {
		t.Errorf("id not escaped: %q", got)
	}
	// A code wins only because exactly one address is ever set; the guard is in
	// requireOneTaskAddress, and taskPath must not silently prefer one.
	if got := taskPath("", "X-1"); got != "/mission/tasks/code/X-1" {
		t.Errorf("code path = %q", got)
	}
}

// TestTaskActionPath covers the action routes: they hang off whatever address
// taskPath produced, so an id and a code both reach the same action.
func TestTaskActionPath(t *testing.T) {
	for _, action := range []string{"assign-agent", "move", "transfer-ownership", "claim", "archive", "unarchive"} {
		for _, tc := range []struct{ id, code, base string }{
			{id: "task-1", base: "/mission/tasks/task-1"},
			{code: "ACME-1", base: "/mission/tasks/code/ACME-1"},
		} {
			want := tc.base + "/actions/" + action
			if got := taskPath(tc.id, tc.code) + "/actions/" + action; got != want {
				t.Errorf("taskPath(%q, %q) + %s = %q, want %q", tc.id, tc.code, action, got, want)
			}
		}
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
		archive:       true,
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
		"include_archived":            "true",
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

	// --include-archived is opt-in: unflagged, the call must not ask for
	// archived rows at all, because "false" and "absent" are the same answer to
	// the server and a stray parameter would hide that.
	if got := tasksListPath(taskListFilters{status: "Doing", archive: false}); strings.Contains(got, "include_archived") {
		t.Errorf("archive=false must send no include_archived, got %q", got)
	}
	if got := tasksListPath(taskListFilters{archive: true}); got != "/mission/tasks?include_archived=true" {
		t.Errorf("archive=true path = %q", got)
	}
}

// TestIncludeArchivedPath covers the read flag: it appends the parameter only
// when asked, and appends nothing at all otherwise — the bare path is what every
// unflagged read has always sent.
func TestIncludeArchivedPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id, code string
		archived bool
		want     string
	}{
		{name: "id, unflagged", id: "task-1", want: "/mission/tasks/task-1"},
		{name: "id, flagged", id: "task-1", archived: true, want: "/mission/tasks/task-1?include_archived=true"},
		{name: "code, unflagged", code: "ACME-1", want: "/mission/tasks/code/ACME-1"},
		{name: "code, flagged", code: "ACME-1", archived: true, want: "/mission/tasks/code/ACME-1?include_archived=true"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := includeArchivedPath(taskPath(tc.id, tc.code), tc.archived); got != tc.want {
				t.Errorf("includeArchivedPath = %q, want %q", got, tc.want)
			}
		})
	}
}
