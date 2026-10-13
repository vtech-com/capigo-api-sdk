# API coverage gaps

Living backlog of Capigo Public API (`/api/v1`) endpoints/actions the CLI cannot
yet wrap because the **server-side endpoint does not exist**, plus any endpoints
that exist but the CLI hasn't surfaced. Add to the SDK as endpoints land.

**Last assessed:** 2026-10-13, against the published spec and the production API. (The route-handler assessment below dates from 2026-06-04, against the `capigo` monorepo **`develop`** branch.)

## How this was determined

The authoritative source is the actual route handlers, not a spec file:
`apps/platform/src/app/api/v1/**/route.ts` (which `export async function GET/POST/PATCH/PUT/DELETE`).

> ⚠️ **Do not trust the checked-in `apps/platform/src/lib/api/openapi.json` for gap analysis.** On 2026-06-04 it was stale: it listed `/pcms/{brands,categories,product-types,units}/{id}` as `PUT`-only, but the real handlers expose `GET,PATCH,PUT` (and the SDK already wraps get/update/replace from the **prod** openapi). The SDK syncs from prod (`make update-spec` ← `https://platform.capigo.app/api/openapi`), which is accurate; the checked-in file is hand-maintained and drifts. **Recommendation to backend: regenerate `openapi.json` from the routes.**

## Spec drift observed via verify-api (2026-08-19)

`make verify-api` against prod (tenant `vtech-group`) reported the published spec still
incomplete versus the live server:

- `/pcms/variants` returns `extra_data`, `legacy_code`, and `manufacturer_code` that
  `api/openapi.json` never declares. This is the **spec's** defect — report it to the API
  team; do not hand-wrap these in the CLI.
- `/health` (200) and `/me` (404) are still absent from the spec, though the CLI calls both.

**Guard gap (not a spec defect):** `verify_api.py` reads only `responses["200"]`, and
`openapi_body_coverage_test.go` skips per-field checks when a command has `--from-json`.
A new request-body field (e.g. `status` on `PUT /pcms/products/{id}/variants`) can
therefore slip past both guards. The read side is covered; the write side is not — tracked
as a follow-up.

## SDK vs. existing API: essentially complete

The CLI wraps every endpoint that currently exists, with one exception:

| Endpoint (exists on develop) | SDK command | Status |
|---|---|---|
| `GET /health` | — | ✅ **being added** (`capigo health`) |
| `GET /pcms/products/{id}` | `capigo products get <id>` | ✅ **landed on develop / pre-staged in SDK** — UUID-only; reconciles on `make update-spec` after prod deploy |
| `PATCH /mission/tasks/{id}` | `capigo tasks update <id>` | ✅ **landed on develop / pre-staged in SDK** — UUID-only; reconciles on `make update-spec` after prod deploy |
| `GET /members/{id}` | `capigo members get <id>` | ✅ **landed on develop / pre-staged in SDK** — UUID-only; reconciles on `make update-spec` after prod deploy |
| `GET /pcms/variants/{id}` | `capigo variants get <id>` | ✅ **landed on develop / pre-staged in SDK** — UUID-only; reconciles on `make update-spec` after prod deploy |

Everything else under `/members`, `/mission/*`, `/pcms/*`, `/tenants`, `/wms/*` is wrapped.

**Board membership (2026-09-28):** the four `/mission/boards/{id}/members*` operations were the
only endpoints present in the platform's spec and absent from this repo's copy — the spec copy
predated them. They are wrapped now (`boards members list`/`add`/`update`/`remove`) and both
paths are in `api/openapi.json`, so nothing here is uncovered. `boards members add` sends the
batch `user_ids` array; the copy was updated by hand for these two paths only, and the next
`make update-spec` against a prod that carries them will reconcile the rest of the document's
formatting.

## API-level gaps — endpoints that do NOT exist yet

"Needed" depends on the Tấm skill scope (defined elsewhere). The clearly-justified
ones for a work-management agent are task update/delete and product get.

| Priority | Missing action | Notes / status |
|---|---|---|
| ~~High~~ | ~~`DELETE /mission/tasks/{id}` (task delete/cancel)~~ | Landed and wrapped as `tasks delete`. |
| Medium | Members: invite, role change, remove | `list` and `get` (UUID) are exposed. Member management (invite/RBAC/positions) exists in product docs but not in public `/api/v1`. |
| Low (likely intentional) | `DELETE` on brands/categories/product-types/units/products | No resource exposes delete. Probably deliberate for reference data. |

> Board and board-list create/update landed on prod, and are now wrapped — see the
> "Deliberately unwrapped" section below for what remains out.

## API-level gaps — answers the API withholds (boards, verified 2026-08-27)

These are not missing endpoints. The endpoints exist and work; what they return leaves the
caller unable to check its own write. Measured against prod with a throwaway board, not
inferred from the spec. The CLI cannot fix any of them — it passes `data` through unchanged,
which is the point — so each one is an ask on the API. Until they land, the bundled skill and
the `boards` help pages carry the caveat instead.

| Priority | Gap | Why it matters |
|---|---|---|
| High | No read returns a board list's `limit` | `--wip-limit` is write-only. The value can be sent and never read back, so a WIP limit cannot be confirmed, and a call that changed nothing is indistinguishable from one that did. **Ask:** include `limit` in the board-list response and in `lists[]` on `GET /mission/boards/{id}`. |
| High | Archiving a list makes it unreachable | `GET /mission/boards/{id}` drops archived lists from `.lists` and from `meta.list_count`, and no endpoint lists them. Only a caller that kept the list id can unarchive. **Ask:** an `include_archived` query param, and `is_archived` in the list response. |
| High | A private board 404s as "Board not found" | `is_public: false` makes the board unreadable and unwritable through `/api/v1`, and the visibility check precedes the update, so the flag cannot be turned back on. With no board delete endpoint, `--is-public=false` is a one-way door for any API caller — and the board cannot be cleaned up either: the web board list is filtered by board membership with no tenant-owner bypass (`findBoardsByUser`), so a board an API key made private is reachable only by that key's own user in the web app, or by a DBA. **Ask:** either let a board owner still read and update their own private board, or distinguish the 404 from a genuinely absent board. |

## The WMS module — wrapped (33 operations, 2026-10-13)

Held out from 2026-08-27 while its surface was still moving, the module is now wrapped whole under
`capigo wms`: nothing the spec declares is left in `unimplementedOps`. The signal the earlier note
waited for arrived — `internal-transfers` gained its `actions/{action}` route, so all four documents
carry create / preview / validate / update / actions — and the design pass it asked for is the
`wms` command tree, its help pages, and `skills/capigo-api/references/domains/wms.md`.

**What is deliberately not there.** The lifecycle route accepts only `submit` and `cancel`
(`internal-transfers`: `cancel` only). Every posting verb — receive, verify, reserve, pick, pack,
export, move, approve, reject, ship — answers 404 on the wire by design, so the CLI has no command
for them. This is an API boundary, not a CLI gap: do not add a generic `action <name>` passthrough to
"cover" it.

### API-level gaps — WMS answers that mislead (verified 2026-10-13, tenant `vtech-uat`)

Measured against prod with the dry-run endpoints, which write nothing. The CLI passes each answer
through and the help pages and skill carry the caveat; each one is an ask on the API.

| Priority | Gap | Why it matters |
|---|---|---|
| High | `validate` answers three different ways | Inbound receipts and internal transfers answer **200** with `{request_id, valid: false, errors[]}` and no `data` key. Outbound shipments answer the same refusal as a **404** `E9425`, and wrap their success as `{data: {valid}}`. Warehouse transfers answer `{kind, valid, checks[]}`. A caller cannot write one check for "is this body acceptable". **Ask:** one envelope — `{data: {valid, errors[]}}` — for all four. |
| High | Warehouse-transfer `validate` and `preview` do not resolve a line's SKU | A body naming a SKU that does not exist answers `valid: true`; the preview echoes the line with `product_variant_id: null`. Only `create` refuses it. `valid: true` therefore does not mean "create will accept this". **Ask:** resolve variants in the dry run, as the other three documents do. |
| Medium | A refused `preview` and a refused `validate` disagree | Inbound: `validate` says 200 `valid: false`, `preview` of the same body says 400 `VALIDATION_ERROR`. Workable, but the pair reads as one feature. |
| Medium | `GET /wms/warehouse-transfers?warehouse_code=KHO-TN` answers 500 `E0501` | Reproducible on `vtech-uat`; every other warehouse and every other KHO-TN list answers 200. A server defect, reported here because the CLI surfaces it as exit 1 with nothing to act on. |
| Low | A warehouse-transfer list row carries no `updated_at` | The value every write needs as `If-Match` is on the detail only, so a write always costs a `get` first. Inbound, outbound and internal-transfer rows carry it. |

## Board and list delete — withdrawn by the API (2026-10-13)

`DELETE /mission/boards/{id}` and `DELETE /mission/boards/{id}/lists/{listId}` left the published
spec, and the live routes no longer accept the method: an authenticated `OPTIONS` answers
`Allow: GET, HEAD, OPTIONS, PATCH` for a board and `Allow: OPTIONS, PATCH` for a list, while the
members route beside them still lists `DELETE`. The CLI had shipped `boards delete` and
`boards lists delete` in v0.26.0; both are removed, since a command that can only fail teaches the
agent a capability that is not there. The private-board row above already rests on it: no endpoint
deletes a board.

## Addressing resources by human key (code), not UUID — API requirement

**Decision (2026-06-04):** users and Tấm hand off / reference work by a **human key**
(e.g. task `TASK-123`, `tenant_code` `acme`), not by UUID. Single-resource operations
(`get`, and future `update`/`delete`) should be addressable by that key.

**Chosen strategy: API-side exact lookup (Strategy A).** The SDK will NOT do client-side
"resolve key → UUID via search". Instead the API must expose exact lookup by the human
key, and the SDK wraps it. Rationale: search-resolve is fuzzy (ilike), can match 0/many,
and isn't uniform (boards/variants have no `q`). Until the endpoint exists, the only
interim option is `tasks list -q "TASK-123"` (fuzzy, returns a list) — not shipped as a
`get` behaviour.

**Current state:** every single-item `GET` is **UUID-only**; no resource supports exact
lookup by its human key. This blocks the whole pattern.

**Canonical human key per resource** (unique keys only — these are the ones in scope):

| Resource | Human key | Unique | API lookup needed |
|---|---|---|---|
| tenant | `tenant_code` | ✅ | already the handle (`--tenant`) — done |
| **task** | `code` (TASK-123) | ✅ per tenant | **priority** — exact lookup by code (backend already has `findOneTaskByCode`). Apply to get + future update/delete. |
| member | `email` | ✅ | exact lookup by email (members has no get-by-id at all yet) |
| variant | `barcode` / `sku` | ✅ | exact lookup (today only `barcode_prefix` search, no exact get) |
| product | `slug` / `sku` | slug ✅ | fold into the `GET /pcms/products/{id}` work being added — also accept slug/sku |

**Deferred (not in scope now):** ref-data (brand / category / product-type / unit) and
**board** only have `name`, which is **not guaranteed unique** — clean code-lookup there
needs the API to add a `code`/`slug` field first. Shelved until there's a concrete need.

**SDK follow-up (per resource, once the API endpoint exists):** `capigo <resource> get <key>`
accepts the human key (and still UUID); same for update/delete. No SDK work until then.

**Note:** The four `get`-by-id commands added in this pre-stage (`products get`, `tasks update`,
`members get`, `variants get`) are **UUID-only** — they do not accept human keys. This section
remains pending until the API exposes exact lookup by code/key.

## When an endpoint lands

1. Backend implements the route handler in the monorepo + the endpoint reaches **prod**.
2. In this repo: `make update-spec` (pulls prod openapi).
3. The `cmd/openapi_path_coverage_test.go` guard will flag the new path → add the CLI command (mirror the closest existing command) and register it in the coverage guards.
