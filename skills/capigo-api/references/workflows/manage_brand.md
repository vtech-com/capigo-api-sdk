# Workflow — Manage Brand

Use this to add a new Brand, rename one, update Brand metadata (e.g. `logo_url`), or resolve
drift between `../coding_references.md` and Capigo for one Brand. For catalogue-wide drift
detection, see [`sync_check.md`](./sync_check.md).

## Where Brand data lives

| Field | Source of truth | Notes |
|---|---|---|
| `name` | Capigo (`PublicBrandResponse.name`) | Display name. |
| `id` (uuid) | Capigo | Server-generated; never local. |
| `logo_url` | Capigo | Nullable. |
| `prefix` (e.g. `AP`) | `../coding_references.md` only | Capigo does not store it. Used in Product Codes. |
| `barcode_part` (2 digits) | `../coding_references.md` only | Capigo does not store it. Used in barcodes. |

`prefix` and `barcode_part` live only in the file, so **the file is the source of truth for
them**, and `name` must stay aligned with Capigo. The file's Brand list and Capigo's Brand
list must be kept consistent — divergence is a bug.

## Invariants

- **Globally unique `prefix`** in `coding_references.md` (case-sensitive).
- **Globally unique `barcode_part`** in `coding_references.md` — two Brands sharing one would
  collide an entire barcode namespace.
- **`prefix` / `barcode_part` are stable.** Once a Brand has products using its prefix, never
  change them — doing so orphans existing Product Codes and barcodes. Refuse by default (see
  B3).
- **Name on Capigo must equal name in the file.** If they diverge, route to `sync_check.md`
  before any product operation.
- **Never invent.** Always verify against the live Capigo list before assuming a Brand exists.

## CLI commands used

All require `--tenant` (Brands are tenant-scoped in Capigo). `update` is a PATCH (partial);
`replace` is a PUT (full). Both `create` and `update` accept either field flags or
`--from-json -`.

| Purpose | Command |
|---|---|
| List brands | `capigo --tenant <code> brands list --output json` |
| Search by name | `capigo --tenant <code> brands list --query "<name>" --output json` |
| Create | `capigo --tenant <code> brands create --name "<name>" [--logo-url <url>]` |
| Update (PATCH) | `capigo --tenant <code> brands update <id> --name "<new>"` |

> `brands create` / `update` / `replace` are stable as of CLI v0.5.0. If a call returns
> "unknown command", the installed CLI is older than expected — ask the user to update it.

## Operation modes

### A. Add a new Brand

1. **Confirm intent.** Is `X` already in `coding_references.md`? (If yes, this may be a sync,
   not an add — re-route.) Is `X` already on Capigo (`brands list --query "X"`)? (If yes, you
   only need the file row — scenario C.)
2. **Pick `prefix`** — a 2–3 char uppercase abbreviation of the name (Anker → `AK`, Aukey →
   `AKY`). Verify it doesn't collide with any existing prefix in `coding_references.md`.
3. **Pick `barcode_part`** — the smallest unused 2-digit code in `coding_references.md`,
   zero-padded.
4. **Pick `logo_url`** — default `null` unless supplied.
5. **Present proposal:**
   ```
   New Brand:
     name:         <X>
     prefix:       <PFX>   ← new, unique
     barcode_part: <NN>    ← new, unique
     logo_url:     <url or null>

   Actions:
     1. Add row to references/coding_references.md
     2. Create on Capigo (tenant: <tenant_name>)
   ```
   End with **"OK to apply?"** and stop.
6. **On confirmation:** edit `coding_references.md` (insert into the Brand Mapping table,
   keeping it sorted by `barcode_part`), then create on Capigo:
   ```bash
   capigo --tenant "$TENANT_CODE" brands create --name "X" --output json
   ```
   Capture the returned `id` and report it.
7. **Verify:** `capigo --tenant "$TENANT_CODE" brands list --query "X" --output json` — expect
   one row with the new `id`.

### B. Update an existing Brand

#### B1. Rename (`name`)

1. Fetch current (`brands list --query "<old>"`), capture `id`. Verify the file row exists.
2. Propose: `name: <old> → <new>` with `prefix`/`barcode_part` unchanged, plus the two
   actions (edit file name cell; update Capigo).
3. On confirmation: edit only the name cell in `coding_references.md`, then
   `capigo --tenant "$TENANT_CODE" brands update <id> --name "<new>"`.
4. Verify.

#### B2. Change `logo_url` (Capigo-only field)

Propose the single changing field; on confirmation update Capigo only
(`brands update <id> --logo-url <url>` or `--clear-logo`). `coding_references.md` doesn't
store this.

#### B3. Refuse: changing `prefix` or `barcode_part`

Refuse by default and explain: changing prefix/barcode_part orphans every existing Product
Code and barcode that uses this Brand, requiring a re-code of the affected catalogue. If the
user insists and confirms they understand, surface the affected products
(`products list --all` + filter aliases starting with the old prefix) and proceed only with
an explicit remediation plan. Treat this as out-of-band, not routine.

### C. Add to file when Capigo already has it

Detected by `sync_check.md` — Capigo lists Brand `Y` but the file has no row.

1. Ask the user for `prefix` and `barcode_part` (they can't be inferred — don't guess).
2. Verify uniqueness in the file.
3. Propose a file-only change (no Capigo write). On confirmation, edit `coding_references.md`.

### D. Create on Capigo when the file already has it

Detected by `sync_check.md` — file has Brand `Z`, Capigo doesn't.

1. `prefix`/`barcode_part` are already in the file; pull `name` from the row.
2. Propose creating on Capigo with that name. On confirmation:
   `capigo --tenant "$TENANT_CODE" brands create --name "<Z>"`. Verify.

## Tenant scope

Brand mutations require `--tenant`. Determine the tenant per **Tenant handling** in
`../../SKILL.md`. The same display name may exist under multiple tenants with different
UUIDs; operate on one tenant at a time, and note which tenant's UUID flows downstream.

## Open questions to surface (when relevant)

- **Conflicting prefix candidates** (e.g. "BSY" vs "BS") — list both options.
- **Brand UUID differs across tenants** for the same name — state which tenant's UUID is in
  use.
- **Near-name match on Capigo** ("Apple" vs "Apple Inc.") — stop and ask before assuming
  they're the same.
