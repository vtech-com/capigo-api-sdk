# Workflow — Manage Product Type

Use this to add a new Product Type, rename one, or resolve drift between
`../coding_references.md` and Capigo for one type. For catalogue-wide drift, see
[`sync_check.md`](./sync_check.md); for Brands, [`manage_brand.md`](./manage_brand.md).

## Where Product Type data lives

| Field | Source of truth | Notes |
|---|---|---|
| `name` | Capigo (`PublicProductTypeResponse.name`) | Display name; Capigo stores only this + `id`. |
| `id` (uuid) | Capigo | Server-generated. |
| `prefix` (e.g. `BA`) | `../coding_references.md` only | Used in Product Codes. |
| `barcode_part` (3 digits) | `../coding_references.md` only | Used in barcodes. |
| English / Chinese name | `../coding_references.md` only | Optional translation columns. |

`prefix` and `barcode_part` live only in the file, so **the file is the source of truth for
them**.

## Invariants

- **Globally unique `prefix`** in `coding_references.md`.
- **Globally unique `barcode_part`** (3 digits; `'000` reserved for `Unclassified`).
- **`prefix` / `barcode_part` are stable** once products use them — refuse changes by default
  (same policy as `manage_brand.md` B3).
- **Name on Capigo must equal the canonical file name.** Resolve drift before product ops.
- **Never invent** — verify against the live Capigo list first.

## CLI commands used

All require `--tenant`. `update` is PATCH; `replace` is PUT. Both `create` and `update` take
field flags or `--from-json -`.

| Purpose | Command |
|---|---|
| List product types | `capigo --tenant <code> product-types list --output json` |
| Search by name | `capigo --tenant <code> product-types list --query "<name>" --output json` |
| Create | `capigo --tenant <code> product-types create --name "<name>"` |
| Update (PATCH) | `capigo --tenant <code> product-types update <id> --name "<new>"` |

> Stable as of CLI v0.5.0. If a call returns "unknown command", the installed CLI is older
> than expected — ask the user to update it.

## Operation modes

### A. Add a new Product Type

1. **Confirm intent.** Is it already in `coding_references.md` under another label (many parts
   already have a type, e.g. "Sạc dự phòng" → `Power Bank / PB`)? Already on Capigo
   (`product-types list --query "<name>"`)?
2. **Decide name.** Capigo's `name` is the Vietnamese label per current convention (e.g. "Pin
   Liền Cáp", "Màn Hình"). Optional English/Chinese names are file-only.
3. **Pick `prefix`** — 2–3 char uppercase from the English name (Battery Assembly → `BA`,
   Charging Port → `CP`). Verify uniqueness. **RT vs RE:** hand-held → `RT`; bench-mounted /
   mains-powered → `RE`. Don't introduce a new prefix when one of these fits.
4. **Pick `barcode_part`** — smallest unused 3-digit code, zero-padded.
5. **Present proposal:**
   ```
   New Product Type:
     name (vi):    <Vietnamese label>
     name (en):    <English label>   ← file-only
     name (zh):    <Chinese label>   ← file-only, optional
     prefix:       <PFX>             ← new, unique
     barcode_part: '<NNN>            ← new, unique

   Actions:
     1. Add row to references/coding_references.md
     2. Create on Capigo (tenant: <tenant_name>) with name = "<Vietnamese label>"
   ```
   End with **"OK to apply?"** and stop.
6. **On confirmation:** edit `coding_references.md` (insert into the Product Type Mapping
   table, sorted by `barcode_part`), then
   `capigo --tenant "$TENANT_CODE" product-types create --name "<Vietnamese label>"`. Capture
   and report the `id`.
7. **Verify:** `product-types list --query "<Vietnamese label>" --output json`.

### B. Update an existing Product Type

#### B1. Rename (`name`)

Fetch current, capture `id`, verify the file row exists. Propose `name: <old> → <new>` with
`prefix`/`barcode_part` unchanged. On confirmation: edit only the name cell in the file, then
`product-types update <id> --name "<new>"`. Verify.

#### B2. Refuse: changing `prefix` or `barcode_part`

Same policy as `manage_brand.md` B3. Refuse by default; surface affected products if the user
insists.

### C. Add to file when Capigo already has it

Capigo has type `Y` but the file doesn't. Ask the user for `prefix` and `barcode_part` (can't
be inferred), verify uniqueness, propose a file-only change, then edit `coding_references.md`.

### D. Create on Capigo when the file already has it

File has type `Z`, Capigo doesn't. Use the canonical Vietnamese name from the row, propose,
then `product-types create --name "<Z>"`. Verify.

## Special types

- **`Unclassified` (`UN`, `'000`)** — reserved fallback. Never create products against it
  without explicit user confirmation; the right move for an unclassifiable part is usually to
  add the missing type via this workflow.
- **`Repair Tools` (`RT`, `'036`) and `Repair Equipment` (`RE`, `'035`)** — keep the
  hand-held vs bench-mounted distinction; don't merge them.

## Open questions to surface (when relevant)

- **Prefix candidate clashes** — multiple short forms collide with existing prefixes; list
  candidates.
- **vi/en/zh mismatch** — a label that doesn't translate cleanly; surface and ask.
- **"New type, or maps to an existing one?"** — when the part could fit an existing type,
  default to suggesting the existing type and confirm the user really wants a new entry.
