# Task domain relationships

Read this only when creating or changing tasks, assignees, followers, or subtasks. Read
`boards.md` as well when board placement is involved.

## Task record schema

The authoritative field reference for task records returned by the API.

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Machine identifier for API calls |
| `code` | string | Stable human reference, e.g. `VTECH-10064`. Unique within a tenant. |
| `title` | string | |
| `description` | string | |
| `status` | string | `Pending`, `To-Do`, `Doing`, `Done`, `Closed`, `Cancelled` |
| `priority` | string | `Low`, `Normal`, `High`, `Urgent` — capitalised. No `Medium`. |
| `assignee` | object \| null | `{id, display_name, member_code}` — nested object, **not** `assignee_id`/`assignee_name` |
| `owner` | object \| null | `{id, display_name, member_code}` |
| `board_id` | UUID \| null | Must travel with `board_list_id`; set both or neither |
| `board_list_id` | UUID \| null | **Not** `list_id` |
| `due_date` | string \| null | ISO-8601 date |
| `has_subtasks` | boolean | |
| `attachments` | array | Metadata only — `id`, `file_name`, `mime_type`, `size_bytes`. No download URL. |
| `followers` | array | `[{id, display_name, member_code}]` |
| `parent` | object \| null | Nested `{id, code, title}` on detail endpoints (`tasks get`) |
| `parent_task_id` | UUID \| null | Flat field on `tasks list` only |
| `meta_data` | object \| null | Extra metadata. Contains `url` — the browser-accessible link to the task. |
| `created_at` | string | ISO-8601 timestamp |
| `updated_at` | string | ISO-8601 timestamp |

### Comment / activity entry

| Field | Type | Notes |
|---|---|---|
| `kind` | string | `comment` or `activity` |
| `content` | string | The message body or rendered event text. There is no `body` or `description` field. |
| `author` | object | `{id, name, type}` |
| `created_at` | string | ISO-8601 timestamp |
| `attachments` | array | |
| `parent_id` | UUID \| null | Set on a reply |
| `ui_data` | object | |

Comments come from humans and bots; activity entries are system events (status
transitions, field changes, creation). The timeline is history, not state — the
authoritative current status lives on the task itself.

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

## Verify

After a write, check `error`, `meta.tenant`, the returned assignee, and any returned board/list.
Report the generated task code as the stable human reference.
