package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

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
