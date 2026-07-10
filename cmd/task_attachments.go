package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

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
tasks get. No download URL is ever included there, which is why this group
exists: it mints one.

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
