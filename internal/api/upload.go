package api

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

// MaxTaskAttachmentBytes mirrors the API's own ceiling for a task attachment:
// 50 MB, the same limit the web UI enforces (MAX_TASK_ATTACHMENT_BYTES in the
// platform's shared constants). The server checks it again — it is repeated here
// so a 60 MB file is refused locally instead of being uploaded first.
const MaxTaskAttachmentBytes = 50 * 1024 * 1024

// ReadUploadFile reads the file to upload, refusing one the API cannot accept.
//
// The whole file is read into memory on purpose: files are capped at
// MaxTaskAttachmentBytes, and a known length is what lets the request carry a
// Content-Length the server can refuse an oversized body by — a streamed body
// of unknown length cannot be turned away before it is read.
func ReadUploadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("cannot read %q: %v", path, err),
			HTTPStatus: 400,
		}
	}
	if info.IsDir() {
		return nil, &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("%q is a directory; give the file to upload", path),
			HTTPStatus: 400,
		}
	}
	if info.Size() == 0 {
		return nil, &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("%q is empty; the API requires a file larger than 0 bytes", path),
			HTTPStatus: 400,
		}
	}
	if info.Size() > MaxTaskAttachmentBytes {
		return nil, &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("%q is %d bytes; the API accepts at most %d bytes", path, info.Size(), MaxTaskAttachmentBytes),
			HTTPStatus: 400,
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &APIError{
			Code:       "VALIDATION_ERROR",
			Message:    fmt.Sprintf("cannot read %q: %v", path, err),
			HTTPStatus: 400,
		}
	}
	return data, nil
}

// DetectContentType guesses the media type to declare for an upload.
//
// The API reads the MIME type from the part's Content-Type and checks it against
// a fixed allow-list, so the CLI has to send something specific and cannot leave
// the type to the server. The file's extension is tried first — a .pdf is a PDF
// whatever its first bytes look like — then the first 512 bytes by
// http.DetectContentType, then application/octet-stream.
//
// A wrong guess is not hidden: an octet-stream, or any type the allow-list does
// not carry, comes back as the server's own INVALID_FILE_TYPE naming the type it
// saw, and --content-type is how the caller corrects it.
func DetectContentType(path string, data []byte) string {
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); byExt != "" {
		return stripMediaTypeParameters(byExt)
	}
	return stripMediaTypeParameters(http.DetectContentType(data))
}

// stripMediaTypeParameters drops a media type's parameters (`; charset=utf-8`).
// The API's allow-list carries bare types, and `text/plain; charset=utf-8` is
// not `text/plain` to a set lookup — so the parameters have to go, not the type.
func stripMediaTypeParameters(mediaType string) string {
	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		return strings.TrimSpace(mediaType[:i])
	}
	return strings.TrimSpace(mediaType)
}

// quoteFileName escapes a file name for a Content-Disposition parameter, the way
// mime/multipart does internally — CreatePart does not do it for us.
func quoteFileName(name string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(name)
}

// UploadField is a text field in a multipart request.
type UploadField struct {
	Name  string
	Value string
}

// UploadPart is one file to send in a multipart/form-data request.
type UploadPart struct {
	// FieldName is the form field the server reads the file from (`file` on
	// every endpoint here).
	FieldName string
	// FileName is the name the server stores, and the one it echoes back.
	FileName string
	// ContentType is what the server checks against its allow-list: the part's
	// own Content-Type is the only place it reads one from.
	ContentType string
	Data        []byte
}

// multipartBody builds a multipart/form-data body and its Content-Type header.
//
// The body is a known length, which is what lets the request carry a
// Content-Length the server can refuse an oversized body by — a streamed body
// of unknown length cannot be turned away before it is read.
func multipartBody(
	fields []UploadField,
	files []UploadPart,
) (*bytes.Buffer, string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	for _, field := range fields {
		if err := mw.WriteField(field.Name, field.Value); err != nil {
			return nil, "", fmt.Errorf("build multipart body: %w", err)
		}
	}

	for _, file := range files {
		header := textproto.MIMEHeader{}
		header.Set(
			"Content-Disposition",
			fmt.Sprintf(
				`form-data; name="%s"; filename="%s"`,
				file.FieldName,
				quoteFileName(file.FileName),
			),
		)
		header.Set("Content-Type", file.ContentType)

		part, err := mw.CreatePart(header)
		if err != nil {
			return nil, "", fmt.Errorf("build multipart body: %w", err)
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, "", fmt.Errorf("build multipart body: %w", err)
		}
	}

	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("build multipart body: %w", err)
	}

	return &body, mw.FormDataContentType(), nil
}

// postMultipart sends a prepared multipart body to path.
func (c *Client) postMultipart(
	ctx context.Context,
	path string,
	fields []UploadField,
	files []UploadPart,
	tenant *string,
	headers map[string]string,
) (*Response, error) {
	body, contentType, err := multipartBody(fields, files)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	return c.send(req, headers, tenant)
}

// UploadTaskAttachment posts the file bytes to path as multipart/form-data, in a
// single part named `file`.
//
// The whole job happens server-side: the API stores the bytes and records the
// metadata in this one request, so there is no presigned URL to orchestrate
// here — which is the point of the endpoint, and the reason this method takes
// bytes rather than a URL to hand back to the caller.
//
// idempotencyKey, when non-empty, is sent as the Idempotency-Key header: the
// server answers 200 instead of 201 when the same key arrives with the same file
// (nothing is stored a second time) and 409 E0601 when it arrives with a
// different one. contentType is what the server checks against its allow-list.
func (c *Client) UploadTaskAttachment(
	ctx context.Context,
	path string,
	fileName string,
	contentType string,
	data []byte,
	tenant *string,
	idempotencyKey string,
) (*Response, error) {
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}

	return c.postMultipart(ctx, path, nil, []UploadPart{{
		FieldName:   "file",
		FileName:    fileName,
		ContentType: contentType,
		Data:        data,
	}}, tenant, headers)
}

// PostTaskCommentWithAttachments posts a comment and its files in one request:
// the text as a `content` field, each file as its own `file` part.
//
// The API stores each file, records the attachment, and writes the comment with
// them — the same work the web UI drives from a browser, with the file transfer
// moved to the server. Answer 201 when this call posted the comment, 200 when
// the Idempotency-Key replayed one an earlier attempt posted, 409 E0601 when
// the key meets a different comment.
func (c *Client) PostTaskCommentWithAttachments(
	ctx context.Context,
	path string,
	content string,
	files []UploadPart,
	tenant *string,
	idempotencyKey string,
) (*Response, error) {
	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}

	fields := []UploadField{{Name: "content", Value: content}}

	return c.postMultipart(ctx, path, fields, files, tenant, headers)
}
