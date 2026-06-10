# Product Code & Barcode — canonical rules

The canonical structure and terminology for Product Codes and Barcodes. Allocation mechanics
for the numeric barcode live in `barcode_algorithm.md`; the Brand/Type coding values live in
`coding_references.md`.

## 1. Product Code rules

- **Human readable** — staff should be able to interpret it at a glance.
- **Maximum length: 18 characters** (hard limit).
- **Language**: strictly English abbreviations. Never Vietnamese initials.
- **Separator**: hyphens `-` between segments.
- **Hierarchy**: general → specific.

```
{Brand}-{Product_Type}-{Product}-{Variant}
```

For Simple Products the `{Variant}` segment is omitted; `variant.code = product.code`.

### Segment definitions

1. **`{Brand}`** — Brand prefix. See `coding_references.md` (Brand Mapping).
2. **`{Product_Type}`** — Product Type prefix. See `coding_references.md` (Product Type
   Mapping).
3. **`{Product}`** — concise English abbreviation of the product name. Aim for 2–4 chars that
   identify the model/series and minimise collisions with other products in the same
   `(Brand, Product Type)` namespace.
4. **`{Variant}`** — concise English abbreviation of the variant attributes (colour,
   capacity, size…). Join multiple attributes with a hyphen when it aids readability
   (`BLK-128G`). The suffix only needs to be unique **within its Product**.

**Example:** `VT-DS-IP13-OLED-BLK` (Vtech – Display – iPhone 13 – OLED – Black)

## 2. Barcode rules

- **Numeric only** — barcodes contain digits only, for reliable scanning.
- **Scannability** — optimised for thermal printing and handheld scanners.

### Barcode structure

```
6 {Brand_Barcode_Part} {Product_Type_Barcode_Part} {Variant_Barcode_Part} {Check_Sum}
```

| Segment | Width | Source |
|---|---|---|
| Leading `6` | 1 digit | Fixed prefix for this catalogue. |
| `Brand_Barcode_Part` | 2 digits | `coding_references.md` Brand Mapping. |
| `Product_Type_Barcode_Part` | 3 digits | `coding_references.md` Product Type Mapping. |
| `Variant_Barcode_Part` | 4 digits | Allocated MAX+1 within the `(Brand, Product Type)` namespace (per tenant) — see `barcode_algorithm.md`. |
| `Check_Sum` | 1 digit | Sum of the 10 preceding digits (see `barcode_algorithm.md`). |

Total: 11 digits.

## 3. Product terminology (Vietnamese)

These terms appear in product names and often drive Brand / Product / Variant decisions:

- **Zin** — genuine / manufacturer part (zin bóc máy, zin new chính hãng). A third-party
  screen (JK, GX, Salaman…) and a genuine zin screen (Apple/Samsung…) for the same device
  are two different Products under two different Brands.
- **Keng** — ~98–99% as good as new (đẹp keng).
- **Ek** — ép kính (glass-pressed screen).
- **Lv** — lên vỏ (housing replaced; other functions still work, the new housing just makes
  it look better).
- **Sản phẩm độ** — modded: reshaped so a lower model takes a higher model's form factor.
  Only valid for the same product's mod, not across different products.
- **Spa** — refurbished/serviced (repaired or had parts replaced).
- **Phẩy** — cosmetically flawed or more visibly used than a clean/keng unit. This is a
  **condition, not a Brand**; it becomes suffix `B` on the Product segment, producing a
  separate Product from the clean version when it needs separate stock tracking.
