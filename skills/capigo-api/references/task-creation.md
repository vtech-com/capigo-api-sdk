# Creating Capigo tasks

Use this reference only when creating a task or subtask. First run the relevant leaf help, for
example:

```bash
capigo tasks create --help
capigo tasks subtasks create --help
```

The installed help is authoritative for flags, fields, and output. This guide records stable task
constraints; do not turn a team's workflow or naming conventions into default behavior.

## Assignment and board placement

Before creating the task, inspect the user's request:

- An explicit assignee must resolve to one exact, active member in the target tenant. Never guess a
  UUID or select a partial name. Use an AI-agent assignee only when the leaf help supports it.
- For "me" or no assignee, omit the assignee flag: Capigo applies its creator-assignment default.
  Do not spend a member lookup or ask a self-versus-unassigned question.
- An explicitly unassigned task needs a help-confirmed CLI path. Do not send an empty assignee or
  treat omission as unassigned.
- Omit `--board` and `--list` unless the user requests board placement. When they do, resolve the
  exact board and list in the target tenant and send both together. If the list is absent or
  ambiguous, ask; list position and names are workflow-specific, so never choose one by default.

## Subtasks

- A subtask belongs to a top-level parent; it cannot have children and is never placed on a board.
- Only the parent owner, parent assignee, or tenant owner can change that structure. Do not retry a
  permission failure as though it were transient.
- A parent's status is independent of its subtask progress. Do not infer or change it from the
  children.

## Verify the write

Check that stdout has no `error` key, confirm `meta.tenant`, and inspect the returned assignee and
board/list when supplied. Report the generated task code; it is the stable human reference.
