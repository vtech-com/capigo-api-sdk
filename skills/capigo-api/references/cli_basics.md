# `capigo` CLI — flag-level reference

Per-command flags, examples, and mechanics for the `capigo` command-line tool. This is the
detail layer `../SKILL.md` points into — read `SKILL.md` first for the operating loop,
capability map, tenant philosophy, output-mode doctrine, and self-diagnosis; come here for the
exact flags of a specific command.

> If a command's real `--help` output disagrees with this file, the binary wins. See
> [Exit codes and self-diagnosis](#exit-codes-and-self-diagnosis) at the end.

## Contents

- [Call shape](#call-shape)
- [Setup](#setup)
  - [Authentication](#authentication) · [Config](#config) ·
    [Environment variables](#environment-variables)
- [Output modes and global flags](#output-modes-and-global-flags)
- [Tenant scoping (per-command facts)](#tenant-scoping-per-command-facts)
- [Command reference](#command-reference)
  - [Products](#products) · [Variants](#variants) · [Tasks](#tasks)
    ([creating subtasks](#creating-subtasks-tasks-subtasks-tasks-create---subtasks-json) ·
    [reading comments/timeline](#reading-a-tasks-discussion--history-tasks-comments) ·
    [downloading attachments](#downloading-an-attachment)) ·
  - [Boards](#boards) · [Members](#members) ·
  - [Reference data: brands / categories / product-types / units](#reference-data) ·
  - [Tenants](#tenants) · [Auth](#auth) · [Config commands](#config-commands)
- [Deep mechanics](#deep-mechanics)
  - [Pagination](#pagination) · [Passing JSON input (`--from-json`)](#passing-json-input---from-json) ·
    [Finding records: code vs UUID](#finding-records-code-vs-uuid) ·
    [Pre-staged commands](#pre-staged-commands)
- [Exit codes and self-diagnosis](#exit-codes-and-self-diagnosis)

## Call shape

```
capigo [global flags] <group> <command> [args] [flags]
```

## Setup

### Authentication

The CLI authenticates with an API key that starts with `csk_`. It is read, in order of
precedence, from the `CAPIGO_API_KEY` env var, then `~/.capigo/config.json`.

```bash
capigo auth login --key csk_xxx     # store the key in the active profile
capigo auth whoami                  # confirm who you are (hits /me)
capigo auth logout                  # clear the stored key
capigo health                       # preflight: API reachable + key accepted (exit 0 = ok)
```

`capigo health` is the preflight to run before a batch of work — exit 0 means the API is
reachable and the key is accepted; a non-zero exit (e.g. 2) tells you why before you run real
work. It prints `{"data":{"ok":bool,"timestamp":string},"meta":{}}`. It is not tenant-scoped.

**`auth whoami` (GET `/me`) is not a reliable preflight** — it can 404 on a deployment where
that endpoint isn't live yet. Prefer `capigo health`. If any command returns **exit code 2**,
the key is missing/invalid/expired — ask the user to log in again rather than retrying.

### Config

Credentials and settings live in `~/.capigo/config.json` (file mode `600`). There is exactly
one active profile (`active_profile`); the CLI does not take a `--profile` flag.

```bash
capigo config set <key> <value>            # set a config value
capigo config get <key>                    # read a config value
capigo config set-default-tenant <code>    # so you can omit --tenant
capigo config unset-default-tenant         # clear it
```

Config shape:

```json
{
  "version": 1,
  "profiles": {
    "default": {
      "api_key": "csk_…",
      "api_url": "https://platform.capigo.app",
      "default_tenant": "acme",
      "known_tenants": ["acme", "globex"]
    }
  },
  "active_profile": "default"
}
```

### Environment variables

The CLI binds three env vars (useful for CI / agents that inject secrets without writing a
config file):

| Var | Effect |
|---|---|
| `CAPIGO_API_KEY` | API key (`csk_…`) |
| `CAPIGO_TENANT` | Default tenant code for the call |
| `CAPIGO_API_URL` | Override API base URL |

```bash
CAPIGO_API_KEY=csk_… CAPIGO_TENANT=acme capigo products list
```

## Output and global flags

There is no output flag and no output modes. Every command that succeeds prints exactly one
shape to stdout:

```json
{ "data": …, "meta": { … } }
```

| Global flag | Meaning |
|---|---|
| `--api-url <url>` | Override the API base URL (staging / local dev). |
| `-v, --verbose` | Print the HTTP request/response (Authorization header redacted). |

`data` is an array for a `list` command, an object for a single item (`get`, `create`,
`update`, `replace`). The CLI does not unwrap the API's own `{"data": …}` envelope — `.data.id`
is correct where `.id` used to be. `meta` carries pagination for a list — see
[Pagination](#pagination); **one call is not the whole result set**. So:
`… products list | jq '.data[]'` but `… products get <id> | jq '.data.name'`.

Redirecting (`>`) or piping (`|`) stdout is always safe: it is JSON and nothing else is ever
written to it, so there is no prefix line to strip.

`products list` also reports the server timestamp as `meta.server_time`; feed it back as
`--updated-since` for incremental delta sync.

A failing command prints `{"error": {...}}` on stdout — still JSON — plus a one-line summary on
stderr; see [Exit codes and self-diagnosis](#exit-codes-and-self-diagnosis).

## Tenant scoping (per-command facts)

`--tenant <code>` is a **per-command** flag (not global). Resolution order: `--tenant` flag →
`CAPIGO_TENANT` env → `default_tenant` in config. For the *why* behind the echo lines and the
ask-the-user flow, see `SKILL.md` → Tenant handling. The facts that matter when picking flags:

- **Every PCMS command requires a tenant** — `products`, `variants`, `brands`, `categories`,
  `product-types`, `units`, for *every* verb (list, get, create, update, replace, variants).
  With no tenant resolved, the CLI rejects the call (exit 5).
- **Mission reads may span tenants** — `tasks list/get` and `boards list/get` work without
  `--tenant` and then return rows across every tenant you can access; each record names its own
  tenant, and `meta.tenant` names none — there was no single tenant to name. `tasks create` and
  `tasks subtasks` require a tenant.
- `members list/get` also accept an optional `--tenant`; omitting it resolves across all
  accessible tenants.

List the tenants you can reach:

```bash
capigo tenants list
```

## Command reference

Flags below are the notable ones; run `capigo <group> <command> --help` for the complete,
authoritative list from your binary.

### Products

Tenant **required** on all subcommands.

| Command | Key flags |
|---|---|
| `products list` | `--tenant` (req), `--query/-q` (2–500 chars; matches name, aliases, tags, variant name, SKU, barcode), `--updated-since` (ISO 8601 delta sync), `--ids` (comma UUIDs, max 50; mutually exclusive with `--all`), `--all` (auto-paginate), `--page`, `--limit` (1–100, default 20) |
| `products get <id>` | `--tenant` (req) — full single product (variants, options, brand, category, type, unit). UUID-addressed only. |
| `products create` | `--tenant` (req); simple mode: `--name` (req), `--sku`, `--barcode`, `--price`, `--status` (DRAFT/ACTIVE/ARCHIVED), `--currency`, `--description`, `--brand-id`, `--category-id`, `--product-type-id`, `--unit-id`, `--aliases` (repeatable), `--tags` (repeatable); or `--from-json -` for options+variants |
| `products update <id>` | `--tenant` (req); any of `--name`, `--description`, `--status`, `--currency`, `--brand-id`, `--category-id`, `--product-type-id`, `--unit-id`, `--aliases` (repeatable), `--tags` (repeatable); or `--from-json -` (mutually exclusive with field flags). At least one field required. This is products' **one** write verb for updates — there is no `products replace`. |
| `products variants` | `--tenant` (req), `--product-id` (req), `--from-json -` (req) — a **JSON array** of variant objects |

Key facts callers depend on:

- **`aliases[]` and `tags[]` are two separate string-array fields on a product.** Aliases are
  alternative names / product codes (codes conventionally live here); tags are free-form labels
  for organization and filtering. Both are settable on `create` and `update` via `--aliases` /
  `--tags` (each repeatable), or inside `--from-json` as `"aliases": [...]` / `"tags": [...]`.
  Both are matched by `--query` (see search note below).
- **`products variants` takes a JSON array**, not an object. An item **with** `variant_id` is
  updated; **without** `variant_id` it is created (and `name` is required). One call upserts
  many variants at once — this is the **sole** variant write path (see [Variants](#variants)).
- The variant `sku` field carries the variant's code; the variant `barcode` field carries the
  numeric barcode.
- A variant item also accepts `manufacturer_code`, `legacy_code`, and `extra_data` (arbitrary
  key-value metadata) — pass them inside `--from-json` like any other field. `variants get`,
  `products get`, and `products variants` echo them back on the object.
- **Soft-deleted products still appear in list results.** Check the `is_deleted` field — the
  `status` field alone does NOT reveal deletion. Never report a soft-deleted product as
  available, and never update or upsert variants onto one unless the user explicitly asks to
  restore it.
- The product object carries `aliases[]` and `tags[]`, so alias/Product-Code and tag checks
  don't need a second call — but completeness still requires paging (`--all`).
- **`--ids` reports what it could NOT find** in `meta.missing_ids`. Asking for 5 UUIDs and
  getting 3 rows back is still exit 0, but the missing IDs are named there. Treat a missing ID
  as "deleted or wrong-tenant", not as something to silently skip in your answer.
- **`--all` auto-paginates and streams every row.** If it fails mid-pagination (rate limit,
  network), the rows already fetched are still printed, `meta.complete` is `false`, and the
  command exits non-zero. Check `complete` before treating an `--all` result as the whole
  catalogue.

```bash
# Create a simple product with a Product Code alias
echo '{"name":"Pin iPhone 13 Pro Max","brand_id":"<uuid>","product_type_id":"<uuid>",
       "unit_id":"<uuid>","status":"DRAFT","aliases":["AP-BA-13PM"]}' \
  | capigo --tenant acme products create --from-json -

# Upsert variants on that product
echo '[{"name":"Đen","sku":"AP-BA-13PM-B","barcode":"63400700011"}]' \
  | capigo --tenant acme products variants --product-id <uuid> --from-json -
```

### Variants

`variants list` — **tenant required**. Lists PCMS variants filtered by barcode prefix; its
main job is finding the highest barcode under a prefix (e.g. for an allocation scheme that
auto-increments within a namespace). This group is **read-only** — every variant write goes
through `products variants` (above); there is no `variants update`/`create`/`replace`.

| Flag | Notes |
|---|---|
| `--tenant` | required |
| `--barcode-prefix` | filter variants whose barcode starts with this value |
| `--sort` | `barcode` (asc) or `-barcode` (desc); default `-barcode` |
| `--page`, `--limit` | pagination (limit default 20) |

```bash
capigo --tenant acme variants list --barcode-prefix 634007 --sort -barcode --limit 1
```

`variants get <id>` — **tenant required** — fetches one variant's full detail (sku, barcode,
price, options, type, timestamps). UUID-addressed only; orphaned/soft-deleted/cross-tenant
variants return 404.

### Tasks

Tenant is **optional** for reads, **required** for `create` and `subtasks`.

| Command | Key flags |
|---|---|
| `tasks list` | `--tenant`, `--query/-q`, `--status`, `--priority`, `--assignee-id`, `--owner-id`, `--board-id`, `--board-list-id`, `--due-after`/`--due-before` (ISO 8601 date), `--created-after`/`--created-before` (ISO 8601 timestamp), `--parent-task-id` (use `null` for top-level only), `--page`, `--limit` |
| `tasks get <id>` | `--tenant` |
| `tasks comments <id>` | `--tenant` (optional); `--type comment\|activity` (default both), `--sort asc\|desc` (default `desc` = newest first), `--page`, `--limit` (max 50). UUID-addressed only. |
| `tasks attachments download <task-id> <attachment-id>` | `--tenant` (optional); `--dest/-d` (file or directory; default: original file name in the current directory). Downloads a **task-level** attachment. |
| `tasks comments attachments download <task-id> <attachment-id>` | Same flags as above. Downloads an attachment posted on a **comment/activity** entry. |
| `tasks create` | `--title` (required), `--tenant` (required), `--description`, `--priority`, `--status`, `--due-date` (RFC3339), `--assignee` (user id), `--board` (id), `--list` (board list id), `--follower-id` (repeatable), `--subtasks-json` (array of subtask items → creates task + subtasks atomically) |
| `tasks update <id>` | `--tenant` (optional); any of `--title`, `--description` (empty string clears), `--status`, `--assignee` (UUID; `--assignee ""` unassigns), `--board` + `--list` (sent together; `--board "" --list ""` removes from board), `--follower-id` (repeatable, additive — removal not supported). At least one flag required. UUID-addressed only. |
| `tasks subtasks <parent-id>` | `--tenant` (required); single subtask via `--title` (+ `--description`, `--assignee`, `--due-date` `YYYY-MM-DD`, `--priority`, `--status`), or a batch via `--from-json -` (array of subtask items). Max 25 per request. |

```bash
capigo tasks list --status To-Do
capigo tasks create --tenant acme --title "Fix login bug" --priority high
```

#### Creating subtasks (`tasks subtasks`, `tasks create --subtasks-json`)

A task can have subtasks (child tasks). Two ways to create them, both **all-or-nothing**
(if any item is invalid, nothing is created; max 25 subtasks per request):

- **Under an existing parent** → `tasks subtasks <parent-id>`. One subtask via `--title`, or a
  batch via `--from-json -` (a JSON **array** of subtask items).
- **A new parent plus its subtasks in one atomic call** → `tasks create … --subtasks-json <file>`.
  The parent is built from the normal `tasks create` flags; `--subtasks-json` is the subtasks array.

A subtask item is `{title (required), description?, assignee_id?, due_date? (YYYY-MM-DD),
priority? (Low/Normal/High/Urgent), status? (Pending/To-Do/Doing/Done/Closed/Cancelled)}`.
Note `due_date` here is a calendar **date** (`YYYY-MM-DD`), unlike `tasks create --due-date`
which is an RFC3339 datetime.

Both endpoints are still **pre-staged** — see [Pre-staged commands](#pre-staged-commands).

```bash
# Add two subtasks under an existing task
echo '[{"title":"Design"},{"title":"Build","priority":"High"}]' \
  | capigo tasks subtasks <parent-uuid> --tenant acme --from-json -

# Create a parent task and its subtasks atomically
echo '[{"title":"Subtask A"},{"title":"Subtask B"}]' \
  | capigo tasks create --tenant acme --title "Epic X" --subtasks-json -
```

#### Reading a task's discussion + history (`tasks comments`)

Use this when you need a task's current state — what people said and how the work
progressed — before summarizing it or deciding what follow-up task to create. It returns the
task's timeline: human comments interleaved with system activity, oldest-or-newest first.

Read `.data[]` from the response. Each entry tells you what it is via `kind`:

- `kind: "comment"` — a message a person or agent typed; the real discussion lives in
  `content`.
- `kind: "activity"` — a system event (status change, (re)assignment, title/description/
  due-date edit, task creation). `content` is a ready-made sentence
  (`"Trâm changed status from Doing to Done"`) and `ui_data` holds the structured before/after.

Other fields per entry: `author {id, name, type}` (who did it), `attachments[]`
(`id`/`file_name`/`mime_type`/`size_bytes` — no download URL; see below), `parent_id`,
`created_at`.

```bash
# Whole timeline, newest first
capigo tasks comments <task-uuid> | jq '.data[] | {created_at, kind, author: .author.name, content}'
# Only the human discussion (skip status/assignment noise)
capigo tasks comments <task-uuid> --type comment
```

Two things to keep honest:

- **`author.name` may be `"System"`.** That's a graceful fallback when the original
  actor can't be resolved (e.g. a removed member) — not an error. Don't block on it.
- **For the *current* status, trust the task, not the latest activity message.** Activity
  events are written asynchronously and can lag by a moment, so the authoritative state is the
  task itself (`tasks get`). Treat `tasks comments` as the history/narrative, not the source of
  truth for the live status/assignee.

#### Downloading an attachment

Both `tasks get` and `tasks comments` report attachment metadata (`id`, `file_name`,
`mime_type`, `size_bytes`) but **never a download URL** — fetch one on demand with a
dedicated command, right before you need the file, not ahead of time:

```bash
# Task-level attachment (from `tasks get <id>`, field .attachments[].id)
capigo tasks attachments download <task-uuid> <attachment-uuid>

# Comment/activity attachment (from `tasks comments <id>`, field .attachments[].id
# on the relevant timeline entry)
capigo tasks comments attachments download <task-uuid> <attachment-uuid>

# Choose where it lands
capigo tasks attachments download <task-uuid> <attachment-uuid> --dest ./downloads/
```

These commands are **live** as of v0.19 (confirmed on prod since 2026-07-03) — no pre-staged
caveat applies. Both fetch a signed URL and download the bytes to disk in the same call —
there is no separate "get the URL" step and the CLI never prints the raw URL. Reasons that
matter for how you use this:

- **The URL is single-use and short-lived (5 minutes).** Do not try to extract or reuse a
  URL from a previous invocation, and do not queue this command to run "later" — run it right
  when you need the file.
- **Without `--dest`, the file lands in the current directory** under its original name;
  an existing file at the destination is overwritten.
- **A rejected/expired-URL error is `ATTACHMENT_URL_EXPIRED`** (self-diagnosing — the
  block tells you to just re-run the command; it mints a fresh URL every time).
- **Comment attachments are tenant-scoped, not task-scoped** — the download will succeed
  for any attachment in the caller's tenant, even one from a different task's thread than the
  `<task-uuid>` you passed. That `<task-uuid>` only establishes tenant context; it is not a
  membership check on the specific attachment.

### Boards

Tenant **optional** for reads.

| Command | Key flags |
|---|---|
| `boards list` | `--tenant`, `--query/-q` (case-insensitive name search), `--page`, `--limit` (default 20) |
| `boards get <id>` | `--tenant` — returns the board with its `lists` array |

### Members

Workspace members. Tenant **optional** for reads; omitting it resolves across all accessible
tenants.

| Command | Key flags |
|---|---|
| `members list` | `--tenant`, `--query/-q` (name/email search), `--page`, `--limit` |
| `members get <id>` | `--tenant` — 404 for an inaccessible or cross-tenant member. UUID-addressed only. |

### Reference data

`brands`, `categories`, `product-types`, and `units` share the same CRUD shape. **Tenant
required** on every verb.

| Verb | HTTP | Notes |
|---|---|---|
| `list` | GET | `--query/-q` name-contains search (max 200 chars) |
| `get <id>` | GET | fetch one by UUID |
| `create` | POST | see per-resource required flags below |
| `update <id>` | PATCH | partial update — only provided fields change |
| `replace <id>` | PUT | full replace — all fields required |

Per-resource flags:

- **brands** — `create --name` (req), `--logo-url`; `update` adds `--clear-logo`;
  `replace --name` (req) plus one of `--logo-url` / `--no-logo`.
- **categories** — `create --name` (req), `--parent-id`; `update` adds `--clear-parent`;
  `replace --name` (req) plus one of `--parent-id` / `--root`.
- **product-types** — `create --name` (req), `--description`; `update` adds
  `--clear-description`; `replace --name` (req) plus one of `--description` /
  `--no-description`.
- **units** — `create --name --abbreviation` (both req); `update` either; `replace` both req.

```bash
capigo --tenant acme brands list --query nike
capigo --tenant acme product-types create --name "Pin Liền Cáp"
echo '{"name":"Pin Liền Cáp"}' | capigo --tenant acme product-types create --from-json -
```

### Tenants

`tenants list` — lists tenants the user can access; takes no `--tenant`. Discovered tenant
codes are merged into `known_tenants` in config.

### Auth

| Command | Flags | Notes |
|---|---|---|
| `auth login` | `--key csk_…` (required) | Stores the key in the active profile. |
| `auth whoami` | | Shows the authenticated user (GET `/me`). Not a reliable preflight — see [Authentication](#authentication). |
| `auth logout` | | Clears the stored key. |

### Config commands

`config set <key> <value>` · `config get <key>` · `config set-default-tenant <code>` ·
`config unset-default-tenant`.

## Deep mechanics

### Pagination

**Every `list` command is paginated. A single call returns at most one page — by default 20
rows (`--limit`, max 100), *not* the whole collection.** This is the single most common way an
agent goes wrong: it runs `products list`, gets 20 rows, and concludes "that's everything" —
so a record on page 2 looks like it doesn't exist, and a uniqueness/duplicate check passes
when it shouldn't.

Read the truth from the `meta` object every `list` returns:

```json
{ "data": [ … ], "meta": { "page": 1, "limit": 20, "total": 137, "has_more": true } }
```

- **`has_more`** — `true` means there are more pages. This is your signal to keep going.
- **`total`** — the full count across all pages, regardless of the current page size.
- **`page` / `limit`** — where you are and how many per page.

How to get a complete result set:

- **To *find* a specific product (by name / alias / Product-Code / SKU / barcode), reach for
  `--query` FIRST.** It searches those fields server-side (`ILIKE` substring; see the lookup
  section below) and a specific value usually lands on a single page — far cheaper than pulling
  the whole catalogue. Only fall back to `--all` + local filtering when a substring `--query`
  can't match the stored form (e.g. shortened aliases — see the caveat below) or you genuinely
  need every row.
- **`products list` has `--all`** — it auto-paginates internally and streams every row. Use it
  when you truly need the **whole catalogue** (a full export, or an exhaustive scan `--query`
  can't express): `capigo --tenant acme products list --all | jq '.data[]'`.
  **If `--all` fails mid-pagination** (rate limit, network), the rows already fetched are
  still printed, `meta.complete` is `false`, and the command exits non-zero. **Check
  `complete` before treating an `--all` result as the whole catalogue** — and never treat it
  as complete when the exit code is non-zero.
- **Every other `list`** (`tasks`, `boards`, `members`, `brands`, `categories`,
  `product-types`, `units`, `variants`) has **no `--all`** — page manually: start at
  `--page 1`, and while `meta.has_more` is `true`, request the next `--page`. Raising
  `--limit` to 100 cuts the number of round-trips.

Never treat a first page as complete when the answer depends on the full set (does X exist? is
this code/alias/barcode already taken? how many of Y are there?). Either narrow with
`--query`/`--ids` until the result fits one page, or page to the end using `meta.total` /
`meta.has_more`.

Counting "how many X are there?": **never answer by counting the rows you see** — that's one
page (≤20 by default). Read `meta.total` directly with a 1-row page so you don't pull the
whole collection:

```bash
capigo --tenant acme brands list --limit 1 | jq '.meta.total'
```

### Passing JSON input (`--from-json`)

Write commands that carry a structured body accept `--from-json <path>`, where `-` means
stdin. This is the reliable path for anything richer than a few flags (options + variants,
alias arrays, etc.):

```bash
echo '{"name":"Pin iPhone 13","aliases":["AP-BA-13"]}' \
  | capigo --tenant acme products create --from-json -
```

When `--from-json` is given, individual field flags are ignored (and for `products update`
they are mutually exclusive — passing both errors out).

### Finding records: code vs UUID

The single-item `get` commands (`products get`, `variants get`, `members get`,
`tasks get`, `boards get`) are **UUID-addressed only** — there is no "get by Product Code /
SKU / barcode / task code" yet. Two caveats worth knowing when you search by human key first:

- **Substring direction matters.** `--query` matches `stored_value ILIKE '%your-term%'` — the
  *stored* value must **contain** your term. So searching a term that is *longer* than the
  stored value finds nothing: if an alias is stored shortened as `VVD013`, querying the full
  code `SLM-DS-VVD013` returns **zero** rows. Search the shortest distinctive fragment
  (`VVD013`) instead. When you can't predict the stored form, fall back to `--all` + local
  filter over `.data[].aliases[]`.
- **For soft-deleted (tombstone) products, `--query` only matches name and aliases** —
  variant fields (name/SKU/barcode) are not indexed for deleted products, so to sweep
  tombstones use `--all` / `--updated-since` instead of `--query`.

### Pre-staged commands

Some commands ship in the CLI ahead of the matching API reaching production — the current
pre-staged set (shipped ahead of prod) is tracked in `CHANGELOG.md`'s entries; check there for
what's currently provisional. As of this writing, `tasks subtasks` and
`tasks create --subtasks-json` are pre-staged: they exist on the platform's `develop` branch
but may 404 on a tenant whose API hasn't deployed yet.

If a pre-staged command 404s / returns unimplemented on a given tenant, that's expected — fall
back to the `list`-based path (see [Finding records](#finding-records-code-vs-uuid)) or, for
subtasks, tell the user that endpoint isn't live on that tenant yet. Do not look up an API
endpoint or the OpenAPI spec to work around it — that is out of scope for the agent (see
`SKILL.md`); report it as a finding instead.

## Exit codes and self-diagnosis

See `SKILL.md` → Gate 2 for the exit-code table and the full self-diagnosis flow (the on-stdout
diagnosis block, `--help`, escalation).

The one nuance not covered there: **`--verbose` re-runs the call and adds the full HTTP
request/response trace** (auth header redacted) — the error body itself is already shown
without it, so reach for `--verbose` only when you need the request line too.
