# Creating Capigo tasks: assignee and board placement

Use this reference only when creating a task or subtask. First run the relevant leaf help, for
example:

```bash
capigo tasks create --help
capigo tasks subtasks create --help
```

The installed help is authoritative for flags, fields, and output. This guide covers the human
judgment help cannot make: whether an otherwise unspecified task should be self-assigned, and
which board and list a task lands on.

## Pause when the assignee is not explicit

Before creating the task, inspect the user's request:

- If it clearly names the assignee, resolve that exact member safely and use the documented flag.
- If it explicitly says "me" or otherwise requests self-assignment, the assignment choice is
  already clear; resolve the user's exact member identity as described below.
- If it explicitly says to leave the task unassigned, omit the assignee as documented by help.
- If the assignee is omitted or otherwise unclear, pause before the write and ask:
  **"Do you want this assigned to you, or left unassigned?"**

Do not assume that omission means self-assignment. Do not silently leave it unassigned either; the
choice affects ownership and notification behavior.

## Resolve self without inferring identity

If the user chooses self-assignment, do not infer their Capigo identity from the operating-system
account, Git author, tenant owner, first member returned, API key, or local configuration. The CLI's
`auth whoami` command is not a reliable identity lookup because its API route is not implemented.

1. Read `capigo members list --help` and keep the lookup scoped to the task's tenant.
2. If the conversation does not already contain an exact work email or other unique Capigo member
   identifier, ask the user for one. An unambiguous email is safer than a display name.
3. Search with the flags shown by live help. Accept only one exact matching member in the intended
   tenant; do not choose a fuzzy or partial match.
4. If no exact match or multiple plausible matches remain, show only the minimum useful candidate
   details and ask the user to choose. Do not create the task yet.
5. Use the selected member UUID with the assignee flag documented by `tasks create --help`.

If the user chooses unassigned, omit the assignee flag. Never send an empty string, guessed UUID,
or member from a cross-tenant search.

## Placing a task on a board

`--board` and `--list` on `tasks create` are always sent together — putting a task on a board
means choosing a list, not just a board. Never guess either UUID.

1. Resolve the board first: search `capigo boards list -q <name>`, scoped to the task's tenant,
   then run `capigo boards get <id>` to read its `.data.lists[]`.
2. If the user named a specific list, match it exactly against those list names.
3. If the user did not specify a list, do not pause to ask — unlike the assignee gate above, list
   placement is cheap to correct afterward. Pick the first list by `position`, or a more fitting
   one when the list names make the choice obvious (a "Backlog" or "To-Do" list for new work).
4. After creating the task, report which list it landed in and enumerate the board's other list
   names, so the user can see the options at a glance. Mention that moving it later is
   `tasks update <id> --board <id> --list <id>` (see `tasks update --help`; both flags travel
   together there too).

Report the list choice and alternatives every time you pick one yourself: the user was not asked,
so they need enough information in the result to correct it.

## Verify the write

After creation, check that stdout has no `error` key, confirm `meta.tenant`, and inspect the returned
task's assignee field. Report whether the task is assigned to the resolved member or unassigned.
When the task was placed on a board, also confirm the returned task's board and list against what
was intended, alongside the list-choice report described above.
