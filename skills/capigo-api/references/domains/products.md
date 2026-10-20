# Product domain relationships

Read this only when creating or changing products, variants, options, properties, or product
references. Leaf help is authoritative for command flags; this document defines the product model
the agent must preserve.

## Create flow

1. Resolve the tenant first. Every product, variant, and reference lookup is tenant-scoped.
2. Resolve a unit ID in that tenant — creation fails without one. Resolve brand, category, and
   product type IDs too when the request names them; they stay optional. Never guess an ID or
   create reference data merely to complete a product request.
3. Choose exactly one shape:
   - **Simple product:** use the simple fields. Capigo creates one default variant; its SKU,
     barcode, and price remain variant data even when supplied as product convenience fields.
   - **Variant product:** use `--from-json` with options and every intended variant explicitly.
4. Keep the default lifecycle state unless the user explicitly requests another state.

## Options and variants

- A product has at most **two** option axes — the API enforces this, it is not merely a convention.
  Use `options[0]` → `option1` and `options[1]` → `option2`. A third axis, or an `option3` on a
  variant, is rejected. Reads still return `option3` on variants written before the cap; that is
  legacy data, not a field you may write back.
- A variant object accepts only the keys the API names. Any other key — an `option3`, a misspelling
  — fails the whole request; it is never quietly dropped. So a rejected write is a claim about the
  *payload*, not evidence that the operation is unsupported: read the error, fix the key, resend.
- Every variant option value must be declared by its corresponding option. The caller supplies the
  intended combinations; the backend does not generate a matrix.
- Treat a stated option matrix as complete unless the user explicitly requests only selected
  combinations. Ask rather than silently omit or invent combinations.
- A SKU is unique within a tenant; an option combination is unique within its product. A barcode is
  not guaranteed unique. These rules do not define the tenant's code or duplicate-product policy.
- A variant carries `status`: `active` or `inactive` — the SKU's lifecycle state. It is distinct
  from the product's own lifecycle state and is not a soft delete; it defaults to `active`.

## Properties

`properties` holds a product's named attributes — origin, voltage, material — as a map from key
to `{label, value, position, definition_id}`. `{}` means none. Read `capigo products create --help`
and `capigo property-definitions --help` before writing either.

**Two kinds of key.** A key that matches one of the tenant's property definitions follows that
definition: it takes the definition's label, and `definition_id` names it. Any other key is ad hoc
to this one product and must carry its own `label`. Definitions exist so the same attribute keeps
one key across the catalog, which is what makes it comparable and filterable. So before adding a
property, run `capigo property-definitions list` and reuse the key of a definition that means the
same thing, even when the user words it differently — an ad-hoc `xuat_xu` beside a defined
`origin` splits one attribute in two. When nothing fits, write it ad hoc with a label. Do not
create a definition unless the user asks for a shared property: a definition is a tenant-wide
catalog-admin change, not a step of editing one product.

**Sending `properties` replaces the whole set.** On update, an omitted `properties` keeps what is
stored, `{}` clears it, and any other map becomes the complete set — a key you leave out is
deleted. To add or change one property:

1. `capigo products get <id>` and take `.data.properties`.
2. Add or edit the one entry; leave every other entry as read.
3. Send the whole map with `capigo products update <id> --from-json`.

Sending entries back exactly as read is safe: the label of a defined key and any `definition_id`
are ignored on write. `position` is an integer, unique within the map, and alone sets the display
order — the map's key order means nothing; give a new entry the highest position plus one.
`value` is always a string, 1–1000 characters: send `"220"`, not `220`.

After the write, compare `.data.properties` with what the user asked for and report the full
resulting set, not only the key you touched. A refused map writes nothing; read the error, fix the
named entry, resend.

**Definition changes reach every product.** Relabelling a definition relabels it on every product
that carries it. Deleting one never fails for being in use: each such product keeps the property,
now ad hoc, and the key is spent — keys never change and are never reused, deleted ones included.
Tell the user that reach before relabelling or deleting, and do not delete-and-recreate to "rename"
a key; it cannot work.

## Images

- Product and variant reads carry `media[]`: signed, time-limited URLs (`expiresAt`), not permanent
  links. Do not cache or hand out a `media[].url` for later use — re-read the product or variant to
  get a fresh one when the old one may have expired.

## Metadata and recovery

- Aliases are alternate names or codes. Tags are product-level labels shared by all variants; they
  are not variant attributes.
- Product reads carry `notes[]` — internal staff notes (`id`, `content`, `tag`, `author`,
  timestamps). Read-only: no write accepts or mutates them; `[]` means the product has no notes.
- After a write, check `error`, `meta.tenant`, `.data.options`, and `.data.variants` rather than
  assuming the submitted payload was stored unchanged. Use `capigo products variants --help` for
  later variant changes.
- Search before retrying an ambiguous failed write, or when the user explicitly requires
  uniqueness. Do not preflight-search by name merely to impose a no-duplicates policy.
