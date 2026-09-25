package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A .pdf that starts with something other than "%PDF" is still a PDF for the
// API's purposes: the type is a declaration, and the extension is the best
// declaration available without a content-sniffing library.
func TestDetectContentType_PrefersTheExtension(t *testing.T) {
	got := DetectContentType("/tmp/invoice.pdf", []byte("hello"))
	if got != "application/pdf" {
		t.Errorf("DetectContentType = %q, want application/pdf", got)
	}
}

// The API's allow-list holds bare types, so a parameter would make text/plain
// fail a lookup that text/plain passes.
func TestDetectContentType_StripsParameters(t *testing.T) {
	got := DetectContentType("/tmp/notes.txt", []byte("hello"))
	if got != "text/plain" {
		t.Errorf("DetectContentType = %q, want text/plain (no charset parameter)", got)
	}
}

func TestDetectContentType_FallsBackToTheBytes(t *testing.T) {
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 16)...)
	got := DetectContentType("/tmp/unnamed-blob", png)
	if got != "image/png" {
		t.Errorf("DetectContentType = %q, want image/png", got)
	}
}

// A type the API does not accept is refused by the API, not guessed at here —
// but the guess must not be empty, which the server reports as "no declared
// Content-Type" and which says nothing about what was sent.
func TestDetectContentType_UnknownIsOctetStream(t *testing.T) {
	got := DetectContentType("/tmp/unnamed-blob", []byte{0x00, 0x01, 0x02, 0x03})
	if got != "application/octet-stream" {
		t.Errorf("DetectContentType = %q, want application/octet-stream", got)
	}
}

func TestReadUploadFile_ReadsTheBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	data, err := ReadUploadFile(path)
	if err != nil {
		t.Fatalf("ReadUploadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("data = %q, want hello", string(data))
	}
}

// Every refusal here is one the API would also make. The point is to make it
// without spending the upload first.
func TestReadUploadFile_RefusesWhatTheAPICannotAccept(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty fixture: %v", err)
	}

	oversize := filepath.Join(dir, "big.bin")
	f, err := os.Create(oversize)
	if err != nil {
		t.Fatalf("create oversize fixture: %v", err)
	}
	// Sparse: one byte past the limit without writing 50 MB to disk.
	if err := f.Truncate(MaxTaskAttachmentBytes + 1); err != nil {
		t.Fatalf("truncate oversize fixture: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close oversize fixture: %v", err)
	}

	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{"missing file", filepath.Join(dir, "nope.txt")},
		{"directory", subdir},
		{"empty file", empty},
		{"over the limit", oversize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := ReadUploadFile(tc.path)
			if err == nil {
				t.Fatalf("ReadUploadFile(%q) returned %d bytes, want an error", tc.path, len(data))
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v, want an *APIError so it renders and exits like every other refusal", err)
			}
			if apiErr.HTTPStatus != 400 {
				t.Errorf("HTTPStatus = %d, want 400 (exit 5)", apiErr.HTTPStatus)
			}
		})
	}
}

// The request the API actually receives: one part named `file`, carrying the
// declared type and the bytes — and a Content-Length, which is what lets the
// server refuse an oversized body before reading it.
func TestUploadTaskAttachment_SendsOneFilePart(t *testing.T) {
	var gotPartName, gotFileName, gotPartType, gotBody string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mission/tasks/task-1/attachments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart/form-data with a boundary", r.Header.Get("Content-Type"))
		}
		if r.ContentLength <= 0 {
			t.Errorf("ContentLength = %d, want a known length so the server can refuse an oversized body", r.ContentLength)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "upload-42" {
			t.Errorf("Idempotency-Key = %q, want upload-42", got)
		}
		if got := r.Header.Get("X-Tenant-Code"); got != "acme" {
			t.Errorf("X-Tenant-Code = %q, want acme", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer csk_testkey123" {
			t.Errorf("Authorization = %q, want the key", got)
		}

		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		gotPartName = part.FormName()
		gotFileName = part.FileName()
		gotPartType = part.Header.Get("Content-Type")
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		gotBody = string(body)

		// Exactly one part: a second one would be a field the API ignores.
		if _, err := mr.NextPart(); err != io.EOF {
			t.Errorf("second part = %v, want io.EOF", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"id":"att-1","file_name":"notes.txt","mime_type":"text/plain","size_bytes":5}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	tenant := "acme"

	resp, err := client.UploadTaskAttachment(
		context.Background(),
		"/mission/tasks/task-1/attachments",
		"notes.txt",
		"text/plain",
		[]byte("hello"),
		&tenant,
		"upload-42",
	)
	if err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}

	if gotPartName != "file" {
		t.Errorf("part name = %q, want file — the API looks for exactly this name", gotPartName)
	}
	if gotFileName != "notes.txt" {
		t.Errorf("part filename = %q, want notes.txt", gotFileName)
	}
	if gotPartType != "text/plain" {
		t.Errorf("part Content-Type = %q, want text/plain — the API checks its allow-list against this", gotPartType)
	}
	if gotBody != "hello" {
		t.Errorf("part body = %q, want hello", gotBody)
	}
}

// No key means no header at all: an empty Idempotency-Key would have every
// upload share one key on the server.
func TestUploadTaskAttachment_OmitsTheIdempotencyHeaderWhenEmpty(t *testing.T) {
	var present bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	if _, err := client.UploadTaskAttachment(
		context.Background(), "/mission/tasks/task-1/attachments", "notes.txt", "text/plain", []byte("hello"), nil, "",
	); err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}
	if present {
		t.Error("Idempotency-Key was sent for an upload that named no key")
	}
}

// A replay answers 200, not 201: the server stored nothing this time, and the
// caller has to be able to tell the two apart.
func TestUploadTaskAttachment_ReplayIs200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"att-1","file_name":"notes.txt","mime_type":"text/plain","size_bytes":5}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.UploadTaskAttachment(
		context.Background(), "/mission/tasks/task-1/attachments", "notes.txt", "text/plain", []byte("hello"), nil, "upload-42",
	)
	if err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}

	var body TaskAttachmentEnvelope
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.ID != "att-1" || body.Data.SizeBytes != 5 {
		t.Errorf("data = %+v, want the replayed attachment", body.Data)
	}
}

// A key reused for a different file is a conflict the caller must be able to
// act on, not a generic failure.
func TestUploadTaskAttachment_ConflictIs409(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"E0601","message":"Idempotency-Key was already used to attach a different file to this task"}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	_, err := client.UploadTaskAttachment(
		context.Background(), "/mission/tasks/task-1/attachments", "other.txt", "text/plain", []byte("hello"), nil, "upload-42",
	)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want an *APIError", err)
	}
	if apiErr.Code != "E0601" {
		t.Errorf("Code = %q, want E0601", apiErr.Code)
	}
	if got := ExitCodeFor(err); got != 8 {
		t.Errorf("ExitCodeFor = %d, want 8", got)
	}
}

// A file name with a quote in it must not break out of the Content-Disposition
// parameter: the part name is what the server looks for, and a malformed header
// would fail the whole request.
//
// The fixture carries no path separator on purpose: both ends' multipart
// parsers strip directory information from a file name (RFC 7578 §4.2), so a
// name containing one is not round-tripped by either side and would say nothing
// about the escaping this test is here for.
func TestUploadTaskAttachment_EscapesTheFileName(t *testing.T) {
	var gotFileName string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		gotFileName = part.FileName()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	if _, err := client.UploadTaskAttachment(
		context.Background(), "/mission/tasks/task-1/attachments", `we"ird name.txt`, "text/plain", []byte("hello"), nil, "",
	); err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}
	if gotFileName != `we"ird name.txt` {
		t.Errorf("part filename = %q, want it round-tripped unchanged", gotFileName)
	}
}

// A comment with files sends the text as a form field and each file as its own
// part, in the order given: the API reads `content` and repeated `file` parts.
func TestPostTaskCommentWithAttachments_SendsFieldsAndParts(t *testing.T) {
	type part struct {
		name        string
		fileName    string
		contentType string
		body        string
	}
	var parts []part

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/mission/tasks/task-1/comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data; boundary=") {
			t.Errorf("Content-Type = %q, want multipart/form-data with a boundary", r.Header.Get("Content-Type"))
		}
		if r.ContentLength <= 0 {
			t.Errorf("ContentLength = %d, want a known length so the server can refuse an oversized body", r.ContentLength)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "comment-42" {
			t.Errorf("Idempotency-Key = %q, want comment-42", got)
		}

		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			body, err := io.ReadAll(p)
			if err != nil {
				t.Fatalf("read part: %v", err)
			}
			parts = append(parts, part{
				name:        p.FormName(),
				fileName:    p.FileName(),
				contentType: p.Header.Get("Content-Type"),
				body:        string(body),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"msg-1","kind":"comment","content":"here it is"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	acme := "acme"

	resp, err := client.PostTaskCommentWithAttachments(
		context.Background(),
		"/mission/tasks/task-1/comments",
		"here it is",
		[]UploadPart{
			{FieldName: "file", FileName: "a.txt", ContentType: "text/plain", Data: []byte("one")},
			{FieldName: "file", FileName: "b.txt", ContentType: "text/plain", Data: []byte("two")},
		},
		&acme,
		"comment-42",
	)
	if err != nil {
		t.Fatalf("PostTaskCommentWithAttachments: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}

	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3 (one field, two files)", len(parts))
	}
	if parts[0].name != "content" || parts[0].body != "here it is" || parts[0].fileName != "" {
		t.Errorf("first part = %+v, want the content field", parts[0])
	}
	for i, want := range []part{
		{name: "file", fileName: "a.txt", contentType: "text/plain", body: "one"},
		{name: "file", fileName: "b.txt", contentType: "text/plain", body: "two"},
	} {
		got := parts[i+1]
		if got.name != want.name || got.fileName != want.fileName ||
			got.contentType != want.contentType || got.body != want.body {
			t.Errorf("file part %d = %+v, want %+v", i, got, want)
		}
	}
}

// Without a key the header is absent, as on the upload path: an empty
// Idempotency-Key would have every comment that omitted one share a key.
func TestPostTaskCommentWithAttachments_OmitsTheIdempotencyHeaderWhenEmpty(t *testing.T) {
	var present bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"msg-1"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	if _, err := client.PostTaskCommentWithAttachments(
		context.Background(), "/mission/tasks/task-1/comments", "hello", nil, nil, "",
	); err != nil {
		t.Fatalf("PostTaskCommentWithAttachments: %v", err)
	}
	if present {
		t.Error("Idempotency-Key was sent for a comment that named no key")
	}
}

// A replay answers 200 with the same body: the comment the earlier attempt
// wrote. The CLI turns that status into meta.replayed, so it has to survive
// the call untouched.
func TestPostTaskCommentWithAttachments_ReplayIs200(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg-1","kind":"comment","content":"here it is"}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	resp, err := client.PostTaskCommentWithAttachments(
		context.Background(), "/mission/tasks/task-1/comments", "here it is", nil, nil, "comment-42",
	)
	if err != nil {
		t.Fatalf("PostTaskCommentWithAttachments: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}

	var body struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "msg-1" {
		t.Errorf("id = %q, want the replayed comment", body.ID)
	}
}

// The multipart writer is the only place the part's Content-Type can be set, so
// a value the caller supplied has to survive into the wire format.
func TestUploadTaskAttachment_DeclaredTypeReachesTheWire(t *testing.T) {
	var gotType string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		gotType = part.Header.Get("Content-Type")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv)
	if _, err := client.UploadTaskAttachment(
		context.Background(), "/mission/tasks/task-1/attachments", "notes.bin", "text/markdown", []byte("# hi"), nil, "",
	); err != nil {
		t.Fatalf("UploadTaskAttachment: %v", err)
	}
	if gotType != "text/markdown" {
		t.Errorf("part Content-Type = %q, want text/markdown", gotType)
	}
}
