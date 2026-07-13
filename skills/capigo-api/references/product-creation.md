# Creating Capigo products: simple and variants modes

Use this reference only when creating a product. First run:

```bash
capigo products create --help
```

The leaf help is authoritative for flags and limits. This guide records catalogue invariants; do
not invent the tenant's code, barcode, naming, approval, or duplicate-product policy.

## Apply catalogue constraints

- Resolve the tenant and every brand, category, product type, and unit ID within it. Never guess a
  UUID or create reference data merely to complete a product request.
- Choose one shape: a simple product has one default variant populated by the simple fields; a
  product with options uses `--from-json` and sends every intended variant explicitly.
- In variants mode, `option1` through `option3` map positionally to `options[]`. Use only declared
  option values, and validate the requested combinations are complete and unique before sending:
  the backend does not generate the matrix for the caller.
- A SKU is unique within a tenant and an option combination is unique within a product. A barcode
  is not guaranteed unique. Let the user's policy decide what codes to supply.
- Keep the platform default status unless the user explicitly requests a lifecycle state. Aliases
  are alternate names or codes; tags are product-level labels, not variant attributes.

Do not preflight-search by name merely to impose a no-duplicates policy. Search before retrying an
ambiguous failed write, or when the user explicitly requires uniqueness.

## Verify the result

1. Check that stdout has no `error` key.
2. Confirm `meta.tenant` is the intended tenant.
3. Inspect `.data.variants` and `.data.options` rather than assuming the submitted payload was
   stored unchanged.
4. If subsequent variant changes are needed, discover and use `capigo products variants --help`;
   do not guess a command under the read-only `variants` group.

After a network interruption, search before retrying: the write may have committed even though its
response did not reach the client.
