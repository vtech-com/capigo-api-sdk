package output

import (
	"bytes"
	"strings"
	"testing"
)

var fakeComments = []TaskComment{
	{ID: "msg_001", Created: "2026-06-01T08:00:00Z", Author: "Trâm", Kind: "comment", Content: "đã xong phần A", Attachments: 0},
	{ID: "msg_002", Created: "2026-06-01T09:00:00Z", Author: "Sơn", Kind: "activity", Content: "Sơn changed status from Doing to Done", Attachments: 1},
}

func TestRender_TaskComment_Table(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "task_comment"}
	if err := Render(&buf, "table", fakeComments, opts); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Created", "Author", "Kind", "Content", "Files", "comment", "activity", "Trâm", "Sơn"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q\ngot:\n%s", want, out)
		}
	}
	// ID is deliberately not a table column (kept only for quiet mode).
	if strings.Contains(out, "msg_001") {
		t.Errorf("comment ID should not appear in table output\ngot:\n%s", out)
	}
}

func TestRender_TaskComment_Quiet(t *testing.T) {
	var buf bytes.Buffer
	opts := RenderOpts{ResourceKind: "task_comment"}
	if err := Render(&buf, "quiet", fakeComments, opts); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := strings.Fields(buf.String())
	want := []string{"msg_001", "msg_002"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("quiet mode should print IDs %v, got %v", want, got)
	}
}
