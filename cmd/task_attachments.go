package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/config"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// taskAttachmentDownloadPath builds the request path for downloading a
// task-level attachment.
func taskAttachmentDownloadPath(taskID, attachmentID string) string {
	return "/mission/tasks/" + taskID + "/attachments/" + attachmentID + "/download"
}

// commentAttachmentDownloadPath builds the request path for downloading a
// comment/message-level attachment.
func commentAttachmentDownloadPath(taskID, attachmentID string) string {
	return "/mission/tasks/" + taskID + "/comments/attachments/" + attachmentID + "/download"
}

// runAttachmentDownload is the shared implementation behind both `tasks
// attachments download` and `tasks comments attachments download`: fetch the
// signed-URL metadata, download the bytes to disk, and report the result.
// path is the fully-built request path (see the two helpers above).
func runAttachmentDownload(client *api.Client, tenant *string, path, dest string) error {
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

	switch outputMode {
	case "json":
		return output.WriteJSONObject(os.Stdout, map[string]any{
			"file_name":  meta.FileName,
			"mime_type":  meta.MimeType,
			"size_bytes": meta.SizeBytes,
			"saved_path": destPath,
		})
	case "quiet":
		_, err := fmt.Fprintln(os.Stdout, destPath)
		return err
	default: // table
		_, err := fmt.Fprintf(os.Stdout, "Saved: %s (%d bytes, %s)\n", destPath, meta.SizeBytes, meta.MimeType)
		return err
	}
}

// tasks attachments (group) + download flags
var (
	taskAttachmentsDownloadTenant string
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
                                     [-o table|json|quiet]

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

  -o, --output table|json|quiet
      Controls what is printed after the file is written, not the file
      itself — every mode writes the same bytes to the same path. Defaults
      to table.
      See capigo help output.

OUTPUT
  The file is written to the resolved destination path.

  table (default):

      Saved: invoice.pdf (48213 bytes, application/pdf)

  -o json:

      { "file_name": "invoice.pdf", "mime_type": "application/pdf",
        "size_bytes": 48213, "saved_path": "invoice.pdf" }

  quiet prints the saved path alone:

      invoice.pdf

  The signed URL behind the download is short-lived (five minutes). The CLI
  never prints it and mints a fresh one on every call, so a URL-expired error
  is answered by running the command again.

  Exit 4 when no such task or attachment is reachable.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskAttachmentsDownloadTenant, profile)

		return runAttachmentDownload(client, tenant, taskAttachmentDownloadPath(args[0], args[1]), taskAttachmentsDownloadDest)
	},
}

// tasks comments attachments (group) + download flags
var (
	taskCommentsAttachmentsDownloadTenant string
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
                                              [-o table|json|quiet]

FLAGS
  <task-id>
      Task UUID. Positional, required. Establishes the tenant this download
      is scoped to (see --tenant below).

  <attachment-id>
      Attachment UUID, from tasks comments .data[].attachments[].id.
      Positional, required.

        capigo tasks comments attachments download <task-uuid> <att-uuid>

  --tenant <code>
      Optional; scopes the lookup. This endpoint is scoped to the task's
      tenant, not to the task itself: the download succeeds for any
      attachment id that exists in that tenant, including one posted on a
      different task's thread.

  -d, --dest <path>
      A file, or a directory. Defaults to the original file name in the
      current directory. An existing file at the resolved path is
      overwritten.

        capigo tasks comments attachments download <task-uuid> <id> -d ./dl

  -o, --output table|json|quiet
      Controls what is printed after the file is written, not the file
      itself — every mode writes the same bytes to the same path. Defaults
      to table.
      See capigo help output.

OUTPUT
  The file is written to the resolved destination path.

  table (default):

      Saved: invoice.pdf (48213 bytes, application/pdf)

  -o json:

      { "file_name": "invoice.pdf", "mime_type": "application/pdf",
        "size_bytes": 48213, "saved_path": "invoice.pdf" }

  quiet prints the saved path alone:

      invoice.pdf

  The signed URL behind the download is short-lived (five minutes). The CLI
  never prints it and mints a fresh one on every call, so a URL-expired error
  is answered by running the command again.

  Exit 4 when no such task or attachment is reachable.`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}
		profile, err := config.ActiveProfile(cfg)
		if err != nil {
			return handleErr(err)
		}
		tenant := resolveTenant(taskCommentsAttachmentsDownloadTenant, profile)

		return runAttachmentDownload(client, tenant, commentAttachmentDownloadPath(args[0], args[1]), taskCommentsAttachmentsDownloadDest)
	},
}

func init() {
	tasksAttachmentsDownloadCmd.Flags().StringVar(&taskAttachmentsDownloadTenant, "tenant", "", "scope to this tenant code")
	tasksAttachmentsDownloadCmd.Flags().StringVarP(&taskAttachmentsDownloadDest, "dest", "d", "", "destination file or directory (default: original file name in the current directory)")
	tasksAttachmentsCmd.AddCommand(tasksAttachmentsDownloadCmd)
	// tasksAttachmentsCmd is registered under `tasks` by tasks.go, not here.
	// Command sorting is off, so registration order is display order — and
	// init() runs in file-name order, which would put attachments above list.

	tasksCommentsAttachmentsDownloadCmd.Flags().StringVar(&taskCommentsAttachmentsDownloadTenant, "tenant", "", "scope to this tenant code")
	tasksCommentsAttachmentsDownloadCmd.Flags().StringVarP(&taskCommentsAttachmentsDownloadDest, "dest", "d", "", "destination file or directory (default: original file name in the current directory)")
	tasksCommentsAttachmentsCmd.AddCommand(tasksCommentsAttachmentsDownloadCmd)
	tasksCommentsCmd.AddCommand(tasksCommentsAttachmentsCmd)
}
