# Creating Capigo products: simple and variants modes

Use this reference only when creating a product. First run:

```bash
capigo products create --help
```

The leaf help is authoritative here: it documents both request shapes (flag mode, where
`--sku`/`--barcode`/`--price` populate the default variant, and `--from-json` for a product with
options), the `option1`..`option3` positional mapping, and that the backend never auto-generates
the option matrix — every variant must be sent explicitly. This guide only adds the judgment and
workflow the help cannot give you.

## Before writing

1. Resolve the tenant and pass `--tenant <code>` explicitly when there is any doubt.
2. Search for the intended name, alias, SKU, or barcode with `products list` so an accidental
   duplicate can be discussed before writing.
3. Resolve referenced brand, category, product type, and unit IDs through their CLI groups.
   Never guess UUIDs.
4. Decide whether the product has one sellable form (simple mode) or needs an explicit option
   matrix (variants mode via `--from-json`).

Use the user's own organization policy for codes, barcodes, naming, and whether duplicates are
acceptable — this skill does not invent that policy.

## Before sending a variants-mode payload

Validate that the requested variant combinations are complete and not accidentally missing or
duplicated. Neither the CLI nor the backend will catch a combination you forgot or sent twice.

## Verify the result

1. Check that stdout has no `error` key.
2. Confirm `meta.tenant` is the intended tenant.
3. Inspect `.data.variants` and `.data.options` rather than assuming the submitted payload was
   stored unchanged.
4. If subsequent variant changes are needed, discover and use `capigo products variants --help`;
   do not guess a command under the read-only `variants` group.

If product creation fails after a network interruption, search for the product before retrying.
The write may have committed even if the response did not reach the client.
