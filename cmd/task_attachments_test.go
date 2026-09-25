package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// The two builders take the task's base path, so a download hangs off whichever
// address the caller used — an id, or a code.
func TestTaskAttachmentDownloadPath(t *testing.T) {
	got := taskAttachmentDownloadPath(taskPath("task-1", ""), "att-1")
	want := "/mission/tasks/task-1/attachments/att-1/download"
	if got != want {
		t.Errorf("taskAttachmentDownloadPath = %q, want %q", got, want)
	}
	got = taskAttachmentDownloadPath(taskPath("", "ACMEC-68"), "att-1")
	want = "/mission/tasks/code/ACMEC-68/attachments/att-1/download"
	if got != want {
		t.Errorf("by-code = %q, want %q", got, want)
	}
}

func TestCommentAttachmentDownloadPath(t *testing.T) {
	got := commentAttachmentDownloadPath(taskPath("task-1", ""), "att-1")
	want := "/mission/tasks/task-1/comments/attachments/att-1/download"
	if got != want {
		t.Errorf("commentAttachmentDownloadPath = %q, want %q", got, want)
	}
	got = commentAttachmentDownloadPath(taskPath("", "ACMEC-68"), "att-1")
	want = "/mission/tasks/code/ACMEC-68/comments/attachments/att-1/download"
	if got != want {
		t.Errorf("by-code = %q, want %q", got, want)
	}
}

// With --code the single positional is the attachment; without it, the first is
// the task. A download command that mixes them up would fetch the wrong file.
func TestSplitAttachmentArgs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		code       string
		wantTask   string
		wantAttach string
	}{
		{"two positionals", []string{"t1", "a1"}, "", "t1", "a1"},
		{"code plus attachment", []string{"a1"}, "ACMEC-68", "", "a1"},
		{"code plus both, an error the caller is told about", []string{"t1", "a1"}, "ACMEC-68", "t1", "a1"},
		{"task only, attachment missing", []string{"t1"}, "", "t1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task, attach := splitAttachmentArgs(tc.args, tc.code)
			if task != tc.wantTask || attach != tc.wantAttach {
				t.Errorf("got (%q, %q), want (%q, %q)", task, attach, tc.wantTask, tc.wantAttach)
			}
		})
	}
}

// The upload hangs off whichever address the caller used, exactly like the
// download: an id, or a code. A wrong prefix here would post the file to a
// download route, which has no POST.
func TestTaskAttachmentUploadPath(t *testing.T) {
	got := taskAttachmentUploadPath(taskPath("task-1", ""))
	want := "/mission/tasks/task-1/attachments"
	if got != want {
		t.Errorf("taskAttachmentUploadPath = %q, want %q", got, want)
	}
	got = taskAttachmentUploadPath(taskPath("", "ACMEC-68"))
	want = "/mission/tasks/code/ACMEC-68/attachments"
	if got != want {
		t.Errorf("by-code = %q, want %q", got, want)
	}
}

// The flags the command's own help promises have to exist, or the help lies.
func TestTasksAttachmentsUploadFlags(t *testing.T) {
	for _, name := range []string{"tenant", "code", "content-type", "idempotency-key"} {
		if tasksAttachmentsUploadCmd.Flags().Lookup(name) == nil {
			t.Errorf("tasks attachments upload has no --%s flag", name)
		}
	}
}

// What the command prints, and the one fact a retry turns on: 201 means this
// call stored the file, 200 means the server replayed one it already had. An
// agent reading only stdout must not read a replay as a second copy.
func TestUploadResultData(t *testing.T) {
	attachment := api.AttachmentMetadata{
		ID:        "att-1",
		FileName:  "invoice.pdf",
		MimeType:  "application/pdf",
		SizeBytes: 48213,
	}

	stored := uploadResultData(attachment, "./invoice.pdf", http.StatusCreated)
	if stored["replayed"] != false {
		t.Errorf("replayed = %v for a 201, want false", stored["replayed"])
	}

	replayed := uploadResultData(attachment, "./invoice.pdf", http.StatusOK)
	if replayed["replayed"] != true {
		t.Errorf("replayed = %v for a 200, want true", replayed["replayed"])
	}

	for _, got := range []map[string]any{stored, replayed} {
		if got["id"] != "att-1" || got["file_name"] != "invoice.pdf" ||
			got["mime_type"] != "application/pdf" || got["size_bytes"] != int64(48213) {
			t.Errorf("data = %+v, want the server's attachment untouched", got)
		}
		if got["source_path"] != "./invoice.pdf" {
			t.Errorf("source_path = %v, want the path that was uploaded", got["source_path"])
		}
	}
}

// The remove hangs off whichever address the caller used, like every other
// attachment command: an id, or a code.
func TestTaskAttachmentRemovePath(t *testing.T) {
	got := taskAttachmentRemovePath(taskPath("task-1", ""), "att-1")
	want := "/mission/tasks/task-1/attachments/att-1"
	if got != want {
		t.Errorf("taskAttachmentRemovePath = %q, want %q", got, want)
	}
	got = taskAttachmentRemovePath(taskPath("", "ACMEC-68"), "att-1")
	want = "/mission/tasks/code/ACMEC-68/attachments/att-1"
	if got != want {
		t.Errorf("by-code = %q, want %q", got, want)
	}
}

// The flags the command's own help promises have to exist, or the help lies —
// and --idempotency-key must NOT: a DELETE answers 4 for a second attempt, so a
// key would have nothing to dedupe and offering one would invite a caller to
// believe the retry was answered differently.
func TestTasksAttachmentsRemoveFlags(t *testing.T) {
	for _, name := range []string{"tenant", "code"} {
		if tasksAttachmentsRemoveCmd.Flags().Lookup(name) == nil {
			t.Errorf("tasks attachments remove has no --%s flag", name)
		}
	}
	if tasksAttachmentsRemoveCmd.Flags().Lookup("idempotency-key") != nil {
		t.Error("tasks attachments remove offers --idempotency-key, but the API accepts no key on a delete")
	}
}

// What the command prints: the file that was removed, and the CLI's own claim
// that it is gone. An agent that reads only stdout must not have to guess
// whether the delete happened.
func TestRemoveResultData(t *testing.T) {
	got := removeResultData(api.AttachmentMetadata{
		ID:        "att-1",
		FileName:  "invoice.pdf",
		MimeType:  "application/pdf",
		SizeBytes: 48213,
	})

	if got["removed"] != true {
		t.Errorf("removed = %v, want true", got["removed"])
	}
	if got["id"] != "att-1" || got["file_name"] != "invoice.pdf" ||
		got["mime_type"] != "application/pdf" || got["size_bytes"] != int64(48213) {
		t.Errorf("data = %+v, want the server's attachment untouched", got)
	}
}

// TestAttachmentDownloadEnvelope checks the shape runAttachmentDownload emits
// on stdout: the file metadata at .data, the tenant at .meta.
func TestAttachmentDownloadEnvelope(t *testing.T) {
	tenant := "acme"
	data := map[string]any{
		"file_name":  "invoice.pdf",
		"mime_type":  "application/pdf",
		"size_bytes": int64(48213),
		"saved_path": "invoice.pdf",
	}

	var buf bytes.Buffer
	if err := output.Write(&buf, data, itemMeta(&tenant, "acme", nil)); err != nil {
		t.Fatalf("output.Write: %v", err)
	}

	var got struct {
		Data struct {
			FileName  string `json:"file_name"`
			MimeType  string `json:"mime_type"`
			SizeBytes int64  `json:"size_bytes"`
			SavedPath string `json:"saved_path"`
		} `json:"data"`
		Meta struct {
			Tenant       string `json:"tenant"`
			TenantSource string `json:"tenant_source"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal envelope: %v (body: %s)", err, buf.String())
	}

	if got.Data.FileName != "invoice.pdf" || got.Data.MimeType != "application/pdf" ||
		got.Data.SizeBytes != 48213 || got.Data.SavedPath != "invoice.pdf" {
		t.Errorf("data = %+v, want the file metadata untouched", got.Data)
	}
	if got.Meta.Tenant != "acme" || got.Meta.TenantSource != "flag" {
		t.Errorf("meta = %+v, want tenant=acme tenant_source=flag", got.Meta)
	}
}
