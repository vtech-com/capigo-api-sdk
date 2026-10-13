package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// tasks promote flags
var (
	taskPromoteTenant string
	taskPromoteCode   string
	taskPromoteList   string
)

var tasksPromoteCmd = &cobra.Command{
	Use:   "promote [<id>]",
	Short: "Turn a subtask into a task of its own",
	Long: `Promote a subtask to a top-level task.

PURPOSE
  Detach a subtask from its parent and make it a task in its own right. It is
  the same row: it keeps its code, status, assignee, due date, followers,
  comments, files and history, and only stops being a child. The address is
  the subtask's own id or code, not its parent's.

  The caller must be the subtask's owner, its assignee, or an owner of the
  tenant. A caller who may not promote it gets exit 4, the same answer a
  missing task gets. An archived or deleted subtask is refused with exit 8 and
  code SUBTASK_NOT_PROMOTABLE.

  A task that is already top-level is answered as it stands, with exit 0 and
  nothing changed — so a retry is safe, and no Idempotency-Key is needed or
  accepted. That answer ignores --list: see OUTPUT.

USAGE
  capigo tasks promote (<id> | --code <code>) [--list <uuid>] [--tenant <code>]

FLAGS
  <id>
      UUID of the subtask. Positional. Give this or --code, never both.

  --code <code>
      Address the subtask by its code — the key a person quotes, like
      ACMEC-68. A code is unique within a tenant, not across them, so --code
      needs a tenant: pass --tenant, or set a default.

        capigo tasks promote --code ACMEC-68 --tenant acme

  --list <uuid>
      Board list to place the promoted task in. Its board is read from the
      list, the task goes to the end of it, and the task's owner and assignee
      become members of that board. Left out, the task is promoted to no
      board. The list must be a live list of a live board of the tenant: a
      deleted or archived one exits 5 with code DESTINATION_UNAVAILABLE.

        capigo tasks promote --code ACMEC-68 --tenant acme \
            --list 9ab2c744-16fe-4d09-8a52-3ef0b7c61d84

  --tenant <code>
      Tenant to scope the lookup to. Optional with an id; required with
      --code.

OUTPUT
  The task as it now stands is at .data, in the same shape as tasks get. Read
  parent: null is what says it is top-level now.

      { "data": { "id": "...", "code": "ACMEC-68", "title": "...",
                  "parent": null, "board_id": "...",
                  "board_list_id": "9ab2c744-16fe-4d09-8a52-3ef0b7c61d84", ... },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-13T04:12:33Z" } }

  meta.list_applied is false when --list was given and the task is not in that
  list. It means the task was already top-level: the call changed nothing, and
  the list you named was ignored. To place such a task, use tasks move.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()

		var id string
		if len(args) == 1 {
			id = args[0]
		}
		list := strings.TrimSpace(taskPromoteList)

		client, cfg, err := buildClient()
		if err != nil {
			return handleErr(err)
		}

		tenant := resolveTenant(taskPromoteTenant, activeProfileOrEmpty(cfg))
		requireOneTaskAddress(id, taskPromoteCode, tenant)

		// No body promotes to no board; the API reads an absent list that way.
		var body any
		if list != "" {
			body = map[string]string{"board_list_id": list}
		}

		resp, err := client.Do(ctx, "POST", taskPath(id, taskPromoteCode)+"/actions/promote", body, tenant)
		if err != nil {
			return handleErr(err)
		}

		var envelope api.RawEnvelope
		if err := json.Unmarshal(resp.Body, &envelope); err != nil {
			return handleErr(fmt.Errorf("decode response: %w", err))
		}

		meta := itemMeta(tenant, taskPromoteTenant, envelope.Meta)
		meta.ServerTime = resp.ServerTime
		if list != "" && !api.TaskIsInList(envelope.Data, list) {
			// A task that was already top-level answers 200 unchanged and
			// ignores the list. Nothing in the answer says "ignored", so a
			// caller who named a list would report a placement that never
			// happened.
			if meta.Extra == nil {
				meta.Extra = map[string]any{}
			}
			meta.Extra["list_applied"] = false
		}
		return output.Write(os.Stdout, rawItem(envelope.Data), meta)
	},
}

func init() {
	tasksPromoteCmd.Flags().StringVar(&taskPromoteTenant, "tenant", "", "scope to this tenant code")
	tasksPromoteCmd.Flags().StringVar(&taskPromoteCode, "code", "", "address the subtask by its code (e.g. ACMEC-68) instead of by id")
	tasksPromoteCmd.Flags().StringVar(&taskPromoteList, "list", "", "board list to place the promoted task in")
}
