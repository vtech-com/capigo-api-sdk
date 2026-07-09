package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

func TestTaskAttachmentDownloadPath(t *testing.T) {
	got := taskAttachmentDownloadPath("task-1", "att-1")
	want := "/mission/tasks/task-1/attachments/att-1/download"
	if got != want {
		t.Errorf("taskAttachmentDownloadPath = %q, want %q", got, want)
	}
}

func TestCommentAttachmentDownloadPath(t *testing.T) {
	got := commentAttachmentDownloadPath("task-1", "att-1")
	want := "/mission/tasks/task-1/comments/attachments/att-1/download"
	if got != want {
		t.Errorf("commentAttachmentDownloadPath = %q, want %q", got, want)
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
	if err := output.Write(&buf, data, itemMeta(&tenant, "acme")); err != nil {
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
