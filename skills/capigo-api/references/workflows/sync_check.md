# Workflow — Sync Check (drift detection)

Use this to verify that `../coding_references.md` and Capigo agree — typically before a batch
of product operations, or after someone edited the file or changed data in the Capigo UI.

**This workflow never auto-fixes anything.** It reports drift; the user decides how to
resolve, then runs the matching [`manage_brand.md`](./manage_brand.md) /
[`manage_product_type.md`](./manage_product_type.md) workflow.

## What "in sync" means

For Brands and Product Types:

- Every row in `coding_references.md` has a matching Capigo entry with the same `name`.
- Every Capigo entry has a matching file row (so we know its `prefix` and `barcode_part`).
- Within `coding_references.md`, every `prefix` is unique and every `barcode_part` is unique.

Units and Categories aren't tracked in `coding_references.md`; this check ignores them.

## CLI commands used

Both require `--tenant` (PCMS is tenant-scoped).

| Purpose | Command |
|---|---|
| List Capigo brands | `capigo --tenant <code> brands list --limit 100 --output json` |
| List Capigo product types | `capigo --tenant <code> product-types list --limit 100 --output json` |

If there are more than ~20 entries, pass `--limit 100` or iterate `--page`.

## Procedure

### Step 1 — Determine tenant

Sync is tenant-scoped (Brands/Product Types are per-tenant in Capigo). Determine
`$TENANT_CODE` per **Tenant handling** in `../../SKILL.md`. With multiple tenants, ask which
to check, or check each in turn.

### Step 2 — Load both sides

**File side** — parse `coding_references.md`: Brand Mapping → `{name, prefix, barcode_part}`;
Product Type Mapping → `{name_vi, name_en, name_zh, prefix, barcode_part}`.

**Capigo side** — list both endpoints and parse `data[]` → `{id, name}`:

```bash
capigo --tenant "$TENANT_CODE" brands list --limit 100 --output json
capigo --tenant "$TENANT_CODE" product-types list --limit 100 --output json
```

### Step 3 — Compute drift

For each of brands and product types, compute four sets:

| Set | Definition | Resolution |
|---|---|---|
| **In sync** | Same `name` in file and Capigo | None |
| **File-only** | In file, no Capigo entry with that name | `manage_*.md` mode D (create on Capigo) |
| **Capigo-only** | On Capigo, no file row with that name | `manage_*.md` mode C (add to file with user-supplied prefix + barcode_part) |
| **Name mismatch** | Same identity but differing `name` (likely a rename on one side) | `manage_*.md` rename to reconcile |

Name matching is **case-sensitive exact**. Differences in capitalization or whitespace count
as mismatches — surface them so the user decides whether to normalize.

### Step 4 — Internal uniqueness check (file only)

Inside `coding_references.md`: two Brand rows sharing a `prefix` or `barcode_part` → a bug to
fix before any product op (same for Product Types). A Brand prefix colliding with a Product
Type prefix isn't illegal (different namespaces) but is worth flagging since it makes codes
harder to read.

### Step 5 — Report

One consolidated message. Vietnamese summary, English technical terms. Example:

```
Sync check (tenant: <tenant_name>)

Brands
  In sync:       42
  File-only:     2  → AKY (Aukey), VLN (Valieno)   [create on Capigo]
  Capigo-only:   1  → "ZTE"                          [add to file]
  Name mismatch: 1  → "Apple" (file) vs "Apple Inc" (Capigo)

Product Types
  In sync:       45
  File-only:     0
  Capigo-only:   0
  Name mismatch: 0

File internal checks
  Brand prefixes unique:        OK
  Brand barcode_parts unique:   OK
  Type prefixes unique:         OK
  Type barcode_parts unique:    OK
  Brand/Type prefix overlap:    none

Suggested actions:
  1. manage_brand.md (D): create AKY, VLN on Capigo
  2. manage_brand.md (C): add ZTE to coding_references.md (need prefix + barcode_part)
  3. manage_brand.md (B1): reconcile "Apple" / "Apple Inc"
```

Stop after reporting. Don't loop into a fix automatically — the user picks which actions to
run and in what order.

## When to run

- **Before** a bulk product-creation session, to catch missing prefixes early.
- **After** someone edits `coding_references.md` by hand.
- **After** a Brand or Product Type is added/edited in the Capigo UI (bypassing this skill).
- **Periodically**, as catalogue hygiene.

## Cross-tenant note

A Brand can exist in tenant A but not tenant B and still be "in sync with the file" from A's
perspective — sync is per-tenant. For a multi-tenant report, run the check per tenant and say
which tenant each finding applies to.

## What this workflow does NOT do

- Auto-create missing entries on either side.
- Edit `coding_references.md` directly.
- Fuzzy-match aggressively (only flag exact-name mismatches and obvious near-matches —
  aggressive fuzzy matching invites silent errors; defer to the user).
- Validate Product Code or Barcode integrity against existing products (a separate check, not
  defined here).
