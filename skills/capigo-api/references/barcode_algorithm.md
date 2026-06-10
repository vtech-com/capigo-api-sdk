# Variant Barcode Part — allocation algorithm

The full barcode (see `product_code_and_barcode.md`) is:

`6{Brand_Barcode_Part}{Product_Type_Barcode_Part}{Variant_Barcode_Part}{Check_Sum}`

`Brand_Barcode_Part` (2 digits) and `Product_Type_Barcode_Part` (3 digits) come from
`coding_references.md`. This file defines how to allocate the 4-digit `Variant_Barcode_Part`
by querying existing variants through `capigo variants list`.

## Scheme

- **Namespace:** each `(Brand, Product Type)` pair has its own counter, **within a tenant**.
- **Width:** 4 digits, zero-padded (`0001` … `9999`).
- **Allocation:** the next-available number in the namespace (highest existing + 1).
- **Stability:** if a `variant.sku` already exists, keep its current `variant.barcode`.
- **No reuse:** treat any barcode ever assigned as permanently allocated — don't reuse a
  number for a different variant even if the original was deleted.

Capacity per tenant: ~44 Brands × 40 Product Types × 9,999 variants — far above any realistic
catalogue.

## Tenant scope — important

`GET /pcms/variants` **requires** a tenant (`X-Tenant-Code`), so `capigo variants list` is
always scoped to one tenant. There is no cross-tenant variant read. Consequences:

- Always pass `--tenant <code>` when allocating barcodes.
- The barcode namespace counter, and barcode uniqueness, are **per-tenant**. A barcode is
  unique within the tenant's catalogue, not globally across all tenants.

(Earlier versions of this doc claimed a cross-tenant counter via a tenant-less call. That was
wrong for the current API — the endpoint rejects a tenant-less request.)

## Persistence model

Capigo's `variant` records are the allocation registry. Every persisted `variant.barcode`
embeds the namespace and the 4-digit allocation:

```text
6 BB TTT VVVV CHECKSUM
  ^^ ^^^ ^^^^
  |  |   Variant_Barcode_Part
  |  Product_Type_Barcode_Part
  Brand_Barcode_Part
```

## Allocation procedure

For each variant being inserted, within the chosen `$TENANT_CODE`:

1. Determine Brand and Product Type from the variant's planned code segments (or from its
   parent Product).
2. Look up Brand `Barcode Part` (2d) in `coding_references.md`.
3. Look up Product Type `barcode_part` (3d) in `coding_references.md`.
4. Build the namespace prefix: `"6" + brand_bc + type_bc` (6 digits total).
5. Fetch the highest existing barcode in that namespace:

   ```bash
   capigo --tenant "$TENANT_CODE" variants list \
     --barcode-prefix "6<brand_bc><type_bc>" \
     --sort -barcode \
     --limit 1 \
     --output json
   ```

6. Determine `MAX`:
   - `data` empty → `MAX = 0`.
   - Otherwise → take `data[0].barcode`, extract characters `[6:10]` (4 chars, 0-indexed,
     exclusive end) = the current `Variant_Barcode_Part`; parse as integer → `MAX`.

   Example: barcode `"63400700011"` → `[6:10]` = `"0001"` → `MAX = 1`.

7. Assign `Variant_Barcode_Part`:
   - Simple product / single new variant: `MAX + 1`, zero-padded to 4 digits.
   - Variable product with N new variants: `MAX + 1`, `MAX + 2`, …, `MAX + N`.
   - Adding variants to an existing product: only the new variants get fresh numbers;
     existing ones stay untouched.
8. Assemble the 10-digit base: `6 + Brand_BC(2d) + Type_BC(3d) + Variant_BC(4d)`.
9. Append `Check_Sum` = the sum of all 10 base digits as a plain integer (not zero-padded).
10. Re-check uniqueness before writing:

    ```bash
    capigo --tenant "$TENANT_CODE" variants list \
      --barcode-prefix "<computed_barcode>" --limit 1 --output json
    ```

    If `data` is non-empty, increment `Variant_Barcode_Part` and recompute. (Guards against a
    race between the MAX query and the write — rare but cheap to check.)

11. Write via `capigo --tenant "$TENANT_CODE" products variants --product-id <id> --from-json -`.

## Worked example

For Variant Code `AP-BA-13PMZ-DLC` (Apple, Battery Assembly) in tenant `acme`:

1. Brand Barcode Part: Apple = `34`.
2. Product Type Barcode Part: Battery Assembly = `007`.
3. Namespace prefix = `6` + `34` + `007` = `634007`.
4. Query `capigo --tenant acme variants list --barcode-prefix "634007" --sort -barcode --limit 1 --output json`.
5. Suppose `data` is empty → `MAX = 0` → `Variant_Barcode_Part = 0001`.
6. Base = `634007` + `0001` = `6340070001` (10 digits).
7. Checksum = `6+3+4+0+0+7+0+0+0+1` = `21`.
8. Final barcode = `634007000121`.

## SKU conflict (exit 8)

Barcode allocation is independent of SKU. If `products variants … --from-json -` returns exit
8 (SKU conflict), change the variant code (sku) before retrying; re-allocate the barcode only
if the new sku shifts the `(Brand, Product Type)` namespace (it normally does not).
