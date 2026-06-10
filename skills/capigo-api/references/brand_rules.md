# Brand assignment decision tree

Derive the Brand for a product from the **textual evidence** in the request: the product
name, alternate names, category words, and any description. When the evidence doesn't support
a specific Brand, the answer is `No Brand` — decide and move on.

## Source of truth

- **`brand_aliases.md`** — maps keywords (iPhone, Galaxy, Mi, Redmi, SLM, …) to Brands. Read
  it every time; the table may grow.
- **`coding_references.md`** — the authoritative list of valid Brand entries (Apple, Samsung,
  Salaman, Vtech, No Brand, …). A Brand assignment must point to an entry here, never to
  anything invented.

## Decision tree

Walk these checks in order:

### 1. Is the part **genuine** (Zin / hàng hãng / chính hãng / new hãng) or **Bóc Máy** (pulled/refurbished OEM)?

**Genuine-new indicators** (→ suffix `Z`): `Zin`, `Zin New`, `Zin New Chính Hãng`,
`Zin Foxconn`, `Hàng Hãng`, `Chính Hãng`, `New Hãng`, `OEM` (when paired with "new"/"chính
hãng"). These all describe a sealed-new genuine part from the device manufacturer → `Z`.

**Bóc Máy indicators** (→ suffix `RF`): `Bóc Máy`, `BM`, `Pull`, `Pulled`, `Zin Bóc Máy`,
`Refurbished`, `Tái Chế`, `Recycled`, plus any "Bóc Máy NN%" health-grade pattern.

If a request carries both kinds of wording (e.g. "Pin Zin Bóc Máy 90%"), the **physical
state** governs: pulled from a device = `RF`. "Zin New / Hàng Hãng / Chính Hãng" with no
pull/refurb signal = `Z`.

- **Yes → Brand = device manufacturer**, matched against `brand_aliases.md` (e.g. `iPhone` →
  Apple; `Galaxy`/`Note`/`A`-series → Samsung).
  - Zin New / sealed-new genuine → append suffix **`Z`** to the Product segment of the
    Product Code: `AP-BA-13PMZ`.
  - Bóc Máy / refurbished / pulled → append suffix **`RF`**: `AP-BA-13PMRF`.
  - Zin and Bóc Máy of the same model are **two separate Products**, not variants of one
    another. Their colour/capacity variants live inside each respective Product.

### 2. Is the part a **độ / chế** (modded) item?

Indicators: name contains `Độ`, `Chế`, or a "Model X độ thành Model Y" pattern (e.g. "Phím
Độ XSM lên 15 Pro Max").

- **Yes → Brand = whatever the request evidences for the *target* device** (often `No Brand`,
  since modded parts are usually generic aftermarket).
- Append suffix **`D`** to the Product segment, with the *target* device in that segment:
  `NB-BT-15PMD`.
- Modded products are **always separate Products** — never grouped with genuine or regular
  aftermarket.

### 3. Is there an **aftermarket maker** keyword in the name?

Check the text against `brand_aliases.md` for aftermarket-maker aliases (`SLM` → Salaman,
`JK`, `GX`, `Vtech` as a literal keyword, etc.).

- **Yes → Brand = that aftermarket maker.** No suffix on the Product segment.
- The device model (iPhone 13, Galaxy S22) is a **compatibility attribute**, not the Brand —
  it lives in the Product segment.

### 4. None of the above

- **Brand = `No Brand` (`NB`).** No suffix. Note in `assumptions` that there was no Zin/RF
  indicator and no aftermarket-maker keyword, so it falls back to No Brand by rule.

## What is NOT a Brand

These are conditions, sources, processing states, or quality grades — never Brands, and they
never affect Brand assignment:

`Spa`, `Đẹp`, `Keng`, `Phẩy`, `A+`, `DLT`, `DLC`, `Super`, `Bản Âu`, `EU`, `CNC`, `Ép Kính`,
`Lên Vỏ`, `IC Fix`, `Hàn Sau`.

They belong in the Variant segment or in notes. (`Phẩy` specifically becomes suffix `B` on the
Product segment — see `product_code_and_barcode.md`.)

## Same model, different Brand → different Products

A Zin (Apple) screen and a JK aftermarket screen for the same iPhone are **two separate
Products** — never merge. Likewise:

- `AP-DS-13Z` (Apple Zin display) vs `NB-DS-13` (No Brand aftermarket display for iPhone 13).
- `AP-BA-15Z` vs `AP-BA-15RF` (Zin New vs Bóc Máy of the same model, both Apple).

## Combination examples

- `Pin iPhone 13 Pro Max Zin New` → Brand Apple, Product `13PMZ` → `AP-BA-13PMZ`.
- `Pin iPhone 13 Pro Max Bóc Máy 90%` → Brand Apple, Product `13PMRF` → `AP-BA-13PMRF`
  (variant carries `BM90`).
- `Pin iPhone 13 Pro Max Salaman` → Brand Salaman → `SLM-BA-13PM`.
- `Pin iPhone 13 Pro Max` (no other indicator) → Brand `No Brand` → `NB-BA-13PM`. Surface in
  `open_questions` to confirm the intended Brand.
- `Phím Độ XSM lên 15 Pro Max` → Brand `No Brand`, target device 15PM, suffix `D` →
  `NB-BT-15PMD`.
- `Cảm ứng JK iPhone 13` → if `JK` is in `coding_references.md`, Brand JK → `JK-DG-13`.

## Ambiguity → ask, do not guess

If steps 1–4 leave you between two Brands, **do not pick one to keep moving**. Surface it in
`open_questions` with the candidate Brands, the evidence for each, and a recommendation if
you have one.
