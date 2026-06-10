---
name: capigo-api
description: >
  Drive the `capigo` CLI to read and write the Capigo platform — the entry point for any
  Capigo work an agent does for a tenant. Covers PCMS catalogue (Products, Variants, Brands,
  Product Types, Categories, Units) including Product Code / Barcode generation, plus Mission
  Tasks, Boards, and Members. Use this whenever a request maps to Capigo data, even if the user doesn't
  name the CLI — e.g. "đánh mã / tạo mã / sinh code", "thêm sản phẩm / thêm biến thể / sửa sản
  phẩm", "cập nhật barcode", "thêm brand / tạo product type mới / đổi tên thương hiệu", "kiểm
  tra sync / check drift", "tạo task / giao việc / danh sách công việc", "xem board", "đọc bình
  luận / xem hiện trạng / lịch sử trao đổi của một task", "liệt kê sản phẩm / tìm SKU". It is
  also the reference for how the `capigo` CLI works at all: logging
  in, picking a tenant, reading exit codes, choosing output format, and self-diagnosing when a
  command misbehaves. If a request touches Capigo and you're unsure how to talk to it, reach
  for this skill first instead of guessing at raw API calls.
---

# capigo-api

Single entry point for operating the Capigo platform through the `capigo` command-line tool.
Everything an agent does to Capigo — looking something up, creating a product, allocating a
barcode, filing a task — goes through this CLI, never through raw HTTP. The CLI already
handles authentication, tenant headers, error mapping, and output shaping, so leaning on it
keeps the agent honest and the behaviour predictable.

This skill is a **dispatcher**. It teaches the shared fundamentals (how to call the CLI, how
to read its exit codes, how to pick a tenant, how to recover when something looks wrong) and
then routes the specific request to the matching workflow document. Read this file end to
end the first time; afterwards you can jump straight to the relevant workflow.

## Scope at a glance

| What the user wants | Where to go |
|---|---|
| Create/update one Product or Variant; generate Product Code / Variant Code / Barcode | [`references/workflows/manage_product.md`](./references/workflows/manage_product.md) |
| Add a Brand, rename a Brand, reconcile Brand drift | [`references/workflows/manage_brand.md`](./references/workflows/manage_brand.md) |
| Add a Product Type, rename one, reconcile Type drift | [`references/workflows/manage_product_type.md`](./references/workflows/manage_product_type.md) |
| Check that `coding_references.md` and Capigo agree (drift detection) | [`references/workflows/sync_check.md`](./references/workflows/sync_check.md) |
| Read a task's **hiện trạng** — its comments and status/assignment history — before summarizing it or deciding on a follow-up task | Drive the CLI directly — see `tasks comments` in [`references/cli_basics.md`](./references/cli_basics.md) |
| Tasks, Boards, Members, Categories, Units, or any plain read/lookup | Use the CLI directly — see [`references/cli_basics.md`](./references/cli_basics.md) |

Categories and Units have full CRUD on the CLI but no dedicated catalogue workflow yet; read
them live and follow `cli_basics.md`. Tasks (`list`/`get`/`create`/`update`), Boards, and
Members are likewise driven straight from `cli_basics.md`. When one of these grows its own
multi-step procedure, add a workflow doc here and link it from the table — the dispatcher
pattern is meant to expand.

## CLI fundamentals (read once)

The full command surface, flags, and worked examples live in
[`references/cli_basics.md`](./references/cli_basics.md). The essentials you need before any
operation:

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
- **Tenant scoping.** `--tenant <code>` is a **per-command** flag, not global. Every PCMS
  command (products, variants, brands, categories, product-types, units — read *and* write)
  **requires** a tenant; only `tasks`/`boards` reads may omit it to span tenants. See
  **Tenant handling** below.
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
domain shape, but the live system is what actually runs. So trust order is: this skill's
reference docs **<** the real CLI behaviour (`--help`, `--verbose`) **<** the OpenAPI
document. If the OpenAPI spec contradicts a reference doc here, the spec wins — note the
drift to the user so the skill can be corrected.

## Hard invariants (apply to every workflow)

These are non-negotiable because breaking them corrupts a shared catalogue or crosses a
tenant boundary. If you can't satisfy one, stop and ask — a wrong write is far more expensive
than a clarifying question.

- **`references/coding_references.md` is the source of truth** for Brand `prefix` /
  `barcode_part` and Product Type `prefix` / `barcode_part`. Capigo itself stores only `id`,
  `name`, and (for Brands) `logo_url` — it does **not** store these coding fields. The file
  and Capigo must agree on `name`; any divergence is drift to resolve via the brand/type/sync
  workflows, not to paper over.
- **Never invent reference data.** A Brand or Product Type must already exist both in
  `coding_references.md` and on Capigo before any product write references it. If either side
  is missing, route to the matching workflow first.
- **Identifiers are stable by default.** Don't change an existing `product.code`,
  `variant.code`, `variant.barcode`, Brand `prefix`/`barcode_part`, or Product Type
  `prefix`/`barcode_part`. Changing them orphans existing codes, so it requires an explicit
  user request and confirmation.
- **New writes must be unique within the tenant.** A new `product` alias, `variant.sku`, or
  `variant.barcode` must not collide with anything already in that tenant's catalogue. Verify
  before you insert. (PCMS is tenant-scoped — uniqueness is per-tenant, not global.)
- **Human in the loop.** Never create or update without the user's explicit confirmation of
  the final proposal. The default rhythm is **propose → wait → execute**.
- **Writes always name a tenant.** No PCMS write without `--tenant <code>`.

## Tenant handling

Decide the tenant at the very start of every operation, before fetching anything.

- **PCMS commands** (`products`, `variants`, `brands`, `categories`, `product-types`,
  `units` — every verb, read and write) **require** `--tenant <code>`. The platform rejects a
  PCMS call with no tenant. This means catalogue lookups, barcode counters, and uniqueness
  checks are all scoped to a single tenant.
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

## Dispatch logic

When a request arrives:

1. **Read the intent** and classify it against *Scope at a glance*:
   - A specific product/SKU/variant, codes, barcodes, colours, model variants → `manage_product.md`.
   - A Brand named with create/rename/sync intent → `manage_brand.md`.
   - A Product Type / part-category named with create/rename intent → `manage_product_type.md`.
   - "sync", "drift", "does the file match Capigo" → `sync_check.md`.
   - Tasks, boards, categories, units, or a plain lookup → drive the CLI directly per `cli_basics.md`.
2. **Pick exactly one workflow.** If two seem to apply (e.g. create a product but its Brand
   is missing), start with `manage_product.md` — its steps already route out to the brand/type
   workflows when a dependency is missing, then return.
3. **Determine the tenant** (above) before any data fetch.
4. **Follow the chosen workflow document** — it carries the full propose → confirm → execute
   loop and its own edge cases.
5. **On any mid-workflow blocker** (missing Brand/Type, drift, input too sparse), pause and
   either route to the right helper workflow or ask one targeted question. Don't guess.

## Shared references

Workflows lean on these. Read them when a workflow points you there; if a workflow and a
reference disagree, the **reference wins** (and the OpenAPI doc wins over both — see
self-diagnose above).

- [`references/cli_basics.md`](./references/cli_basics.md) — every command, its flags, output
  modes, env vars, and the self-diagnosis recipe.
- [`references/coding_references.md`](./references/coding_references.md) — **source of truth**
  for Brand and Product Type `prefix` / `barcode_part`.
- [`references/brand_aliases.md`](./references/brand_aliases.md) — keyword → Brand mapping
  (iPhone → Apple, SLM → Salaman, …).
- [`references/brand_rules.md`](./references/brand_rules.md) — Brand decision tree (Zin → `Z`,
  Bóc Máy → `RF`, Độ → `D`, Phẩy → `B` condition).
- [`references/product_code_and_barcode.md`](./references/product_code_and_barcode.md) —
  canonical Product Code + Barcode structure and Vietnamese terminology.
- [`references/variants_and_options.md`](./references/variants_and_options.md) — Option
  naming, Variant abbreviations, Simple vs Variable products.
- [`references/barcode_algorithm.md`](./references/barcode_algorithm.md) — how to allocate the
  4-digit Variant Barcode Part (per-tenant) and compute the checksum.

## What this skill does NOT do

- Talk to Capigo over raw HTTP. Everything goes through `capigo`.
- Bulk migration of legacy catalogues, or one-shot import of a reviewed Markdown baseline —
  out of scope here; those are separate efforts.
- Change existing identifiers (`product.code`, `variant.code`, `variant.barcode`, Brand/Type
  `prefix`/`barcode_part`) unless the user explicitly asks and approves the exact change.
- Edit reference data outside this skill's own `references/` directory.
