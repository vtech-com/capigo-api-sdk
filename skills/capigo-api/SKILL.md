---
name: capigo-api
description: >
  Drive the `capigo` CLI to read and write the Capigo platform — the entry point for any
  Capigo work an agent does for a tenant. Covers the full command surface (Mission Tasks,
  Boards, Members, and the PCMS catalogue: Products, Variants, Brands, Product Types,
  Categories, Units), how to find the right command for what you want to do, and what to do
  when a command errors or seems to be missing. Use this whenever a request maps to Capigo
  data, even if the user doesn't name the CLI — e.g. "tạo task / giao việc / danh sách công
  việc", "xem board", "đọc bình luận / xem hiện trạng / lịch sử trao đổi của một task", "liệt
  kê sản phẩm / tìm SKU / tra barcode". It is also the reference for how the CLI works at all:
  logging in, picking a tenant, reading exit codes, choosing output format, and self-diagnosing
  when a command misbehaves. If a request touches Capigo and you're unsure how to talk to it,
  reach for this skill first instead of guessing at raw API calls.
---

# capigo-api

The operating manual for the Capigo platform through the `capigo` command-line tool.

**Everything you do to Capigo goes through this CLI — never raw HTTP.** The CLI holds your API
key, attaches the tenant header, maps errors to stable exit codes, and shapes output; hand-rolling
a `curl` throws all that away and is the most common way an agent corrupts data or reaches a wrong
conclusion. You don't know the API's endpoints and never need to — the CLI *is* your interface.

Two ideas do most of the work here: **(1)** to find a command, scan the capability map below (a
complete, domain-grouped inventory) — don't guess a name from intuition; **(2)** a command you
can't find is a *finding, not a dead end* — if the map and `--help` show nothing, report the CLI
gap and stop; it never means the platform can't do it, and never justifies raw HTTP.

## The operating loop

Run this for every Capigo operation. The two 🛑 gates are where agents most often go wrong.

```
INTENT (at the API level: "update a variant's name", "find a product by code")
   │
   ▼
① LOOK IT UP in the capability map → which command does this?
   ├─ found ─────────────────────────────────┐
   └─ not found → 🛑 GATE 1                   │
        re-scan the domain block + run        │
        `capigo <group> --help`               │
        ├─ command exists → use it            │
        └─ still nothing → this is a CLI GAP: │
           report it as a finding and STOP.   │
           (never raw HTTP, never a workaround)│
                                              ▼
② ACT through the CLI (never raw HTTP)
   │
   └─ command errors? → 🛑 GATE 2
        read the on-stdout diagnosis block first,
        then `--help` / `--verbose`; still stuck → ask a human.
        Never invent a destructive workaround.
   │
   ▼
③ CONFIRM: read the echoed tenant + result, then report the truth.
```

The intent in ① is already at the **API level** — "I need to set a variant's `name`", not "the
user wants to rename a product." Translating a business request into an API intent (and any
policy about *whether* to do it) happens above this skill, in your task/catalogue skill or with
the user. This skill starts once the intent is mechanical.

## Common wrong turns (read these first)

These are the exact mistakes that recur — each is a guessed command name that doesn't exist,
which then tempts a wrong "the API can't do this" conclusion. The right command is always there:

- **Update / rename / edit a variant?** → `products variants` (in the Products block).
  🔴 **There is no `variants update` / `variants create`.** The `variants` group is read-only;
  every variant write is an upsert through `products variants`.
- **Get a product by its Product Code / SKU / barcode?** → there is **no "get by code"**.
  Use `products list --query "<code>"`, then act on the returned `.id`.
- **Count "how many X"?** → read `meta.total` from any `list -o json`. **Never count `data[]`**
  — that's one page (≤20).

## Capability map

A complete inventory of the CLI, grouped by domain. Jump to the block you need:

**Products · Variants · Tasks · Boards · Members · Reference data · Tenants/Auth**

Find the block your intent falls under, then read *every* row in it — the blocks are short by
design, and a command whose name you wouldn't have guessed (like `products variants`) is only
found by reading the whole block, not by matching a name. Full flags and examples for any
command are in [`references/cli_basics.md`](./references/cli_basics.md); this map is the index.

### Products  *(tenant required on every command)*

| Command | Reach for it when you want to… |
|---|---|
| `products list --query "<term>"` | find a product by name / alias / code / SKU / barcode. No "get by code" — query, then use `.id` |
| `products get <id>` | see one product in full (variants, options, brand). UUID only |
| `products create --from-json -` | create a product |
| `products update <id> --from-json -` | change a product's metadata: name, description, brand, status. "Archive" = set `{"status":"ARCHIVED"}`. Products have **one** write verb (send only the fields you're changing) — there is **no** `products replace`, unlike reference data |
| `products variants --product-id <pid> --from-json -` | **create or update variants (upsert)** — change a variant's name/sku/barcode/price, or add one. 🔴 This is the **only** variant write; there is **no** `variants update` |

### Variants  *(tenant required — this group is READ-ONLY)*

| Command | Reach for it when you want to… |
|---|---|
| `variants list --barcode-prefix <p> --sort -barcode --limit 1` | find the highest barcode under a prefix (allocation/counters) |
| `variants get <id>` | see one variant's full detail. UUID only |
| ↳ **change or add a variant** | → use **`products variants`** (Products block above). There is no `variants update`/`create`/`replace`. |

### Tasks  *(tenant optional for reads; required for `create`/`subtasks`)*

| Command | Reach for it when you want to… |
|---|---|
| `tasks list [--status/--priority/--assignee-id/--board-id/--due-*…]` | list tasks, optionally filtered |
| `tasks get <id>` | see one task in full. UUID only |
| `tasks comments <id>` | read a task's timeline (comments + activity). For the **current** status trust `tasks get`, not the latest activity line |
| `tasks create --title … --tenant …` | create a task; add `--subtasks-json -` to create it with subtasks atomically |
| `tasks update <id>` | change status / assignee / board / title (≥1 flag; `--assignee ""` unassigns) |
| `tasks subtasks <parent-id> --from-json -` | add subtasks under an existing task (all-or-nothing; max 25) |
| `tasks [comments] attachments download <task-id> <att-id>` | download a task or comment attachment (on demand; URL is single-use, ~5 min) |

### Boards · Members  *(tenant optional for reads)*

| Command | Reach for it when you want to… |
|---|---|
| `boards list [--query]` / `boards get <id>` | list boards / see one board (with its `lists`) |
| `members list [--query]` / `members get <id>` | list workspace members / see one member |

### Reference data — brands · categories · product-types · units  *(tenant required; same shape for all four)*

| Command | Reach for it when you want to… |
|---|---|
| `<group> list [--query]` | list ref data (name-contains query) |
| `<group> get <id>` | see one record. UUID only |
| `<group> create --from-json -` | create one (e.g. units require `--name --abbreviation`) |
| `<group> update <id>` | **partial** update (PATCH — only provided fields change) |
| `<group> replace <id>` | **full** replace (PUT — all fields required) |

### Tenants · Auth · preflight  *(no tenant)*

| Command | Reach for it when you want to… |
|---|---|
| `tenants list` | list the tenants you can reach |
| `capigo health` | **preflight** — is the key accepted and API reachable? (exit 0 = ok). Use this before a batch of work, not `auth whoami` |
| `capigo auth whoami` / `login --key csk_…` / `logout` | check identity / log in / out. ⚠️ `whoami` (GET `/me`) may **404** where it isn't deployed — it is *not* a reliable preflight; use `health`. On **exit 2** the key is bad — ask the *user* to re-login; don't retry |

## When you can't find a command (Gate 1 — a hard rule)

A missing command name is the exact trap that has caused real incidents: the agent guesses a
name (`variants update`), doesn't find it, concludes "the API can't do this," abandons the CLI,
and hand-rolls `curl` against a guessed endpoint. **Every step after the guess is wrong.** So:

1. **Re-scan the whole domain block**, not the name you expected. The right command is often
   named differently than you'd guess — a variant write is `products variants`, not `variants
   update`; there is no "get by code", you `list --query` instead.
2. **You may not conclude "unsupported" until you have run `capigo <group> --help`** and it shows
   no relevant subcommand. Absence of a name you imagined is *not* evidence; the binary's own
   help listing is. This step is mandatory, not optional.
3. **If `--help` genuinely shows nothing, it is a CLI gap — report it as a finding and STOP.**
   Tell the user plainly ("the CLI doesn't currently expose a way to X; this needs a CLI
   feature"). Do **not** look up an API endpoint, and do **not**, under any circumstance, call
   the API directly with `curl` or a fetched key. You act only through the CLI; when the CLI
   can't, the answer is to report it — never to route around it.

**The CLI now assists you here.** Running an unknown command errors (it no longer prints help
and exits 0), and when there's a known redirect its `Next:` line names the right command — e.g.
`variants update` points you to `products variants`, and `products delete` to archiving via
`products update --status ARCHIVED` — plus cobra suggests near-typos, git-style. **Read the
`Next:` line.** This doesn't replace the rule above: for a genuine gap the CLI can't redirect,
still report it and stop.

**Why:** a missing command feels like "the platform can't," but those are different things. The
platform is capable; the CLI is your window onto it; the window occasionally lacks a pane.
Reporting the missing pane gets it added. Climbing through the wall (raw HTTP) loses auth,
tenant scoping, and error mapping — and reliably reaches a wrong answer. (The OpenAPI spec and
raw endpoints are tools for the SDK team who will fix the gap — not for you at runtime.)

## When a command errors (Gate 2 — self-diagnose before guessing)

**A failed command is never proof a feature is missing.** A write that fails is about *your
request* — a bad field, a conflicting value, a wrong id — not about the API lacking the
capability. Never conclude "the API doesn't support X" from an error, and never invent a
destructive workaround (recreating and archiving a record, bulk-rewriting identifiers).

On any error the CLI prints a diagnosis **on stdout** — read it before doing anything else:

- `Server:` the API's own error message — usually the most specific account of what went wrong.
- `Means:` an added interpretation, shown only when the server message is a generic fallback.
- `Note:` the reminder above, shown on write failures.
- `Next:` the concrete fix to try.
- `Response:` the verbatim server body. `request_id=…` is what the API team uses to trace the
  call — surface it when you escalate a suspected false error.

If that isn't enough: **`capigo <command> --help`** (exact flags/defaults), then
**`capigo … --verbose`** (full HTTP trace, auth redacted). Still stuck → **ask a human**;
surface the `Response:` block and `request_id`. Do not guess a cause or build a workaround.

Branch on the **exit code**, not the message wording — the text may change, the codes won't:

| Code | Meaning | What to do |
|---|---|---|
| 0 | Success | Continue |
| 1 | General / unexpected | Read stderr; likely a malformed call |
| 2 | Auth error (key invalid/expired) | Ask the user to `capigo auth login --key csk_…`; don't retry |
| 3 | Permission denied (403) | The caller lacks access to that tenant/resource — surface it |
| 4 | Not found (404) | Re-check the id |
| 5 | Validation error (400) | Read stderr, fix the payload, retry |
| 6 | Network error | Retry once; if it persists, surface it |
| 7 | Rate limit (429) | Back off, then retry |
| 8 | Conflict (409) | A server-enforced unique value already exists (e.g. `sku` — but *not* alias/barcode, which allow duplicates) — change the value or update the existing row |

## Tenant handling

Decide the tenant at the start of every operation. `--tenant <code>` is **per-command** (not
global); resolution order: `--tenant` flag → `CAPIGO_TENANT` env → `default_tenant` in config.

- **PCMS commands** (`products`, `variants`, `brands`, `categories`, `product-types`, `units` —
  every verb) **require** a tenant (else exit 5); catalogue reads are always single-tenant.
- **Mission/Members reads** (`tasks/boards/members list/get`) may omit `--tenant` to span all
  accessible tenants. `tasks create` requires one.

**Check the tenant the CLI echoes.** Every table-mode list footer and every successful write
prints `Tenant: <code>` (with `(from CAPIGO_TENANT)` / `(from config default_tenant)` when
resolved implicitly). If it isn't the tenant the user meant, the data you read — or wrote — is in
the wrong place: stop and redo with an explicit `--tenant`.

If the tenant isn't clear and the user didn't name one, ask before any fetch — offer their
default (`default_tenant`) or the list from `capigo tenants list`.

## Write hygiene (applies to every write)

Generic safety for any agent writing to Capigo. (Organisation-specific rules — code formats,
barcode allocation — live in your catalogue-policy skill, not here.)

- **Confirm before you write.** The safe rhythm is **propose → wait for confirmation →
  execute**. A wrong write costs far more than a clarifying question.
- **Check for collisions first — but know what the server actually enforces.** Only **`sku`** is
  server-enforced unique per tenant: a duplicate SKU fails with **exit 8** (`E9445`), which you
  treat as "it already exists", not as retryable. **`alias` and `barcode` are NOT enforced —
  the platform allows duplicates by design**, so a duplicate alias/barcode will *not* error. If
  your catalogue policy needs them unique, that's on you: search first with
  `products list --query "<value>"` (matches name, aliases, tags, SKU, barcode server-side) and
  decide before writing — don't expect the server to reject it. (Caveat: `--query` is a substring
  on the *stored* value; a shortened stored alias won't be found by the full code — search the
  fragment or use `products list --all`.)
- **Don't silently change identifiers.** Changing an existing `sku`, `barcode`, or alias on a
  live record breaks whatever references it — only do so when the user explicitly asks.
- **Never report a soft-deleted record as available.** In table mode Status shows e.g. `ACTIVE
  (DELETED)`; in JSON check `is_deleted` (the `status` field alone does not reveal deletion).

## Output modes (pick before you act)

- **`table`** (default) — human prose for reading on screen. **Never redirect (`>`) or pipe
  (`|`) table output**: it's text, not JSON, so `json.load()` / `jq` on it fails. The moment you
  add `>` or `|`, also add `-o json`.
- **`-o json`** — for anything you'll parse, store, or pipe. A `-o json` stream is pure JSON
  (the `Server time:` line moves to stderr, so there's no prefix to strip). Contract: every
  `list` returns `{"data":[…],"meta":{…}}` (read `.data[]`); single-item commands return the
  bare object.
- **`quiet`** — prints just an id.

Deeper mechanics — pagination, delta sync, `--from-json`, per-command flags, code-vs-UUID
lookup — are in [`references/cli_basics.md`](./references/cli_basics.md). Reach for it when the
map points you to a command and you need its exact flags.

## What this skill does NOT do

- **Talk to Capigo over raw HTTP.** Everything goes through `capigo`. If the CLI can't do
  something, that's a finding to report — not an endpoint to call.
- **Look up or reason about API endpoints.** You don't know them and don't need them.
- **Define catalogue coding policy** — Product Code formats, barcode allocation, brand naming.
  That's organisational policy; it belongs in a catalogue-policy skill layered on top of this
  one. (VTech internal: `manage-capigo-product` in `vtech-com/agent-skills`.)
- **Bulk-migrate legacy catalogues** — out of scope; a separate effort.
