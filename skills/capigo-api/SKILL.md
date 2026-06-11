---
name: capigo-api
description: >
  Drive the `capigo` CLI to read and write the Capigo platform — the entry point for any
  Capigo work an agent does for a tenant. Covers authentication, tenant handling, exit codes,
  output modes, and the full command surface: Mission Tasks, Boards, Members, and the PCMS
  catalogue (Products, Variants, Brands, Product Types, Categories, Units). Use this whenever
  a request maps to Capigo data, even if the user doesn't name the CLI — e.g. "tạo task /
  giao việc / danh sách công việc", "xem board", "đọc bình luận / xem hiện trạng / lịch sử
  trao đổi của một task", "liệt kê sản phẩm / tìm SKU / tra barcode". It is also the
  reference for how the `capigo` CLI works at all: logging in, picking a tenant, reading exit
  codes, choosing output format, and self-diagnosing when a command misbehaves. If a request
  touches Capigo and you're unsure how to talk to it, reach for this skill first instead of
  guessing at raw API calls.
---

# capigo-api

The operating manual for the Capigo platform through the `capigo` command-line tool.
Everything an agent does to Capigo — looking something up, creating a product, filing a
task — goes through this CLI, never through raw HTTP. The CLI already handles
authentication, tenant headers, error mapping, and output shaping, so leaning on it keeps
the agent honest and the behaviour predictable.

Read this file end to end the first time. The full command surface, flags, and worked
examples live in [`references/cli_basics.md`](./references/cli_basics.md) — jump there for
any specific command.

## Scope

| In scope | Where |
|---|---|
| How the CLI works: auth, tenants, output modes, exit codes, `--from-json`, self-diagnosis | This file + [`references/cli_basics.md`](./references/cli_basics.md) |
| Every command group: `tasks`, `boards`, `members`, `products`, `variants`, `brands`, `categories`, `product-types`, `units`, `tenants`, `config`, `auth`, `health` | [`references/cli_basics.md`](./references/cli_basics.md) |

**Out of scope — catalogue coding policy.** How an organisation assigns Product Codes,
allocates barcodes, names brands, or keeps its reference data in sync is *organisational
policy*, not CLI mechanics. Pair this skill with your organisation's own catalogue-policy
skill for that. (VTech internal: `manage-capigo-product` in the `vtech-com/agent-skills`
repo builds exactly that layer on top of this skill.)

## CLI fundamentals (read once)

- **Authentication.** The CLI reads an API key (`csk_…`) from `~/.capigo/config.json` or the
  `CAPIGO_API_KEY` env var. Confirm the agent is logged in with `capigo auth whoami`, or run
  `capigo health` as a one-shot preflight (exit 0 = API reachable and key accepted). An auth
  failure surfaces as **exit code 2** — when you see it, ask the user to run
  `capigo auth login --key csk_…` rather than retrying blindly.
- **Output format.** Add `--output json` (or `-o json`) to anything you intend to parse;
  `table` (default) is for humans, `quiet` prints only an ID. Always parse `json`, never
  scrape the table. **JSON contract:** every `list` command returns
  `{"data":[…],"meta":{…}}` (read `.data[]`); single-item commands (`get`/`create`/`update`)
  return the bare object.
- **Pagination.** Every `list` returns at most one page (default 20 rows, max 100) — **not the
  whole collection**. Check `meta.has_more` in the JSON and keep paging (`--page`) until it's
  `false`, or use `products list --all`. In JSON mode there is no stderr nudge, so this is on
  you. See `references/cli_basics.md` → Pagination.
- **Tenant scoping.** `--tenant <code>` is a **per-command** flag, not global. Every PCMS
  command (products, variants, brands, categories, product-types, units — read *and* write)
  **requires** a tenant; only `tasks`/`boards`/`members` reads may omit it to span tenants.
  See **Tenant handling** below.
- **JSON input.** Write operations accept a JSON body on stdin via `--from-json -`, e.g.
  `echo '<json>' | capigo --tenant acme products create --from-json -`. This is the reliable
  path for anything richer than a couple of flags.

## Exit codes

Branch on the exit code, never on the wording of an error message — the text may change, the
codes will not.

| Code | Meaning | What to do |
|---|---|---|
| 0 | Success | Continue |
| 1 | General / unexpected error | Read stderr; likely a bug or malformed call |
| 2 | Auth error (key invalid/expired) | Ask the user to `capigo auth login --key csk_…` |
| 3 | Permission denied (403) | The caller lacks access to that tenant/resource — surface it |
| 4 | Not found (404) | Re-check the ID |
| 5 | Validation error (400) | Read stderr, fix the payload, retry |
| 6 | Network error | Retry once; if it persists, surface it |
| 7 | Rate limit (429) | Back off, then retry |
| 8 | Conflict (409) | SKU/alias/resource already exists — change the offending value or update the existing row |

## When something looks wrong — self-diagnose before guessing

If a command errors in a way you don't understand, or the JSON you get back doesn't match
what this skill describes, **don't invent a workaround** — go look it up. Three sources, from
cheapest to most authoritative:

1. **`capigo <command> --help`** — exact flags, arguments, and defaults for that command,
   straight from the binary you're actually running. Start here.
2. **`capigo … --verbose`** (or `-v`) — re-run the failing command with this flag to see the
   real HTTP request and response (the Authorization header is redacted). This shows you the
   exact payload sent and the error body returned, which usually pinpoints a bad field.
3. **The OpenAPI document** — the source of truth for every endpoint, schema, field, and
   tenant requirement: `https://platform.capigo.app/api/openapi`. It sits on the same host
   the CLI already talks to, so you can fetch and inspect it, e.g.
   `curl -s https://platform.capigo.app/api/openapi | jq '.paths."/pcms/products"'`.

**Precedence when sources disagree:** a reference file in this skill describes the *intended*
behaviour, but the live system is what actually runs. So trust order is: this skill's
reference docs **<** the real CLI behaviour (`--help`, `--verbose`) **<** the OpenAPI
document. If the OpenAPI spec contradicts a doc here, the spec wins — note the drift to the
user so the skill can be corrected.

## Write hygiene (applies to every write)

These are generic safety rules for any agent writing to Capigo. (Organisation-specific
rules — code formats, barcode allocation, reference-data governance — live in your
catalogue-policy skill, not here.)

- **Writes always name a tenant.** No PCMS write without `--tenant <code>`; the CLI rejects
  it (exit 5) anyway.
- **Confirm before you write.** The safe rhythm is **propose → wait for the user's
  confirmation → execute**. A wrong write is far more expensive than a clarifying question.
- **Check for collisions first.** A new `sku`, alias, or `barcode` must be unique within the
  tenant — search before you insert, and treat **exit 8** (conflict) as "it already exists",
  not as a retryable error. When that search is a `list`, remember it's paginated: a clean
  first page does **not** prove uniqueness — page to the end (or `products list --all`, or a
  `--query`/`--ids` narrow enough to fit one page) before concluding "no collision".
- **Don't silently change identifiers.** Changing an existing `sku`, `barcode`, or alias on a
  live record breaks whatever references it — only do so when the user explicitly asks.

## Tenant handling

Decide the tenant at the very start of every operation, before fetching anything.

- **PCMS commands** (`products`, `variants`, `brands`, `categories`, `product-types`,
  `units` — every verb, read and write) **require** `--tenant <code>`. This means catalogue
  lookups and uniqueness checks are all scoped to a single tenant.
- **Mission and Members reads** (`tasks list/get`, `boards list/get`, `members list/get`) may
  omit `--tenant` to span every tenant the caller can access. `tasks create` requires a
  tenant; `tasks update` accepts an optional one.

### How to determine the tenant

After parsing the request, before any data fetch, if the tenant isn't already clear, ask:

> "Which tenant should I work in?
> (1) Your default tenant: **{default_tenant_name}** (`{default_tenant_code}`)
> (2) Choose from the list"

If the user picks (2), run `capigo tenants list --output json` and offer the options. If the
user already named a tenant (company name or code) in the request, map it and skip the
question. Store the chosen code as `TENANT_CODE` and reuse it throughout.

## What this skill does NOT do

- Talk to Capigo over raw HTTP. Everything goes through `capigo`.
- Define catalogue coding policy — Product Code formats, barcode allocation, brand naming
  conventions, reference-data governance. That is organisational policy; it belongs in a
  separate skill layered on top of this one.
- Bulk migration of legacy catalogues — out of scope; a separate effort.
