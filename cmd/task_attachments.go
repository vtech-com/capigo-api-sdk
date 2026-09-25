package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// taskAttachmentUploadPath builds the request path for uploading a file to a
// task. base comes from taskPath: the task addressed by id or by code.
func taskAttachmentUploadPath(base string) string {
	return base + "/attachments"
}

// taskAttachmentDownloadPath builds the request path for downloading a
// task-level attachment. base comes from taskPath: the task addressed by id or
// by code.
func taskAttachmentDownloadPath(base, attachmentID string) string {
	return base + "/attachments/" + url.PathEscape(attachmentID) + "/download"
}

// commentAttachmentDownloadPath builds the request path for downloading a
// comment/message-level attachment.
func commentAttachmentDownloadPath(base, attachmentID string) string {
	return base + "/comments/attachments/" + url.PathEscape(attachmentID) + "/download"
}

// runAttachmentDownload is the shared implementation behind both `tasks
// attachments download` and `tasks comments attachments download`: fetch the
// signed-URL metadata, download the bytes to disk, and report the result.
// path is the fully-built request path (see the two helpers above).
func runAttachmentDownload(client *api.Client, tenant *string, tenantFlag, path, dest string) error {
	ctx := context.Background()

	resp, err := client.Do(ctx, "GET", path, nil, tenant)
	if err != nil {
		return handleErr(err)
	}

	var meta api.AttachmentDownload
	if err := json.Unmarshal(resp.Body, &meta); err != nil {
		return handleErr(fmt.Errorf("decode response: %w", err))
	}

	destPath := api.ResolveDownloadDestPath(dest, meta.FileName)

	if err := api.DownloadToFile(ctx, meta.URL, destPath, meta.SizeBytes); err != nil {
		return handleErr(err)
	}

	data := map[string]any{
		"file_name":  meta.FileName,
		"mime_type":  meta.MimeType,
		"size_bytes": meta.SizeBytes,
		"saved_path": destPath,
	}
	return output.Write(os.Stdout, data, itemMeta(tenant, tenantFlag, nil))
}

// tasks attachments (group) + download flags
var (
	taskAttachmentsDownloadTenant string
	taskAttachmentsDownloadCode   string
	taskAttachmentsDownloadDest   string
)

var tasksAttachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Manage a task's own attachments",
	Long: `Files attached directly to a task.

Attachment metadata — id, file_name, mime_type, size_bytes — is listed by
tasks get, and carries no download URL, which is why this group exists:
upload sends a file, download mints the URL, remove deletes one.

USAGE
  capigo tasks attachments <command> [--tenant <code>] [<args>]`,
}

var tasksAttachmentsDownloadCmd = &cobra.Command{
	Use:   "download <task-id> <attachment-id>",
	Short: "Download a task-level attachment",
	Long: `Download a file attached to a task.

PURPOSE
  tasks get lists a task's attachments with their ids but no download URL.
  This command mints a fresh signed URL and writes the bytes to disk in one
  step.

USAGE
  capigo tasks attachments download <task-id> <attachment-id>
                                     [--tenant <code>] [-d <path>]

FLAGS
  <task-id>
      Task UUID. Positional, required.

  <attachment-id>
      Attachment UUID, from tasks get .attachments[].id. Positional,
      required.

        capigo tasks attachments download <task-uuid> <att-uuid>

  --tenant <code>
      Optional; scopes the lookup.

  -d, --dest <path>
      A file, or a directory. Defaults to the original file name in the
      current directory. An existing file at the resolved path is
      overwritten.

        capigo tasks attachments download <task-uuid> <att-uuid> -d ./dl

OUTPUT
  The file is written to the resolved destination path, unconditionally. The
  file metadata is at .data:

      {
        "data": { "file_name": "invoice.pdf", "mime_type": "application/pdf",
                  "size_bytes": 48213, "saved_path": "invoice.pdf" },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  The signed URL behind the download is short-lived (five minutes). The CLI
  never prints it and mints a fresh one on every call, so a URL-expired error
  is answered by running the command again.

  Exit 4 when no such task or attachment is reachable.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(_ *cobra.Command, args []string) error {
		taskID, attachmentID := splitAttachmentArgs(args, taskAttachmentsDownloadCode)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskAttachmentsDownloadTenant, profile)
		requireOneTaskAddress(taskID, taskAttachmentsDownloadCode, tenant)
		requireAttachmentID(attachmentID)

		path := taskAttachmentDownloadPath(taskPath(taskID, taskAttachmentsDownloadCode), attachmentID)
		return runAttachmentDownload(client, tenant, taskAttachmentsDownloadTenant, path, taskAttachmentsDownloadDest)
	},
}

// uploadResultData is what `tasks attachments upload` prints.
//
// It lives outside RunE so the one fact a retry turns on — whether the server
// stored the file or replayed one it already had — is testable without a server:
// an agent that retried must be able to tell those apart from stdout alone.
func uploadResultData(attachment api.AttachmentMetadata, sourcePath string, statusCode int) map[string]any {
	return map[string]any{
		"id":         attachment.ID,
		"file_name":  attachment.FileName,
		"mime_type":  attachment.MimeType,
		"size_bytes": attachment.SizeBytes,
		// A fact about this call the caller cannot read back: the path on this
		// machine the bytes came from.
		"source_path": sourcePath,
		// 200 means the server replayed an upload it already stored under this
		// key; 201 means this command is what stored it.
		"replayed": statusCode == http.StatusOK,
	}
}

// taskAttachmentRemovePath builds the request path for removing a file from a
// task. base comes from taskPath: the task addressed by id or by code. The
// attachment id is escaped because it is a value someone typed, not a literal.
func taskAttachmentRemovePath(base, attachmentID string) string {
	return base + "/attachments/" + url.PathEscape(attachmentID)
}

// removeResultData is what `tasks attachments remove` prints.
//
// The API answers with the file it removed, which is the only chance to see what
// it was: after this call the attachment is gone from every task read, so a
// caller who removed the wrong file has nothing left to compare against.
func removeResultData(attachment api.AttachmentMetadata) map[string]any {
	return map[string]any{
		"id":         attachment.ID,
		"file_name":  attachment.FileName,
		"mime_type":  attachment.MimeType,
		"size_bytes": attachment.SizeBytes,
		// The CLI's own claim, and the reason this command exists: the file is
		// no longer on the task. stdout has to say so itself — there is no
		// status line for an agent to read.
		"removed": true,
	}
}

// tasks attachments (group) + remove flags
var (
	taskAttachmentsRemoveTenant string
	taskAttachmentsRemoveCode   string
)

var tasksAttachmentsRemoveCmd = &cobra.Command{
	Use:   "remove <task-id> <attachment-id>",
	Short: "Remove a file from a task",
	Long: `Remove an attachment from a task and delete the stored file.

PURPOSE
  Retires a file that was attached to a task. The attachment leaves the task's
  list and the stored object is deleted in the same call, so tasks get no
  longer names it.

  This is the counterpart of upload, and it is final: nothing in this CLI
  restores a removed attachment. Removing a file a task does not hold exits 4
  rather than failing silently, which is also what a repeated remove answers —
  so a retry tells you the file is already gone.

USAGE
  capigo tasks attachments remove <task-id> <attachment-id>
                                     [--tenant <code>] [--code <code>]

FLAGS
  <task-id>
      Task UUID. Positional. Give this or --code, never both. With --code the
      only positional is the attachment id.

  <attachment-id>
      Attachment UUID, from tasks get .attachments[].id or from the upload
      output. Positional, required. Omitting it exits 5.

        capigo tasks attachments remove <task-uuid> <att-uuid>

  --code <code>
      Address the task by its code — the key a person quotes, like ACMEC-68.
      A code is unique within a tenant, so --code needs a tenant.

        capigo tasks attachments remove --code ACMEC-68 <att-uuid>

  --tenant <code>
      Optional with a task id; required with --code.

OUTPUT
  The file that was removed is at .data — the only chance to check it was the
  right one, because every task read stops naming it from here:

      {
        "data": { "id": "<att-uuid>", "file_name": "invoice.pdf",
                  "mime_type": "application/pdf", "size_bytes": 48213,
                  "removed": true },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  No --idempotency-key: a delete already answers 4 for a second attempt, so
  there is nothing for a key to dedupe.

  Exit 4 when the task is out of reach, or holds no attachment with that id.
  Exit 5 when the attachment id was not given.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(_ *cobra.Command, args []string) error {
		// The second positional is the attachment id here, as on a download;
		// with --code it is the only positional.
		taskID, attachmentID := splitAttachmentArgs(args, taskAttachmentsRemoveCode)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskAttachmentsRemoveTenant, profile)
		requireOneTaskAddress(taskID, taskAttachmentsRemoveCode, tenant)
		requireAttachmentID(attachmentID)

		resp, err := client.RemoveTaskAttachment(
			context.Background(),
			taskAttachmentRemovePath(taskPath(taskID, taskAttachmentsRemoveCode), attachmentID),
			tenant,
		)
		if err != nil {
			return handleErr(err)
		}

		var body api.TaskAttachmentEnvelope
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		return output.Write(os.Stdout, removeResultData(body.Data), itemMeta(tenant, taskAttachmentsRemoveTenant, nil))
	},
}

// tasks attachments (group) + upload flags
var (
	taskAttachmentsUploadTenant         string
	taskAttachmentsUploadCode           string
	taskAttachmentsUploadContentType    string
	taskAttachmentsUploadIdempotencyKey string
)

var tasksAttachmentsUploadCmd = &cobra.Command{
	Use:   "upload <task-id> <path>",
	Short: "Upload a file to a task",
	Long: `Upload a file and attach it to a task, in one request.

PURPOSE
  One call does the whole job: the file goes up, the server stores it, and
  the attachment is recorded on the task. There is no presigned URL to fetch
  and no second call to make it visible — which is what makes this usable
  from a script, an n8n node, or an agent.

USAGE
  capigo tasks attachments upload <task-id> <path>
                                     [--tenant <code>] [--code <code>]
                                     [--content-type <type>]
                                     [--idempotency-key <key>]

FLAGS
  <task-id>
      Task UUID. Positional. Give this or --code, never both. With --code the
      only positional is the file path.

  <path>
      The file to upload, as a path on this machine. Positional, required.
      Omitting it exits 5.

        capigo tasks attachments upload <task-uuid> ./invoice.pdf

  --code <code>
      Address the task by its code — the key a person quotes, like ACMEC-68.
      A code is unique within a tenant, so --code needs a tenant.

        capigo tasks attachments upload --code ACMEC-68 ./invoice.pdf

  --tenant <code>
      Optional with a task id; required with --code.

  --content-type <type>
      The media type to declare for the file. Detected from the extension,
      then from the file's first bytes, when omitted. The server checks it
      against a fixed list of accepted types (images, PDF, Office documents,
      text/markdown/csv, zip) and answers 400 INVALID_FILE_TYPE naming the
      type it saw, so a wrong guess is corrected by passing this flag — the
      CLI does not keep its own copy of that list. A value with no characters
      is refused here rather than sent: the server would reject the
      declaration after the upload had been spent.

  --idempotency-key <key>
      Optional. Makes a retry safe: sending the same key with the same file
      again answers with the attachment the first attempt stored and stores
      nothing a second time. Reusing the key for a different file (a
      different name, type, or size) fails with E0601. A key with no
      characters is refused here for the same reason the API ignores it: it
      would make the retry unsafe while looking safe.

OUTPUT
  The stored attachment is at .data, with the path uploaded from:

      {
        "data": { "id": "6b1f...", "file_name": "invoice.pdf",
                  "mime_type": "application/pdf", "size_bytes": 48213,
                  "source_path": "./invoice.pdf", "replayed": false },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  .data.id is what tasks attachments download takes. "replayed" is true when
  the server stored nothing because an earlier call with this
  Idempotency-Key already had — a retry that succeeded, not a second copy.

  Exit 4 when no such task is reachable (including another tenant's).
  Exit 5 when the file is empty, over 50 MB, unreadable, or its declared
  type is not accepted. Exit 8 when the Idempotency-Key was already used for
  a different file.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// The second positional is the attachment id on a download and the file
		// path here; the slot is the same, the meaning is the command's.
		taskID, uploadPath := splitAttachmentArgs(args, taskAttachmentsUploadCode)

		idempotencyKey := requireUsableKey(
			cmd.Flags().Changed("idempotency-key"),
			taskAttachmentsUploadIdempotencyKey,
			"idempotency-key",
		)
		contentTypeOverride := requireUsableMediaType(
			cmd.Flags().Changed("content-type"),
			taskAttachmentsUploadContentType,
			"content-type",
		)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskAttachmentsUploadTenant, profile)
		requireOneTaskAddress(taskID, taskAttachmentsUploadCode, tenant)
		requireUploadPath(uploadPath)

		data, err := api.ReadUploadFile(uploadPath)
		if err != nil {
			return handleErr(err)
		}

		contentType := contentTypeOverride
		if contentType == "" {
			contentType = api.DetectContentType(uploadPath, data)
		}

		resp, err := client.UploadTaskAttachment(
			context.Background(),
			taskAttachmentUploadPath(taskPath(taskID, taskAttachmentsUploadCode)),
			filepath.Base(uploadPath),
			contentType,
			data,
			tenant,
			idempotencyKey,
		)
		if err != nil {
			return handleErr(err)
		}

		var body api.TaskAttachmentEnvelope
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		out := uploadResultData(body.Data, uploadPath, resp.StatusCode)
		return output.Write(os.Stdout, out, itemMeta(tenant, taskAttachmentsUploadTenant, nil))
	},
}

// tasks comments attachments (group) + download flags
var (
	taskCommentsAttachmentsDownloadTenant string
	taskCommentsAttachmentsDownloadCode   string
	taskCommentsAttachmentsDownloadDest   string
)

var tasksCommentsAttachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Manage a task comment's attachments",
	Long: `Files posted on a task's timeline (comments and activity entries).

Attachment metadata is listed by tasks comments, on the entry that carries
it. No download URL is ever included there, which is why this group exists:
it mints one.

USAGE
  capigo tasks comments attachments <command> [--tenant <code>] [<args>]`,
}

var tasksCommentsAttachmentsDownloadCmd = &cobra.Command{
	Use:   "download <task-id> <attachment-id>",
	Short: "Download a task comment's attachment",
	Long: `Download a file posted on a comment or activity entry.

PURPOSE
  tasks comments lists each entry's attachments with their ids but no
  download URL. This command mints a fresh signed URL and writes the bytes to
  disk in one step.

USAGE
  capigo tasks comments attachments download <task-id> <attachment-id>
                                              [--tenant <code>] [-d <path>]

FLAGS
  <task-id>
      Task UUID. Positional. Give this or --code, never both. With --code the
      only positional is the attachment id. Establishes the tenant this
      download is scoped to (see --tenant below).

  <attachment-id>
      Attachment UUID, from tasks comments .data[].attachments[].id.
      Positional, required. Omitting it exits 5.

        capigo tasks comments attachments download <task-uuid> <att-uuid>

  --code <code>
      Address the task by its code — the key a person quotes, like ACMEC-68.
      A code is unique within a tenant, so --code needs a tenant. A bare
      argument is never guessed at.

        capigo tasks comments attachments download --code ACMEC-68 <att-uuid>

  --tenant <code>
      Optional with a task id; required with --code. This endpoint is scoped
      to the task's tenant, not to the task itself: the download succeeds for
      any attachment id that exists in that tenant, including one posted on a
      different task's thread.

  -d, --dest <path>
      A file, or a directory. Defaults to the original file name in the
      current directory. An existing file at the resolved path is
      overwritten.

        capigo tasks comments attachments download <task-uuid> <id> -d ./dl

OUTPUT
  The file is written to the resolved destination path, unconditionally. The
  file metadata is at .data:

      {
        "data": { "file_name": "invoice.pdf", "mime_type": "application/pdf",
                  "size_bytes": 48213, "saved_path": "invoice.pdf" },
        "meta": { "tenant": "acme", "tenant_source": "flag" }
      }

  The signed URL behind the download is short-lived (five minutes). The CLI
  never prints it and mints a fresh one on every call, so a URL-expired error
  is answered by running the command again.

  Exit 4 when no such task or attachment is reachable.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(_ *cobra.Command, args []string) error {
		taskID, attachmentID := splitAttachmentArgs(args, taskCommentsAttachmentsDownloadCode)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskCommentsAttachmentsDownloadTenant, profile)
		requireOneTaskAddress(taskID, taskCommentsAttachmentsDownloadCode, tenant)
		requireAttachmentID(attachmentID)

		path := commentAttachmentDownloadPath(taskPath(taskID, taskCommentsAttachmentsDownloadCode), attachmentID)
		return runAttachmentDownload(client, tenant, taskCommentsAttachmentsDownloadTenant, path, taskCommentsAttachmentsDownloadDest)
	},
}

func init() {
	tasksAttachmentsRemoveCmd.Flags().StringVar(&taskAttachmentsRemoveTenant, "tenant", "", "scope to this tenant code")
	tasksAttachmentsRemoveCmd.Flags().StringVar(&taskAttachmentsRemoveCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")
	tasksAttachmentsCmd.AddCommand(tasksAttachmentsRemoveCmd)

	tasksAttachmentsUploadCmd.Flags().StringVar(&taskAttachmentsUploadTenant, "tenant", "", "scope to this tenant code")
	tasksAttachmentsUploadCmd.Flags().StringVar(&taskAttachmentsUploadCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")
	tasksAttachmentsUploadCmd.Flags().StringVar(&taskAttachmentsUploadContentType, "content-type", "", "media type to declare for the file (default: detected from the path, then the bytes)")
	tasksAttachmentsUploadCmd.Flags().StringVar(&taskAttachmentsUploadIdempotencyKey, "idempotency-key", "", "make a retry safe: reuse this key to re-send the same file without storing it twice")
	tasksAttachmentsCmd.AddCommand(tasksAttachmentsUploadCmd)

	tasksAttachmentsDownloadCmd.Flags().StringVar(&taskAttachmentsDownloadTenant, "tenant", "", "scope to this tenant code")
	tasksAttachmentsDownloadCmd.Flags().StringVar(&taskAttachmentsDownloadCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")
	tasksAttachmentsDownloadCmd.Flags().StringVarP(&taskAttachmentsDownloadDest, "dest", "d", "", "destination file or directory (default: original file name in the current directory)")
	tasksAttachmentsCmd.AddCommand(tasksAttachmentsDownloadCmd)
	// tasksAttachmentsCmd is registered under `tasks` by tasks.go, not here.
	// Command sorting is off, so registration order is display order — and
	// init() runs in file-name order, which would put attachments above list.

	tasksCommentsAttachmentsDownloadCmd.Flags().StringVar(&taskCommentsAttachmentsDownloadTenant, "tenant", "", "scope to this tenant code")
	tasksCommentsAttachmentsDownloadCmd.Flags().StringVar(&taskCommentsAttachmentsDownloadCode, "code", "", "address the task by its code (e.g. ACMEC-68) instead of by id")
	tasksCommentsAttachmentsDownloadCmd.Flags().StringVarP(&taskCommentsAttachmentsDownloadDest, "dest", "d", "", "destination file or directory (default: original file name in the current directory)")
	tasksCommentsAttachmentsCmd.AddCommand(tasksCommentsAttachmentsDownloadCmd)
	tasksCommentsCmd.AddCommand(tasksCommentsAttachmentsCmd)
}
