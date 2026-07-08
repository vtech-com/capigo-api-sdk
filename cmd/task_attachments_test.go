package cmd

import "testing"

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
