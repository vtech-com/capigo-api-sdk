# Options and Variants

How to model a product's options and variants, and how to abbreviate variant attributes into
a Variant Code. The Product Code structure and length limits live in
`product_code_and_barcode.md`.

## Simple vs Variable

- A **Simple Product** has exactly one variant. Its `variant.code = product.code`, and the
  variant carries no extra suffix. Every shippable SKU is still a variant row, so even a
  simple product ends up with one variant in Capigo.
- A **Variable Product** has options (e.g. Colour, Size) and one variant per combination.

## Options

Use English dimension names. Common ones:

- `Color`
- `Model`
- `Quality`
- `Capacity`
- `Source`
- `Health_Grade`

A product has **at most 2 options** (`option1`, `option2`). If a category seems to need more,
either collapse correlated values into one composite option or split into multiple products.
When it's not obvious which, surface the choice in `open_questions`.

The backend does **not** auto-generate the Cartesian matrix: when you send `options`, you
must also send one `variant` per combination you want to exist.

### Combo values are atomic

When one SKU represents a combined value, treat the combo as a single option value, not two
variants:

- `Trắng-Đỏ` is one value, not two variants.
- `12Pro/12PRM/13Pro/13PRM` can be one `Model` value when it's one interchangeable SKU.

## Variant abbreviation rules

All abbreviations in Variant Codes are English — Brand, Product Type, Product segment, and
Variant suffix alike.

Colour suffixes (1 char by default; expand only on a collision within the same Product):

| Vietnamese | English | Abbrev |
|---|---|---|
| Đen | Black | `B` |
| Trắng | White | `W` |
| Đỏ | Red | `R` |
| Xanh Lá | Green | `G` |
| Xanh Dương | Blue | `BL` |
| Xanh Rêu | Forest/Moss Green | `FG` or `MG` |
| Vàng | Yellow | `Y` |
| Vàng Sa Mạc / Desert | Desert | `DS` |
| Hồng | Pink | `P` |
| Tím | Purple | `PU` |
| Cam | Orange | `O` |
| Xám | Gray | `GY` |
| Xám Tự Nhiên / Natural | Natural | `N` |
| Gold | Gold | `GD` |

Other attributes:

- **Capacity**: `128GB` → `128G`.
- **Battery health**: `BM80`, `BM90`, `BM100`.
- **Quality grade**: `DLC`, `DLT`, `SP` — keep alphanumeric as-is.
- **Combo values**: hyphen-join, e.g. `Trắng-Đỏ` → `W-R`.

Use the shortest suffix that is unique within the Product. The same suffix can mean different
things in different Products — that's fine, because uniqueness is only required within a
Product.

## Product segment is also English

Don't copy a Vietnamese-initial shorthand into the Product segment. Translate the Vietnamese
name to English first, then abbreviate the English.

| Vietnamese | Wrong (VN initials) | Better (English) |
|---|---|---|
| Đồ Lắp Vỏ | `DLV` | `CASEJIG` |
| Tuvit Vặn Ốc Toét | `TVOT` | `STRIPSD` |
| Keo Dán Cường Lực | `KDCL` | `GLUEGLASS` |
| Kìm Cắt Vành Camera | `KCVC` | `CAMRINGCUT` |

If a translation is genuinely uncertain, put it in `open_questions` rather than guessing.

## Variant Code

```
{Brand}-{Product_Type}-{Product}-{Variant}      (Variable)
{Brand}-{Product_Type}-{Product}                (Simple — equals the Product Code)
```

Rules:

- Target ≤ 15 chars; hard upper bound 18 chars (same limit as the Product Code).
- The full `variant.code` (sku) must be unique within the tenant.
- The variant suffix only needs to be unique within its Product.
- The Product segment may already carry a `Z`, `RF`, `D`, or `B` suffix from the Brand /
  condition rules in `brand_rules.md`.
- For Apple iPhone models, drop the `IP` prefix from the Product segment (`IP13PM` → `13PM`).

Examples:

- `VT-SW-6-B`
- `AP-BA-13PMZ-DLC-BM90`
- `AP-HS-13PMRF-B`

## Field mapping to Capigo

| Internal concept | Capigo field |
|---|---|
| Product Code (e.g. `AP-BA-13PM`) | `product.aliases[]` (append) |
| Variant Code (e.g. `AP-BA-13PM-B`) | `variant.sku` |
| Barcode (11 digits) | `variant.barcode` |

For a Simple Product, `variant.sku = product.code`. Do not rely on a product-level `sku` for
catalogue work — the Product Code belongs in `aliases[]`.

## Colour completeness for Apple products

When building out an Apple colour-based product and the user wants the full set, add only the
colours Apple actually shipped for that specific model over its lifetime (including mid-cycle
additions) — don't invent colours. If a name says a generic `Xanh` and you can't infer the
exact Apple colour, put it in `open_questions` instead of guessing a code.

## Product-specific applicability

Some parts don't exist for newer generations — don't fabricate impossible variants. For
example, iPhone 15 and later have no separately serviceable camera ring (`CR`); if a request
implies one, flag it in `open_questions`.

## Where context notes go

There is no free-text notes field on a variant. Capture non-obvious reasoning in the
proposal you present to the user:

- `assumptions` — health-grade ranges, aftermarket-maker→device mappings, combo-value
  semantics, pre-allocated colour sets.
- `open_questions` — ambiguous mapping, missing reference data, suspected misfiled request.
