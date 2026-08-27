# Board domain relationships

Read this only when creating or moving a task on a board, or when creating or changing a board or
its lists. Leaf help remains authoritative for the available board and task commands.

## Task placement

1. A board owns ordered lists. A top-level task is either not on a board, or references one board
   and one list belonging to that board; send the two IDs together.
2. Resolve the exact board and list in the target tenant. If either name is ambiguous, show the
   smallest useful set of candidates and ask the user to choose.
3. If the user names a board but no list, ask for the list. Do not choose by list position or name;
   the workflow belongs to that tenant.
4. Do not infer a status from a list name or translate a team's workflow into a status transition.
   Send an initial status only when the user explicitly requests it and leaf help supports it.

## Board and list writes

- A board owns its lists. A list can be created with the board in one call, or added to an existing
  board afterwards; both routes reach the same structure. Choose one — do not create a board and
  then re-send the same list.
- A list's WIP limit is a **planning cap**, not an enforced one: it states how many tasks the list is
  meant to hold at once. The server does not refuse a task that exceeds it. Never treat it as
  pagination, and never report a list as "full".
- **A board write's response cannot tell you what changed.** No read returns a list's `limit` or
  `is_archived`, so setting either answers with a record identical to the one before the call.
  Exit 0 means the request was accepted, nothing more. Report these as requested, not as verified,
  and never re-send a write because the response "looked unchanged" — it always does.
- **Archiving a list removes it from every read.** `boards get` then omits it from `.lists` and
  stops counting it in `meta.list_count`, and nothing exposes archived lists. The update command
  still reaches the list, so **keep the list id** — unarchiving needs an id no read will give back.
  The tasks are neither moved nor deleted: they keep pointing at a list no read reports. Move them
  first if the user's intent is to clear the column, and say which ones were left behind if not.
- **A private board is unreachable, one way.** With `is_public` false, reading the board, updating
  it, and writing its lists all fail with 404 "Board not found" — the same answer as a board that
  does not exist. The visibility check runs before the update, so the flag cannot be switched back,
  and no endpoint deletes a board. People still see it in the web app; you do not. Never make a
  board private as a step towards something else, and when a board you just wrote to answers 404,
  consider that it went private before concluding it is gone.
- Tenant is required on every board write, and the tenant a write lands in comes from `--tenant` —
  it **overrides** any `tenant_code` inside a `--from-json` payload. A file carrying a different
  tenant is silently ignored, so never rely on the file to choose the workspace.
- Board writes are not a way to change task placement. Moving a task between lists is a task
  update, per **Task placement** above.

## Permissions and verification

- Board membership governs reading a board. Writing to one — the board itself or its lists — is
  narrower: it takes board ownership or tenant ownership, so a member who can read a board may
  still be refused on a write. Do not promise a write on the strength of being able to read.
- Let the CLI report missing permission; do not retry a permission failure as transient.
- After placement, compare the returned board and list IDs with the resolved pair and report the
  chosen list by name.
