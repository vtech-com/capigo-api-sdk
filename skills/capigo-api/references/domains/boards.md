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

## Deleting a board

- `boards delete` retires the board and the address of everything under it, but it **deletes no work**.
  The board's lists and the tasks in them stay live rows: the lists become unreachable, because a list
  is addressed inside its board, while the tasks stay readable through `tasks get` and `tasks list`.
  Never describe this call as clearing the board.
- Retiring the work is **archiving**, not deleting: `boards lists update --is-archived` archives a list
  and the tasks in it. When the user says "clean up" or "close" a board, ask which they mean before
  choosing — the two calls are not interchangeable.
- Nothing brings a deleted board back. There is no un-delete in the CLI or the API, so confirm the
  board with the user before running it, and never delete a board as a step towards something else.
- A repeat is exit 4, and so is a private board: the visibility check runs before the write, so a board
  the API cannot read is a board it cannot delete. A script must not treat that 4 as a transient
  failure.
- Confirm by reading: after the call, `boards list` no longer shows the board and `boards get` answers
  404. The write also records a `board:deleted` event, which integrators may be watching.

## Deleting a list

- `boards lists delete` is the destructive one, and it is **not** the archive. Archiving (`boards lists
  update --is-archived`) hides the list and archives the tasks in it; deleting retires the list and
  leaves the tasks exactly where they are — active, still filed under a list no read reports. The
  difference decides which one the user meant: archiving is the reversible-feeling way to clear a
  column, deleting is not reversible at all.
- Nothing brings a deleted list back. There is no un-delete in the CLI or the API, so confirm the list
  with the user before running it, and never delete a list as a step towards something else.
- A task in a deleted list is not orphaned by any write here and not archived either: it keeps its
  `board_list_id`, which now names a row no read returns. If the user's intent was to retire the work,
  archive the list first (which archives the tasks) rather than deleting it.
- A repeat is exit 4, and so is a list that belongs to another board — the address names both the board
  and the list, and a mismatch is answered as a missing list. A script must not treat that 4 as a
  transient failure: the list is gone, and asking again will not change it.
- The write needs board ownership or tenant ownership (exit 3 otherwise). Confirm it by reading: after
  the call, `boards get` no longer lists it and `meta.list_count` has dropped.

## Reordering lists

- A list moves **relative to another list**, never to a number: `--after-list-id` puts it directly
  behind that list, `--before-list-id` puts it directly in front. There is no position to send and no
  way to ask for "first" or "last" directly — name the sibling it should sit against.
- One anchor only, and never with a field. `--from-json` alongside an anchor is refused too: the file
  is the whole body, so the flag would be dropped and the list would not move.
- **A list already where it is asked to go is a success.** The server computes the position, and the
  same position as before is a valid outcome of a correct request. Never report it as a failure, and
  never retry on the assumption that nothing happened.
- The anchor must be a list on the **same board** that still appears in `boards get`. An anchor on
  another board, an archived anchor, or the list's own id is refused — do not place a list relative to
  something the user cannot see, and re-read the board after any archive before reordering.
- The new position is the server's, and `.data.position` is where the list now sits. Confirm a move by
  reading the board's list order rather than by trusting the request.

## Permissions and verification

- Board membership governs reading a board. Writing to one — the board itself or its lists — is
  narrower: it takes board ownership or tenant ownership, so a member who can read a board may
  still be refused on a write. Do not promise a write on the strength of being able to read.
- Let the CLI report missing permission; do not retry a permission failure as transient.
- After placement, compare the returned board and list IDs with the resolved pair and report the
  chosen list by name.

## Board membership

- A board's members are a **subset** of the workspace's members: being in the workspace does not put
  anyone on a board, and board membership does not change the workspace role. The two id spaces do not
  interchange — `boards members` takes the `user_id` that `boards members list` reports, not the
  member id `members list` reports. Resolve a person to a board-member `user_id` through
  `boards members list`, never by assuming the workspace id will do.
- `boards members add` is a batch, and it **skips** what is already there rather than refusing: an
  entry comes back `skipped` with reason `already_member`, and `added_count` is what changed the board.
  Exit 0 with `added_count: 0` therefore means the board was already as asked — never report it as a
  failure, and never re-send on the assumption that nothing happened.
- One role per request. A call that needs a member added and another promoted is two calls, and
  changing the role of someone already on the board is `boards members update` — `add` will not
  promote an existing member, it will skip them and leave their role alone.
- A board always keeps one active owner. Removing or demoting the last one is refused (exit 8), so a
  plain "take everyone off this board" is not expressible: name the owner who stays, or promote a
  second owner first.
- `boards members remove` succeeds silently (the API answers 204). Exit 0 is the whole answer; if the
  user needs proof, `boards members list` is the read that shows the member gone.
- Reading a board's members takes board membership (any role) or tenant ownership; the writes take
  board ownership or tenant ownership. A write to a private board is unreachable, exactly as
  **A board write's response cannot tell you what changed** describes above.
