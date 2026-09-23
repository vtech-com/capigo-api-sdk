# Task domain relationships

Read this only when creating or changing tasks, assignees, followers, or subtasks. Read
`boards.md` as well when board placement is involved.

## Create flow

1. Read the relevant `tasks create` or `tasks subtasks create` help and resolve the tenant.
2. Creating a task makes the caller its owner. A task has at most one assignee; followers receive
   updates but do not share responsibility.
3. Resolve an explicit assignee to one exact, active member in the target tenant. Never guess a UUID
   or select a partial name. Use an AI-agent assignee only when leaf help supports it.
4. For "me" or no assignee, omit the assignee flag: Capigo applies its creator-assignment default.
   An explicitly unassigned task needs a help-confirmed CLI path; never send an empty assignee.
5. If placing the task on a board, resolve its exact board and list through `boards.md`; otherwise
   omit both fields.

## Comments

- `tasks comments create` posts a comment; `tasks comments` (no `create`) only reads the timeline.
  Confirm which one a request needs before acting.
- Attachments are referenced by pre-uploaded id, not uploaded by this CLI. Only pass
  `--attachments-json` when the caller already has attachment ids from another channel (the web
  app); otherwise post `--content` alone.
- Addressing works the same as everywhere else in this domain: an id, or `--code` plus `--tenant`.

## Subtasks

- A top-level task can have subtasks. Each subtask has exactly one top-level parent, cannot have
  children, and is not placed on a board.
- Only the parent owner, parent assignee, or tenant owner can change subtask structure. A permission
  failure is final for that operation; do not retry it as a transient error.
- Parent status is independent of subtask progress. Do not infer or change it from child states.

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
