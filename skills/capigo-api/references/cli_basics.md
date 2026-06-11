# `capigo` CLI — fundamentals and command reference

How the `capigo` command-line tool works: authentication, tenants, output, the full command
surface, and how to self-diagnose when a call behaves unexpectedly. Read the section you
need; `../SKILL.md` already covers the high-level rules.

> Everything here describes the binary the agent actually runs. If a command's real `--help`
> output disagrees with this file, the binary wins — and the OpenAPI document
> (`https://platform.capigo.app/api/openapi`) is the ultimate source of truth. See
> [Self-diagnosis](#self-diagnosis) at the end.

## Contents

- [Mental model](#mental-model)
- [Authentication](#authentication)
- [Config](#config)
- [Tenants and tenant scoping](#tenants-and-tenant-scoping)
- [Output modes and global flags](#output-modes-and-global-flags)
- [Pagination](#pagination)
- [Environment variables](#environment-variables)
- [Passing JSON input (`--from-json`)](#passing-json-input---from-json)
- [Command reference](#command-reference)
  - [auth](#auth) · [config](#config-commands) · [tenants](#tenants) · [tasks](#tasks) ·
    [boards](#boards) · [members](#members) · [products](#products) · [variants](#variants) ·
    [reference data: brands / categories / product-types / units](#reference-data)
- [Finding records: code vs UUID](#finding-records-code-vs-uuid)
- [Exit codes](#exit-codes)
- [Self-diagnosis](#self-diagnosis)

## Mental model

`capigo` is a thin, predictable wrapper over the Capigo Public API. It manages the API key,
attaches the tenant header, maps HTTP errors to stable exit codes, and shapes output as a
human table or machine JSON. Because of that, an agent should always go through the CLI and
never assemble raw HTTP requests — the CLI is the contract.

A call looks like:

```
capigo [global flags] <group> <command> [args] [flags]
```

Global flags work anywhere; most scoping flags (notably `--tenant`) are per-command.

## Authentication

The CLI authenticates with an API key that starts with `csk_`. It is read, in order of
precedence, from the `CAPIGO_API_KEY` env var, then `~/.capigo/config.json`.

```bash
capigo auth login --key csk_xxx     # store the key in the active profile
capigo auth whoami                  # confirm who you are (hits /me)
capigo auth logout                  # clear the stored key
capigo health                       # preflight: API reachable + key accepted (exit 0 = ok)
```

Always confirm `auth whoami` (or `capigo health`) succeeds before a batch of work. `health`
is the cheapest automated preflight — exit 0 means the API is reachable and the key is
accepted; a non-zero exit (e.g. 2) tells you why before you run real work. JSON mode emits
`{"ok":bool,"timestamp":string}`. It is not tenant-scoped. If any command returns **exit code
2**, the key is missing/invalid/expired — ask the user to log in again rather than retrying.

## Config

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

## Tenants and tenant scoping

`--tenant <code>` is a **per-command** flag (not global). Resolution order: `--tenant` flag →
`CAPIGO_TENANT` env → `default_tenant` in config.

**This is the rule that trips people up, so internalize it:**

- **Every PCMS command requires a tenant** — `products`, `variants`, `brands`, `categories`,
  `product-types`, `units`, for *every* verb (list, get, create, update, replace, variants).
  With no tenant resolved, the CLI rejects the call (exit 5) and the API would reject it too.
  Consequence: catalogue searches, barcode counters, and uniqueness checks are all scoped to
  **one tenant** — there is no cross-tenant PCMS read.
- **Mission reads may span tenants** — `tasks list/get` and `boards list/get` work without
  `--tenant` and then return rows across every tenant you can access (a "Tenant" column is
  added in table mode). `tasks create` requires a tenant.

List the tenants you can reach:

```bash
capigo tenants list --output json
```

## Output modes and global flags

| Global flag | Meaning |
|---|---|
| `-o, --output table\|json\|quiet` | Output format. Default `table`. Unknown values are rejected. |
| `--api-url <url>` | Override the API base URL (staging / local dev). |
| `-v, --verbose` | Print the HTTP request/response (Authorization header redacted). |

- **`table`** — human-readable, for display only. Don't parse it.
- **`json`** — machine-readable. Use this for anything you'll read programmatically. **JSON
  contract (stable as of v0.6):** every `list` command emits `{"data":[…],"meta":{…}}` — read
  the array at `.data[]`, not the top level. The `meta` object carries pagination — see
  [Pagination](#pagination) below; **one call is not the whole result set**. Single-item
  commands (`get`, `create`, `update`, `replace`) emit the **bare object** (no array wrapper).
  So: `… products list -o json | jq '.data[]'` but `… products get <id> -o json | jq '.name'`.
- **`quiet`** — prints just the resource ID, handy for shell piping.

`products list` also prints the server timestamp to **stderr** (`Server time: …`); feed it
back as `--updated-since` for incremental delta sync.

## Pagination

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

- **`products list` has `--all`** — it auto-paginates internally and streams every row.
  Prefer it whenever you need the full catalogue (e.g. alias/Product-Code checks):
  `capigo --tenant acme products list --all --output json | jq '.data[]'`.
- **Every other `list`** (`tasks`, `boards`, `members`, `brands`, `categories`,
  `product-types`, `units`, `variants`) has **no `--all`** — page manually: start at
  `--page 1`, and while `meta.has_more` is `true`, request the next `--page`. Raising
  `--limit` to 100 cuts the number of round-trips.

> **In table mode** the CLI nudges you on stderr — `Showing 20 of 137. Use --page / --limit to
> paginate.` (and `, or --all` for products). **In JSON mode there is no such nudge** — the
> agent path is JSON, so *you* must inspect `meta.has_more` yourself. Never treat a first page
> as complete when the answer depends on the full set (does X exist? is this code/alias/barcode
> already taken? how many of Y are there?). Either narrow with `--query`/`--ids` until the
> result fits one page, or page to the end.

## Environment variables

The CLI binds three env vars (useful for CI / agents that inject secrets without writing a
config file):

| Var | Effect |
|---|---|
| `CAPIGO_API_KEY` | API key (`csk_…`) |
| `CAPIGO_TENANT` | Default tenant code for the call |
| `CAPIGO_API_URL` | Override API base URL |

```bash
CAPIGO_API_KEY=csk_… CAPIGO_TENANT=acme capigo products list --output json
```

## Passing JSON input (`--from-json`)

Write commands that carry a structured body accept `--from-json <path>`, where `-` means
stdin. This is the reliable path for anything richer than a few flags (options + variants,
alias arrays, etc.):

```bash
echo '{"name":"Pin iPhone 13","aliases":["AP-BA-13"]}' \
  | capigo --tenant acme products create --from-json -
```

When `--from-json` is given, individual field flags are ignored (and for `products update`
they are mutually exclusive — passing both errors out).

## Command reference

Flags below are the notable ones; run `capigo <group> <command> --help` for the complete,
authoritative list from your binary.

### auth

| Command | Flags | Notes |
|---|---|---|
| `auth login` | `--key csk_…` (required) | Stores the key in the active profile. |
| `auth whoami` | | Shows the authenticated user (GET `/me`). |
| `auth logout` | | Clears the stored key. |

### config commands

`config set <key> <value>` · `config get <key>` · `config set-default-tenant <code>` ·
`config unset-default-tenant`.

### tenants

`tenants list` — lists tenants the user can access; takes no `--tenant`. Discovered tenant
codes are merged into `known_tenants` in config.

### tasks

Tenant is **optional** for reads, **required** for `create`.

| Command | Key flags |
|---|---|
| `tasks list` | `--tenant`, `--query/-q`, `--status`, `--parent-task-id` (use `null` for top-level only), `--page`, `--limit` |
| `tasks get <id>` | `--tenant` |
| `tasks comments <id>` | `--tenant` (optional); `--type comment\|activity` (default both), `--sort asc\|desc` (default `desc` = newest first), `--page`, `--limit` (max 50). UUID-addressed only. |
| `tasks create` | `--title` (required), `--tenant` (required), `--description`, `--priority`, `--status`, `--due-date` (RFC3339), `--assignee` (user id), `--board` (id), `--list` (board list id), `--follower-id` (repeatable) |
| `tasks update <id>` | `--tenant` (optional); any of `--title`, `--description` (empty string clears), `--status`, `--assignee` (UUID; `--assignee ""` unassigns), `--board` + `--list` (sent together; `--board "" --list ""` removes from board), `--follower-id` (repeatable, additive — removal not supported). At least one flag required. UUID-addressed only. |

```bash
capigo tasks list --status To-Do --output json
capigo tasks create --tenant acme --title "Fix login bug" --priority high --output quiet
```

#### Reading a task's discussion + history (`tasks comments`)

Use this when you need a task's **hiện trạng** — what people said and how the work
progressed — before summarizing it or deciding what follow-up task to create. It returns the
task's timeline: human comments interleaved with system activity, oldest-or-newest first.

Parse `--output json` and read `.data[]`. Each entry tells you what it is via `kind`:

- `kind: "comment"` — a message a person or agent typed; the real discussion lives in
  `content`.
- `kind: "activity"` — a system event (status change, (re)assignment, title/description/
  due-date edit, task creation). `content` is a ready-made sentence
  (`"Trâm changed status from Doing to Done"`) and `ui_data` holds the structured before/after.

Other fields per entry: `author {id, name, type}` (who did it), `attachments[]`
(`file_name`/`mime_type`/`size_bytes`), `parent_id`, `created_at`.

```bash
# Whole timeline, newest first
capigo tasks comments <task-uuid> --output json | jq '.data[] | {created_at, kind, author: .author.name, content}'
# Only the human discussion (skip status/assignment noise)
capigo tasks comments <task-uuid> --type comment --output json
```

Two things to keep honest:

- **`author.name` may be `"System"`.** That's a graceful fallback when the original
  actor can't be resolved (e.g. a removed member) — not an error. Don't block on it.
- **For the *current* status, trust the task, not the latest activity message.** Activity
  events are written asynchronously and can lag by a moment, so the authoritative state is the
  task itself (`tasks get`). Treat `tasks comments` as the history/narrative, not the source of
  truth for the live status/assignee.

### boards

Tenant **optional** for reads.

| Command | Key flags |
|---|---|
| `boards list` | `--tenant`, `--page`, `--limit` (default 20) |
| `boards get <id>` | `--tenant` — returns the board with its `lists` array |

### members

Workspace members. Tenant **optional** for reads; omitting it resolves across all accessible
tenants.

| Command | Key flags |
|---|---|
| `members list` | `--tenant`, `--query/-q` (name/email search), `--page`, `--limit` |
| `members get <id>` | `--tenant` — 404 for an inaccessible or cross-tenant member. UUID-addressed only. |

### products

Tenant **required** on all subcommands.

| Command | Key flags |
|---|---|
| `products list` | `--tenant` (req), `--query/-q` (2–500 chars; matches name, variant name, SKU, barcode), `--updated-since` (ISO 8601 delta sync), `--ids` (comma UUIDs, max 50; mutually exclusive with `--all`), `--all` (auto-paginate), `--page`, `--limit` (1–100, default 20) |
| `products get <id>` | `--tenant` (req) — full single product (variants, options, brand, category, type, unit). UUID-addressed only. |
| `products create` | `--tenant` (req); simple mode: `--name` (req), `--sku`, `--barcode`, `--price`, `--status` (DRAFT/ACTIVE/ARCHIVED), `--currency`, `--description`, `--brand-id`, `--category-id`, `--product-type-id`, `--unit-id`; or `--from-json -` for options+variants |
| `products update <id>` | `--tenant` (req); any of `--name`, `--description`, `--status`, `--currency`, `--brand-id`, `--category-id`, `--product-type-id`, `--unit-id`, `--aliases` (repeatable); or `--from-json -` (mutually exclusive with field flags). At least one field required. |
| `products variants` | `--tenant` (req), `--product-id` (req), `--from-json -` (req) — a **JSON array** of variant objects |

Key facts callers depend on:

- **`products create` simple mode has no `--aliases` flag.** To attach a product code alias
  (codes conventionally live in `aliases[]`), create via `--from-json` with
  `"aliases": [...]`, or set them after creation with `products update <id> --aliases …`.
- **`products variants` takes a JSON array**, not an object. An item **with** `variant_id` is
  updated; **without** `variant_id` it is created (and `name` is required). One call upserts
  many variants at once.
- The variant `sku` field carries the variant's code; the variant `barcode` field carries the
  numeric barcode.

```bash
# Create a simple product with a Product Code alias
echo '{"name":"Pin iPhone 13 Pro Max","brand_id":"<uuid>","product_type_id":"<uuid>",
       "unit_id":"<uuid>","status":"DRAFT","aliases":["AP-BA-13PM"]}' \
  | capigo --tenant acme products create --from-json -

# Upsert variants on that product
echo '[{"name":"Đen","sku":"AP-BA-13PM-B","barcode":"63400700011"}]' \
  | capigo --tenant acme products variants --product-id <uuid> --from-json -
```

### variants

`variants list` — **tenant required**. Lists PCMS variants filtered by barcode prefix; its
main job is finding the highest barcode under a prefix (e.g. for an allocation scheme that
auto-increments within a namespace).

| Flag | Notes |
|---|---|
| `--tenant` | required |
| `--barcode-prefix` | filter variants whose barcode starts with this value |
| `--sort` | `barcode` (asc) or `-barcode` (desc); default `-barcode` |
| `--page`, `--limit` | pagination (limit default 20) |

```bash
capigo --tenant acme variants list --barcode-prefix 634007 --sort -barcode --limit 1 --output json
```

`variants get <id>` — **tenant required** — fetches one variant's full detail (sku, barcode,
price, options, type, timestamps). UUID-addressed only; orphaned/soft-deleted/cross-tenant
variants return 404.

### reference data

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
capigo --tenant acme brands list --query nike --output json
capigo --tenant acme product-types create --name "Pin Liền Cáp" --output json
echo '{"name":"Pin Liền Cáp"}' | capigo --tenant acme product-types create --from-json -
```

> Reference-data `create`/`update` exist and are stable as of CLI v0.5.0. (Earlier skill
> notes warned they were "being added" — that warning is obsolete.)

## Finding records: code vs UUID

The single-item `get` commands (`products get`, `variants get`, `members get`,
`tasks get`, `boards get`) are **UUID-addressed only** — there is no "get by Product Code /
SKU / barcode / task code" yet. So when you have a human key rather than a UUID, find the
record first, then act on its `id`:

- Product by name / SKU / variant name / barcode → `products list --query "<term>"`, read
  `.data[].id`.
- Product by code alias → `products list --all` and filter `.data[].aliases[]`
  locally (`--query` does not index aliases).
- Variant barcode lookups → `variants list --barcode-prefix …`.

**Pre-staged commands:** some commands were shipped in the CLI ahead of the matching API
reaching production — the v0.7 set (`products get`, `tasks update`, `members get`,
`variants get`) and `tasks comments` (v0.8). If one returns a not-found / unimplemented error
on a tenant whose API hasn't deployed yet, that's expected — fall back to the `list`-based path
above (or, for `tasks comments`, just tell the user the timeline endpoint isn't live yet) and
confirm against the OpenAPI doc (see [Self-diagnosis](#self-diagnosis)).

## Exit codes

Branch on the code, not the message text.

| Code | Meaning | Action |
|---|---|---|
| 0 | Success | Continue |
| 1 | General / unexpected | Read stderr |
| 2 | Auth error | `capigo auth login --key csk_…` |
| 3 | Permission denied (403) | Caller lacks tenant/resource access |
| 4 | Not found (404) | Re-check the ID |
| 5 | Validation error (400) | Fix the payload from stderr |
| 6 | Network error | Retry once, then surface |
| 7 | Rate limit (429) | Back off, retry |
| 8 | Conflict (409) | SKU/alias exists — change the value or update the existing row |

## Self-diagnosis

When a command errors in a way you don't understand, or the JSON doesn't match what a
reference doc here describes, look it up instead of guessing:

1. **`capigo <command> --help`** — exact flags/args/defaults from the running binary.
2. **`capigo … --verbose`** — re-run the failing call to see the real HTTP request and
   response body (auth header redacted); this usually shows the offending field directly.
3. **OpenAPI document** — the source of truth for endpoints, schemas, fields, and tenant
   requirements:

   ```bash
   curl -s https://platform.capigo.app/api/openapi | jq '.paths."/pcms/products".post'
   ```

**Trust order:** this skill's reference docs **<** real CLI behaviour **<** OpenAPI. If the
spec contradicts a reference here, the spec is right — tell the user so the skill can be
fixed.
