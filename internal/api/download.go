package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// downloadTimeout is the hard cap on how long a byte download may take. It is
// deliberately generous (well beyond the signed URL's own 5-minute TTL) since
// the URL is fetched immediately after being minted in the same command
// invocation — a stuck transfer, not an expired URL, is the failure this
// guards against.
const downloadTimeout = 2 * time.Minute

// downloadHTTPClient is a plain HTTP client with no Capigo authentication.
// The signed URL points at a different host (object storage) that neither
// expects nor should receive the Capigo API key.
var downloadHTTPClient = &http.Client{Timeout: downloadTimeout}

// DownloadToFile fetches signedURL and writes it to destPath, verifying the
// byte count against expectedSize when expectedSize > 0. It writes to a
// temporary file in the same directory as destPath and renames it into place
// on success, so a failed or interrupted download never leaves a partial or
// corrupt file at destPath.
func DownloadToFile(ctx context.Context, signedURL, destPath string, expectedSize int64) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}

	resp, err := downloadHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{
			Code:       "ATTACHMENT_URL_EXPIRED",
			Message:    fmt.Sprintf("storage rejected the download (HTTP %d)", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			RawBody:    body,
		}
	}

	dir := filepath.Dir(destPath)
	if _, statErr := os.Stat(dir); statErr != nil {
		return &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("destination directory %q does not exist", dir),
			HTTPStatus: 400,
		}
	}

	tmp, err := os.CreateTemp(dir, ".capigo-download-*")
	if err != nil {
		return &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("create temp file in %q: %v", dir, err),
			HTTPStatus: 400,
		}
	}
	tmpPath := tmp.Name()
	// Clean up the temp file on any path that returns before the rename below.
	succeeded := false
	defer func() {
		_ = tmp.Close()
		if !succeeded {
			_ = os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmp, resp.Body)
	if err != nil {
		return fmt.Errorf("write %q: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %q: %w", tmpPath, err)
	}

	if expectedSize > 0 && written != expectedSize {
		return fmt.Errorf("downloaded %d bytes, expected %d — the file may be truncated; retry the command", written, expectedSize)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("save to %q: %w", destPath, err)
	}
	succeeded = true

	return nil
}

// ResolveDownloadDestPath resolves the --dest flag into a concrete file path.
//   - "" → fileName in the current directory.
//   - an existing directory → fileName inside it.
//   - anything else → dest itself, verbatim (its parent directory must exist;
//     DownloadToFile rejects it otherwise — this function does not create
//     directories).
func ResolveDownloadDestPath(dest, fileName string) string {
	if dest == "" {
		return fileName
	}
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return filepath.Join(dest, fileName)
	}
	return dest
}
