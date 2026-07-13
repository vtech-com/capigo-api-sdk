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

## Subtasks

- A top-level task can have subtasks. Each subtask has exactly one top-level parent, cannot have
  children, and is not placed on a board.
- Only the parent owner, parent assignee, or tenant owner can change subtask structure. A permission
  failure is final for that operation; do not retry it as a transient error.
- Parent status is independent of subtask progress. Do not infer or change it from child states.

## Verify

After a write, check `error`, `meta.tenant`, the returned assignee, and any returned board/list.
Report the generated task code as the stable human reference.
