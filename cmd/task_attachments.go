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
}

var tasksAttachmentsDownloadCmd = &cobra.Command{
	Use:   "download <task-id> <attachment-id>",
	Short: "Download a task-level attachment",
	Long: `Download a task's own attachment (as opposed to a comment attachment —
see 'tasks comments attachments download' for those) to a local file.

The attachment-id comes from 'tasks get <task-id>', field attachments[].id.

Fetches a fresh, short-lived (5 minute) signed URL and downloads the bytes
immediately — the URL is not printed or reusable across invocations. On
success prints the saved file path (and size/mime type in table/json mode).

Without --dest, the file is saved to the current directory using its
original file name. --dest can name a directory (file saved inside it,
original name kept) or an exact file path (its parent directory must
already exist). An existing file at the destination is overwritten.`,
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
}

var tasksCommentsAttachmentsDownloadCmd = &cobra.Command{
	Use:   "download <task-id> <attachment-id>",
	Short: "Download a task comment's attachment",
	Long: `Download an attachment posted on a task's comment/activity timeline
(as opposed to a task-level attachment — see 'tasks attachments download' for
those) to a local file.

The attachment-id comes from 'tasks comments <task-id>', field
attachments[].id on the relevant timeline entry.

Fetches a fresh, short-lived (5 minute) signed URL and downloads the bytes
immediately — the URL is not printed or reusable across invocations. On
success prints the saved file path (and size/mime type in table/json mode).

Without --dest, the file is saved to the current directory using its
original file name. --dest can name a directory (file saved inside it,
original name kept) or an exact file path (its parent directory must
already exist). An existing file at the destination is overwritten.

Note: this endpoint is scoped to the task's tenant, not to the specific task
or comment thread — any active member of that tenant can download any of the
tenant's comment attachments this way.`,
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
	taskCmd.AddCommand(tasksAttachmentsCmd)

	tasksCommentsAttachmentsDownloadCmd.Flags().StringVar(&taskCommentsAttachmentsDownloadTenant, "tenant", "", "scope to this tenant code")
	tasksCommentsAttachmentsDownloadCmd.Flags().StringVarP(&taskCommentsAttachmentsDownloadDest, "dest", "d", "", "destination file or directory (default: original file name in the current directory)")
	tasksCommentsAttachmentsCmd.AddCommand(tasksCommentsAttachmentsDownloadCmd)
	tasksCommentsCmd.AddCommand(tasksCommentsAttachmentsCmd)
}
