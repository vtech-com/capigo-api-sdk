# Board domain relationships

Read this only when creating or moving a task on a board. Leaf help remains authoritative for the
available board and task commands.

## Task placement

1. A board owns ordered lists. A top-level task is either not on a board, or references one board
   and one list belonging to that board; send the two IDs together.
2. Resolve the exact board and list in the target tenant. If either name is ambiguous, show the
   smallest useful set of candidates and ask the user to choose.
3. If the user names a board but no list, ask for the list. Do not choose by list position or name;
   the workflow belongs to that tenant.
4. Do not infer a status from a list name or translate a team's workflow into a status transition.
   Send an initial status only when the user explicitly requests it and leaf help supports it.

## Permissions and verification

- Board membership governs board interactions. Let the CLI report missing permission; do not retry
  a permission failure as transient.
- After placement, compare the returned board and list IDs with the resolved pair and report the
  chosen list by name.
