# Workflow — Manage Product & Variant

Use this when the user describes one product/SKU/variant and wants to create it, add
variants, update metadata, or generate codes. It ends with a `capigo` write against the
chosen tenant. For brands see [`manage_brand.md`](./manage_brand.md); for product types see
[`manage_product_type.md`](./manage_product_type.md); for file-vs-Capigo drift see
[`sync_check.md`](./sync_check.md).

## Invariants specific to this workflow

These extend the global invariants in [`../../SKILL.md`](../../SKILL.md):

- **One Variant per Simple Product**, with `variant.sku = product.code`.
- **Preserve identifiers** on existing rows — don't change `product.code` (alias),
  `variant.sku`, or `variant.barcode` unless the user explicitly asks and you've verified the
  new value is free within the tenant.
- **New values must be unique within the tenant** — product alias, `variant.sku`, and
  `variant.barcode`. PCMS is tenant-scoped, so "unique" means within `$TENANT_CODE`.

## CLI commands used

All require `--tenant` (PCMS is tenant-scoped).

| Purpose | Command |
|---|---|
| Search products (name/variant/SKU/barcode) | `capigo --tenant <code> products list --query "<term>" --output json` |
| Full catalogue scan (alias check) | `capigo --tenant <code> products list --all --output json` |
| Barcode counter lookup | `capigo --tenant <code> variants list --barcode-prefix "<prefix>" --sort -barcode --limit 1 --output json` |
| Create product | `echo '<json>' \| capigo --tenant <code> products create --from-json -` |
| Update product | `echo '<json>' \| capigo --tenant <code> products update <id> --from-json -` |
| Upsert variants | `echo '<json-array>' \| capigo --tenant <code> products variants --product-id <id> --from-json -` |
| Verify after write | `capigo --tenant <code> products list --ids "<id>" --output json` (or `products get <id>` once its API is live — see cli_basics) |

## The 6-step workflow

Steps 1–4 are silent analysis; the user sees one consolidated proposal at step 5.

### Step 1 — Parse the request

Extract from the free-form text:

- **Core noun** (the part): "Pin", "Màn hình", "Tô vít", "Camera sau", "Vỏ".
- **Target device** if any: "iPhone 13 Pro Max", "Galaxy S22", "Universal".
- **Condition / source markers**: `Zin`, `Zin New`, `Bóc Máy`, `Chính Hãng`, `Độ`, `Chế`,
  `Phẩy`, `Spa`, `Keng`, `Ép Kính`, `Lên Vỏ`, aftermarket maker (JK, Salaman/SLM, GX…).
- **Variant attributes**: colour, capacity, model variants, battery health %, edition…
- **Quantity hint**: one SKU, or a matrix? A matrix → Variable Product with many Variants.
- **Operation intent**: create new, add variant, update metadata, or change code/barcode.

If the input is too sparse to identify either the part or the device, ask one targeted
question. Otherwise proceed silently. Then determine the tenant per **Tenant handling** in
`../../SKILL.md` before continuing.

### Step 2 — Classify Product Type

Match the core noun + function to an entry in `../coding_references.md`. Fetch the live UUID
map:

```bash
capigo --tenant "$TENANT_CODE" product-types list --output json
# or targeted:
capigo --tenant "$TENANT_CODE" product-types list --query "<type name>" --output json
```

Build a `name → id` map from `data[]`. **RT vs RE:** Repair Tools (`RT`) = hand-held; Repair
Equipment (`RE`) = bench-mounted / mains-powered.

If no existing Product Type fits, **stop and route to
[`manage_product_type.md`](./manage_product_type.md)** to add it first. Don't invent one.

### Step 3 — Classify Brand

Walk the decision tree in [`../brand_rules.md`](../brand_rules.md):

1. Genuine (`Zin`, `Chính Hãng`, sealed-new OEM) → Brand = device manufacturer, suffix `Z`.
2. Bóc Máy / pulled / refurbished → Brand = device manufacturer, suffix `RF`.
3. Độ / Chế → Brand = `NB` by default (or the evidenced maker), suffix `D`, target device in
   the Product segment.
4. Aftermarket maker keyword (JK, SLM/Salaman, GX…) → Brand = that maker, no suffix.
5. None → Brand = `NB` (No Brand), no suffix.

Verify the Brand exists in `../coding_references.md`, then fetch the UUID:

```bash
capigo --tenant "$TENANT_CODE" brands list --query "<brand name>" --output json
```

If the Brand is in `coding_references.md` but missing on Capigo (or vice versa), **stop and
route to [`manage_brand.md`](./manage_brand.md)** to resolve before continuing. Mixed signals
(e.g. "Pin Zin Bóc Máy 90%") → physical state wins → `RF`. **Phẩy** is a condition → suffix
`B`, separate Product from the clean version.

### Step 4 — Decide create / add / update

Run a candidate-match query first:

```bash
capigo --tenant "$TENANT_CODE" products list --query "<term>" --output json
# Full scan when matching by alias (= product.code), which --query does not index:
capigo --tenant "$TENANT_CODE" products list --all --output json
```

Use `--query` for name/variant/SKU/barcode. To match a Product Code (alias), fall back to
`--all` and filter `data[].aliases[]` locally — within the tenant.

Then triage:

1. **Update existing** — user names an existing code/barcode/row to edit. Fetch the exact row
   first, propose `UPDATE`, preserve identifiers.
2. **Add Variant to existing Product** — same base model + same suffix-class (`Z`/`RF`/`D`/`B`
   /none), differing only along an existing Option dimension. Add a Variant; don't make a new
   Product.
3. **New Variable Product** — input describes a matrix and nothing matches. One Product with
   inferred `option1`/`option2` and one Variant per cell.
4. **New Simple Product** — single SKU, no matrix, no parent. One Product + one Variant where
   `variant.sku = product.code`.

Never merge into an existing Product across a different Brand prefix, a different
`Z`/`RF`/`D`/`B` suffix class, or different physical interchangeability. When unsure, surface
options to the user at step 5 rather than picking silently.

### Step 5 — Draft codes, allocate Barcode, propose

**Product Code** — `{Brand}-{Product_Type}-{Product}` (≤ 18 chars total). Follow Product
segment rules in [`../product_code_and_barcode.md`](../product_code_and_barcode.md) and
[`../variants_and_options.md`](../variants_and_options.md): English only, drop `IP` for Apple
iPhones, no Brand repetition, append `Z`/`RF`/`D`/`B` per the Brand decision. If a draft is
too long, shorten by dropping vowels or abbreviating harder — never by appending digits.

**Variant Code(s)** — `{Product_Code}-{Variant}` (≤ 15 chars). For Simple Products,
`variant.sku = product.code`. Suffix rules (full table in `../variants_and_options.md`):
colours 1 char (expand only on collision within the Product); omit defaults; combo values
hyphen-joined; battery health `BM80/90/100`.

**Field mapping** (see `../variants_and_options.md`):

| Internal | Capigo field |
|---|---|
| `product.code` | `product.aliases[]` (append) |
| `variant.code` | `variant.sku` |
| `variant.barcode` | `variant.barcode` |

`products create` simple mode has no `--aliases` flag, so create via `--from-json` with an
`aliases` array (or set them afterward with `products update <id> --aliases …`).

**Uniqueness check** (within tenant) — before allocating barcodes, verify the product alias
and `variant.sku` don't already exist. Use `--query` for SKU; use `--all` + local
`aliases[]` filter for the product code. On an alias collision for a new Product, pick a
clearer English Product segment (don't append a digit). On a SKU collision, change the
variant suffix (the backend also enforces this — exit 8).

**Allocate Barcode** per [`../barcode_algorithm.md`](../barcode_algorithm.md): namespace
`"6" + brand_bc + type_bc`; fetch the max in that namespace with
`capigo --tenant "$TENANT_CODE" variants list --barcode-prefix … --sort -barcode --limit 1`;
new part = `MAX+1` (…`MAX+N` for N variants); base = 10 digits; checksum = sum of the 10
digits; re-check uniqueness before writing.

**Present the proposal** — one compact message. Vietnamese summary, English technical terms.
Include: (1) mode; (2) Brand + Product Type with prefixes and one-line evidence; (3) Product
Code with `name`/`type`/`option1`/`option2` if new; (4) variant rows/updates as a small
table (`sku | name | option1 | option2 | barcode` or `field | before | after`); (5)
`assumptions` for non-obvious decisions; (6) `open_questions` for anything needing the user's
judgment. End with **"OK to write to Capigo (tenant: {tenant_name})?"** and stop.

### Step 6 — Execute on confirmation

On clear confirmation ("ok", "đúng", "go", "insert", "update"):

**Create Simple Product**, then upsert its single variant:
```bash
echo '{"name":"…","brand_id":"…","category_id":"…","product_type_id":"…","unit_id":"…","status":"DRAFT","aliases":["<product.code>"]}' \
  | capigo --tenant "$TENANT_CODE" products create --from-json -
echo '[{"name":"…","sku":"<variant.code>","barcode":"<variant.barcode>"}]' \
  | capigo --tenant "$TENANT_CODE" products variants --product-id "<product-id>" --from-json -
```

**Create Variable Product** — single call with `options` + `variants` together (the backend
does not auto-generate the matrix):
```bash
echo '<json-with-options-and-variants>' | capigo --tenant "$TENANT_CODE" products create --from-json -
```

**Add Variant to existing**:
```bash
echo '<variants-json-array>' | capigo --tenant "$TENANT_CODE" products variants --product-id "<product-id>" --from-json -
```

**Update Product fields**:
```bash
echo '<update-json>' | capigo --tenant "$TENANT_CODE" products update "<product-id>" --from-json -
```

**Verify**: `capigo --tenant "$TENANT_CODE" products list --ids "<product-id>" --output json`
(read `.data[]`; or `products get <product-id>` once that endpoint is live on the tenant).
Report product ID, name, variant count, aliases. Stop.

If the user rejects or wants a tweak, loop back to the affected step — write nothing until a
fresh confirmation.

## JSON payload schemas

### `products create --from-json -` — Simple

```json
{
  "name": "Product name",
  "brand_id": "uuid",
  "category_id": "uuid",
  "product_type_id": "uuid",
  "unit_id": "uuid",
  "status": "DRAFT",
  "aliases": ["AP-BA-13PM"]
}
```
Then upsert 1 variant with `sku = product.code`.

### `products create --from-json -` — Variable

```json
{
  "name": "Product name",
  "brand_id": "uuid",
  "category_id": "uuid",
  "product_type_id": "uuid",
  "unit_id": "uuid",
  "status": "DRAFT",
  "aliases": ["AP-BA-13PM"],
  "options": [
    {"name": "Màu", "values": ["Đen", "Trắng"]},
    {"name": "Size", "values": ["S", "M"]}
  ],
  "variants": [
    {"name": "Đen / S", "option1": "Đen", "option2": "S", "sku": "AP-BA-13PM-BS", "barcode": "63400700011"},
    {"name": "Đen / M", "option1": "Đen", "option2": "M", "sku": "AP-BA-13PM-BM", "barcode": "63400700022"},
    {"name": "Trắng / S", "option1": "Trắng", "option2": "S", "sku": "AP-BA-13PM-WS", "barcode": "63400700033"},
    {"name": "Trắng / M", "option1": "Trắng", "option2": "M", "sku": "AP-BA-13PM-WM", "barcode": "63400700044"}
  ]
}
```

### `products variants --from-json -` (flat array)

```json
[
  {"name": "Đen / S", "option1": "Đen", "option2": "S", "sku": "AP-BA-13PM-BS", "barcode": "63400700011", "price": 150000}
]
```
An item **with** `variant_id` updates; **without** it creates (requires `name`). One call
upserts many at once.

## Edge cases

- **"Pin iPhone 13 SLM Zin"** — SLM is the maker, not Apple → Brand SLM, no `Z` suffix.
  Surface in `open_questions`.
- **"Phẩy" with no clean parent in Capigo** — still create the `B`-suffixed Product; don't
  auto-create the clean parent.
- **No Brand evidence, no aftermarket keyword** — Brand `NB`; note in `assumptions`.
- **Modded part with unknown target device** — stop and ask; a `D`-suffixed code needs the
  target device.
- **User gives only a loose name to edit** — `products list --query` to surface candidates
  (fall back to `--all` for alias); ask for the exact ID/SKU/barcode before updating.
- **SKU conflict (exit 8)** — change `variant.sku`, regenerate, re-propose before retrying.

## Out of scope

- Raw HTTP — always go through `capigo`.
- Bulk legacy migration and one-shot Markdown-baseline import.
- Creating Brands or Product Types — see `manage_brand.md` / `manage_product_type.md`.
- Drift checks — see `sync_check.md`.
- Changing existing `product.code` / `variant.sku` / `variant.barcode` unless explicitly
  requested and approved.
