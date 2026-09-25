# Task domain relationships

Read this only when creating or changing tasks, assignees, followers, subtasks, or a task's
attachments. Read `boards.md` as well when board placement is involved.

## Create flow

1. Read the relevant `tasks create` or `tasks subtasks create` help and resolve the tenant.
2. Creating a task makes the caller its owner. A task has at most one assignee; followers receive
   updates but do not share responsibility.
3. Resolve an explicit assignee to one exact, active member in the target tenant. Never guess a UUID
   or select a partial name. Use an AI-agent assignee only when leaf help supports it.
4. For "me" or no assignee, omit the assignee flag: Capigo applies its creator-assignment default.
   An explicitly unassigned task needs a help-confirmed CLI path; never send an empty assignee.
5. If placing the task on a board, resolve its exact board and list through `boards.md`; otherwise
   omit both fields. Naming a list also asks for membership of that board (or a tenant owner): a
   create onto a board that the caller is not on is refused with `BOARD_FORBIDDEN` (403), and that
   refusal is final for that caller — do not retry it as a transient error. A wrong tenant is a
   different code (`AUTH_TENANT_MISMATCH`), so the two refusals do not mean the same thing.
6. `--top`, or `--after-task-id <uuid>` naming a card already in that column, decides where the new
   card lands; omit both and it is appended. Both flags need `--list`, are mutually exclusive, and
   are refused with `--subtasks-json` (exit 5).
7. A create that names a list can answer `PLACEMENT_FAILED` (409) with the created task in `data.id`:
   the card exists at the end of that list and only the position was refused. Do not create it again —
   place it with `tasks move`.

## Attachments

- `tasks attachments upload <task-id|--code> <path>` attaches a file in one call: the CLI sends
  the bytes, the server stores them and records the attachment. There is no presigned URL to
  fetch and no second call — it is the way to put a file on a task from here.
- Read `.data.replayed` before believing an upload stored anything. `false` means this call
  stored the file; `true` means the server returned an attachment an earlier call with the same
  `--idempotency-key` had already stored. Retrying an upload without a key can store the file
  twice, so pass `--idempotency-key` when a retry is possible — an agent's second attempt is
  exactly that case.
- The media type is detected from the path, then from the file's first bytes. When the detection
  is wrong the server refuses with `INVALID_FILE_TYPE` and names the type it saw: pass
  `--content-type` with the right one. The accepted types and the 50 MB ceiling are the API's
  and the web UI's own.
- `.data.id` is what `tasks attachments download` takes next.
- `tasks attachments remove <task-id|--code> <attachment-id>` retires a file: it leaves the task's
  list and the stored object is deleted, in the same call. There is no undo anywhere in this CLI,
  so resolve the exact attachment id first — never remove by file name or by position in a list.
- `.data.removed` is `true` on a successful removal, and `.data` names the file that was removed:
  the only chance to check it was the right one, since every task read stops listing it from there.
- A removal takes no `--idempotency-key`. Removing an attachment the task no longer holds exits 4
  (`Attachment not found`), which is also the answer a repeated remove gets — read that as "already
  gone", not as a transient failure to retry. Exit 4 is likewise what an unreachable task answers,
  so it never reveals whether someone else's task exists.
- Attachments on *comments* travel with the comment: `tasks comments create --file <path>` (repeatable,
  at most 10 — an eleventh exits 5 before anything is uploaded) sends the files in the same request
  and the server stores and records them. It is the way to put a file on a comment from here — no
  upload step first. `--content-type <media type>` declares one for every `--file` instead of letting
  the detection decide, which is what a file whose format this machine cannot name needs (a `.md` on a
  host with no MIME database, or bytes that look like another type).
- `--idempotency-key` needs `--file`. With it, re-sending the same key with the same comment does
  not post a second one: the answer is the comment the first attempt wrote, and `meta.replayed` is
  `true`. Passing the key without `--file` exits 5, because the API takes a key only on a comment
  that carries its files — a body of pre-uploaded ids has no key, and silently ignoring it would
  make a retry look safe when it is not. A key holding only whitespace exits 5 as well: the API
  trims the header and reads the blank as no key at all, so a retry under it would post a second
  comment while the flag said it would not.
- `--file` and `--attachments-json` cannot be combined (exit 5): a comment carries either files or
  pre-uploaded ids. `--attachments-json` is for a caller that already holds attachment ids from
  somewhere else.

## Comments

- `tasks comments create` posts a comment; `tasks comments` (no `create`) only reads the timeline.
  Confirm which one a request needs before acting.
- Comment files are sent with `--file` (see Attachments above); `--attachments-json` is only for an
  existing attachment id from another channel (the web app). Otherwise post `--content` alone.
- Addressing works the same as everywhere else in this domain: an id, or `--code` plus `--tenant`.

## Subtasks

- A top-level task can have subtasks. Each subtask has exactly one top-level parent, cannot have
  children, and is not placed on a board.
- Only the parent owner, parent assignee, or tenant owner can change subtask structure. A permission
  failure is final for that operation; do not retry it as a transient error.
- Parent status is independent of subtask progress. Do not infer or change it from child states.
- `tasks subtasks move (<parent-id> | --code <code>) <subtask-id> (--top | --after-subtask-id <uuid>)`
  reorders a subtask among its siblings. A subtask lives in no board column, so it never moves between
  lists — moving a top-level card between columns is `tasks move`.
- Address the parent the subtask actually belongs to. Quoting another parent's subtask exits 4, and that
  is final: it is not the same subtask under a second address, and retrying will not find it.
- The position is computed by the server under a lock, so a retry is safe. Never compute or send a
  number yourself.
- `tasks subtasks delete (<parent-id> | --code <code>) <subtask-id>` retires one subtask: the parent
  task and the sibling subtasks keep their state. Name the parent the subtask actually belongs to —
  any other parent exits 4, and that is final. The parent's owner, its assignee, or a tenant owner may
  call it; a subtask's own assignee is refused. A repeat exits 4, so never follow it with a `tasks get`
  to confirm.

## Followers

- `--follower-id <uuid>` adds a follower (repeatable, idempotent). `--remove-follower-id <uuid>`
  removes one (repeatable; removing a non-follower is a no-op).
- Never pass the same user to both flags: the API answers 400 rather than picking an order.
- Adding someone else as a follower needs more permission than following yourself: the tenant owner,
  the task owner, or the assignee. A 403 here is final for that user, not a transient failure.

## Retrying a create

- `tasks create --idempotency-key <key>` exists for exactly one job: retrying a create you are not
  sure completed. Reuse the key **and** the same body; the API replays the task it already created
  (200 instead of 201) instead of making a second one.
- Do not generate a new key to "try again with a change" — a changed body under the same key is
  409 E0601. A changed task is a `tasks update`, not a second create.
- `--subtasks-json` has no idempotency contract; the CLI refuses the combination (exit 5).

## Agents

- `tasks assign-agent (<id> | --code) --agent-key <key>` moves a task an AI agent owns to another
  agent. It is the agent picker on the task screen, not an assignment tool for people: a task
  assigned to a person has no agent run to move and is refused with `INVALID_AGENT`. For a person
  use `tasks update --assignee`, for an owner use the transfer path.
- The move only works while the agent run is still `pending`. An agent that already started cannot
  be swapped — the refusal is final for that task; do not retry it as a transient error.
- The target agent must be published in the task's tenant (a system agent is accepted). Never invent
  an agent key: list the agents the tenant has, or ask the user which one.
- Every task response carries the assignment: `responsible_type` is `agent` or `human`, and
  `assigned_agent_key` names the agent (null for a person). Read the pair back after a change instead
  of assuming it landed.
- Naming the agent the task already has is a successful no-op, so retrying the same call is safe.

## Claiming a task

- `tasks claim (<id> | --code)` takes an unassigned task for yourself. The call sends no body and
  takes no flag that names a user: the assignee is always the key's user, so this cannot assign a
  task to somebody else. Any active member of the task's tenant may claim — no owner or manager role
  is needed.
- Claim only when the task is free. A task that already has an assignee is refused (409
  `TASK_ALREADY_ASSIGNED`, exit 8), and that is the same answer whether another member took it or you
  already hold it. Do not read the refusal as "retry later"; re-read with `tasks get` to see who
  holds it.
- This is not `tasks update --assignee`: that call reassigns work, while claim only takes work nobody
  owns yet. If the person is already the assignee, there is nothing to claim.
- Claiming changes the assignee alone. Owner, status, followers and board placement stay as they
  were, so never claim a task in order to move it between boards or lists.
- Self-claiming is one of the few writes a plain member can make: it needs membership, not the
  permission to assign.

## Archiving a task

- `tasks archive (<id> | --code)` retires a task. The call sends no body, and the task leaves every default
  read — restore it with `tasks unarchive`, or read it back deliberately with `--include-archived`.
- Permission is narrow: the task's owner, its assignee, or a tenant owner of the task's tenant. A plain
  member is refused (403, exit 3), and so is a member of the task's board. A subtask is stricter — it asks
  the parent's owner or assignee, so a subtask's own assignee alone cannot retire it.
- Archiving is a family operation: naming a parent archives its subtasks with it, and naming a subtask
  archives its parent and siblings. Only the task you named records the `task:archived` event.
- Do not retry after exit 4 ("not found"): the task was archived by your own previous call, and an archived
  task answers 4 on every read that does not ask for it. Add `--include-archived` before concluding
  anything, and see "Finding an archived task" below.
- `tasks delete (<id> | --code)` is the same write under the verb callers reach for: a soft delete that
  leaves the task in the archive. Deleting a top-level task retires its active subtasks with it. The
  answer names the id, and a repeat exits 4 — never follow a delete with a `tasks get` to confirm it,
  because that read is designed to answer 4. Retiring one subtask alone is `tasks subtasks delete`.

## Restoring a task

- `tasks unarchive (<id> | --code)` brings an archived task back. The call sends no body, and the answer is
  the task as it now stands — a restored task is readable again.
- The same three actors as archive may call it: the task's owner, its assignee, or a tenant owner of the
  task's tenant. Anyone else exits 3 (403).
- Restoring does not ask the parent, so a subtask's own assignee may restore their subtask — archiving that
  same subtask would have been refused. Naming any member of an archived family restores the whole family,
  and an archived list holding the task comes back with it.
- A task that is already live is a no-op: nothing is written, no event is recorded, and the answer is the
  task.

## Moving a task on a board

- `tasks move (<id> | --code) --board-list-id <uuid> (--top | --after-task-id <uuid>)` places a card the
  way a drag does: first in the column, or directly behind a card already there.
- The destination column is required, and exactly one placement: `--top`, or `--after-task-id` naming a
  card in that same column. Missing the list, missing both placements, or giving both exits 5 and nothing
  moves. Naming the task itself as the anchor is refused too — the API answers 400 before it moves
  anything, so a 5 there does not mean the move was attempted.
- Only the task's owner, its assignee, or a tenant owner of its tenant may move it. Anyone else exits 4 —
  the same answer a task that does not exist gets, so never report "the task does not exist" off an exit 4
  without checking the caller's relationship to the task.
- Moving is how work is filed onto a board from the CLI: a task with no board placement gains one, and a
  task already on a board changes column. Send the move and read the returned `board_id`, `board_list_id`
  and `position` back rather than assuming the card landed where you pictured it.
- An `--after-task-id` from another column exits 4 and leaves the task where it was. If the anchor was
  archived between your read and the call, the task lands first in the destination column instead — the
  board's own graceful fallback — so a move that succeeds may still not be exactly where you asked.
- A subtask cannot be moved: exit 5 with `SUBTASK_BOARD_FORBIDDEN`, because subtasks live in no column.
- Retry freely. The server computes the position from the card it finds, so a second identical call leaves
  the card where the first one put it and no idempotency key is needed.

## Finding an archived task

- Archived tasks are left out of every read by default. A task you know exists can therefore be missing
  from `tasks list`, and `tasks get` exits 4 for it — that is usually this, not a permission problem.
- Ask for them explicitly: `tasks list --include-archived` lists archived tasks alongside live ones, and
  `tasks get --include-archived` reads one. Both send the API's `include_archived=true` on that one call.
- The code a person quotes still finds the task:
  `tasks get --code ACMEC-68 --tenant acme --include-archived`.
- Without the flag, exit 4 is the same answer a task that never existed gets, so never report "the task
  does not exist" off an unflagged 404 — add the flag first.
- Archiving a parent archives its subtasks with it, so a family goes missing from the list together and
  comes back together. A missing task and a missing subtask are one event, not two.
- The flag widens reads only. `tasks update` on an archived task still exits 4, and a second `tasks archive`
  still exits 4 — restore first with `tasks unarchive`, then change it.

## Ownership

- `tasks transfer-ownership (<id> | --code) --owner-id <uuid>` gives a task a different owner. Two
  actors may call it: the task's **current owner**, and a **tenant owner of the task's tenant**
  (ADR-061). Everyone else is refused with 403, including a tenant owner of another tenant — so this
  is not a general admin override, and not a way to claim a task.
- `--owner-id` takes a user UUID, not a name or a member code: resolve the person with
  `members list`/`members get` first. A target who is not an active member of the task's tenant is a
  400 — do not retry it; fix the id.
- Transferring changes the owner alone. Assignee, followers, board placement and status are
  untouched, so a transfer is never the right way to hand work to someone who should merely do it —
  that is `tasks update --assignee`.
- Naming the current owner is a no-op that still answers 200, and the task comes back in the
  response: read `.data.owner` to confirm the new owner. The new owner and the former owner each get
  an inbox message; the activity entry says when a tenant owner transferred a task they did not own.

## Verify

After a write, check `error`, `meta.tenant`, the returned assignee, and any returned board/list.
Report the generated task code as the stable human reference.
