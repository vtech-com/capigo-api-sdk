package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDownloadDestPath(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		dest     string
		fileName string
		want     string
	}{
		{"empty dest uses file name as-is", "", "invoice.pdf", "invoice.pdf"},
		{"existing directory joins file name", dir, "invoice.pdf", filepath.Join(dir, "invoice.pdf")},
		{"explicit file path used verbatim", filepath.Join(dir, "renamed.pdf"), "invoice.pdf", filepath.Join(dir, "renamed.pdf")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveDownloadDestPath(tt.dest, tt.fileName)
			if got != tt.want {
				t.Errorf("ResolveDownloadDestPath(%q, %q) = %q, want %q", tt.dest, tt.fileName, got, tt.want)
			}
		})
	}
}

func TestDownloadToFile_HappyPath(t *testing.T) {
	const body = "hello, attachment"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")

	if err := DownloadToFile(context.Background(), srv.URL, dest, int64(len(body))); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != body {
		t.Errorf("dest content = %q, want %q", got, body)
	}

	// No leftover temp files in the destination directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (leftover temp file?): %v", len(entries), entries)
	}
}

func TestDownloadToFile_SkipsSizeCheckWhenZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("some bytes"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.txt")
	if err := DownloadToFile(context.Background(), srv.URL, dest, 0); err != nil {
		t.Fatalf("DownloadToFile with expectedSize=0: %v", err)
	}
}

func TestDownloadToFile_StorageRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("expired"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")

	err := DownloadToFile(context.Background(), srv.URL, dest, 0)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "ATTACHMENT_URL_EXPIRED" {
		t.Errorf("code = %q, want ATTACHMENT_URL_EXPIRED", apiErr.Code)
	}

	// Destination must not exist — no partial/empty file left behind.
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("dest file should not exist after a rejected download")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("dir should be empty after a rejected download, got: %v", entries)
	}
}

func TestDownloadToFile_SizeMismatchCleansUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")

	err := DownloadToFile(context.Background(), srv.URL, dest, 99999)
	if err == nil {
		t.Fatal("expected size-mismatch error")
	}
	if !strings.Contains(err.Error(), "downloaded") {
		t.Errorf("error = %v, want a size-mismatch message", err)
	}

	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("dest file should not exist after a size mismatch")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("dir should be empty after a size mismatch, got: %v", entries)
	}
}

func TestDownloadToFile_MissingDestDir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "does-not-exist", "out.txt")

	err := DownloadToFile(context.Background(), srv.URL, dest, 0)
	if err == nil {
		t.Fatal("expected error for missing destination directory")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", apiErr.Code)
	}
}

func TestDownloadToFile_OverwritesExistingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(dest, []byte("old content"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	if err := DownloadToFile(context.Background(), srv.URL, dest, 0); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != "new content" {
		t.Errorf("dest content = %q, want overwritten content", got)
	}
}
