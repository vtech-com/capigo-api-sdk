# Product domain relationships

Read this only when creating or changing products, variants, options, or product references. Leaf
help is authoritative for command flags; this document defines the product model the agent must
preserve.

## Create flow

1. Resolve the tenant first. Every product, variant, and reference lookup is tenant-scoped.
2. Resolve optional brand, category, product type, and unit IDs in that same tenant. Never guess an
   ID or create reference data merely to complete a product request.
3. Choose exactly one shape:
   - **Simple product:** use the simple fields. Capigo creates one default variant; its SKU,
     barcode, and price remain variant data even when supplied as product convenience fields.
   - **Variant product:** use `--from-json` with options and every intended variant explicitly.
4. Keep the default lifecycle state unless the user explicitly requests another state.

## Options and variants

- This skill's supported catalogue model has at most **two** option axes. Use `options[0]` →
  `option1` and `options[1]` → `option2`.
- Every variant option value must be declared by its corresponding option. The caller supplies the
  intended combinations; the backend does not generate a matrix.
- Treat a stated option matrix as complete unless the user explicitly requests only selected
  combinations. Ask rather than silently omit or invent combinations.
- A SKU is unique within a tenant; an option combination is unique within its product. A barcode is
  not guaranteed unique. These rules do not define the tenant's code or duplicate-product policy.

## Metadata and recovery

- Aliases are alternate names or codes. Tags are product-level labels shared by all variants; they
  are not variant attributes.
- After a write, check `error`, `meta.tenant`, `.data.options`, and `.data.variants` rather than
  assuming the submitted payload was stored unchanged. Use `capigo products variants --help` for
  later variant changes.
- Search before retrying an ambiguous failed write, or when the user explicitly requires
  uniqueness. Do not preflight-search by name merely to impose a no-duplicates policy.
