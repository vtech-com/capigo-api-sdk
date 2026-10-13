# WMS domain relationships

Read this only when reading or writing warehouse data: warehouses, locations, availability, inbound
receipts, outbound shipments, internal transfers, or warehouse transfers. Leaf help is authoritative
for command flags and output shapes; this document defines what the agent must not get wrong.

## Addressing

- Everything under `capigo wms` is addressed by **code**, not by UUID, and every command needs a
  tenant.
- Everything except a warehouse itself lives **inside one warehouse**, so those commands also need
  `--warehouse <code>`. A document's code is unique within its warehouse, not within the tenant: the
  same code with a different `--warehouse` is a different document, or exit 4.
- Resolve the warehouse first with `capigo wms warehouses list`. Never guess a warehouse code from a
  document code, a name, or an earlier conversation.
- `wms warehouses list` shows only the warehouses the key's member holds a scope in. An empty list is
  a statement about the key's access, not about the tenant — do not report "this workspace has no
  warehouses".

## These commands plan; they do not move stock

- A key can create, edit, submit, and cancel a document. It **cannot** receive, verify, pick, pack,
  export, move, approve, or reject: those steps are done by people in the Capigo app, and no command
  performs them.
- So never tell the user stock was received, shipped, or moved because a command succeeded. A created
  document is a `DRAFT`; a submitted one is a plan the warehouse has not acted on yet. Read
  `.data.status` and report that word.
- If the user asks for a posting step ("mark it received", "approve the transfer"), say it is done in
  the Capigo app. Do not look for a workaround, and do not cancel and recreate a document to imitate
  one.

## Writing a document

1. Build the body as a JSON file; the body's fields are on the `create` page of that document. A line
   names its variant by `sku` or by `product_variant_id`.
2. Run `validate` (or `preview`) with the same file before `create` whenever the body was assembled
   from user text rather than copied from a previous read. Neither writes anything.
3. Run `create`. Pass `--idempotency-key` when the call might be retried: the same key and body replay
   the first document instead of creating a second, and `meta.replayed` is `true` when that happened.
4. Read `.data.code` and `.data.updated_at` from the answer. The code names the document from now on.

Confirm the warehouse, the lines, and the quantities with the user before `create`, `submit`, or
`cancel` unless they already stated exactly those. `cancel` is final — a cancelled document does not
come back.

### Reading a verdict

`validate` does not answer the same way for every document. Read the leaf help; the differences that
cause wrong reports are:

- **Inbound receipts and internal transfers:** the verdict is `.data.valid`, and the command **exits
  0 even when it is `false`**. An exit code of 0 is not "the body is fine". When it is false,
  `.data.errors[]` says why.
- **Outbound shipments:** a body that would be refused does not come back as `valid: false` — the
  command fails with the reason instead (for example exit 4 for an unknown variant).
- **Warehouse transfers:** `valid: true` covers the warehouses, the direction, and the quantities. It
  does **not** check that a line's SKU exists. Do not tell the user the SKUs were verified; only
  `create` resolves them.

### Changing a document: `--if-match`

- `update`, `submit`, and `cancel` take `--if-match <updated_at>`: the document's current `updated_at`,
  copied exactly as `get` prints it. Do not reformat it, round it, or reuse one from an earlier read
  after a write — every write moves it. An internal transfer's `cancel` is the one exception and takes
  none.
- Exit 8 with code `E0602` means someone changed the document after your read. Nothing was written.
  Read the document again, look at what changed, and only then retry with the new value. Do not loop.
- `update` is a merge patch: a field left out is untouched, and `null` clears `external_ref` or
  `notes`. Whether `lines` replaces the list or merges into it depends on the document and its status
  — read the `update` page before sending lines to anything that is no longer a `DRAFT`.

## Costs that are null

- On an inbound receipt and an outbound shipment, `unit_cost` and `total_value` can be `null` for two
  different reasons. Check `unit_cost_denied` / `total_value_denied` first: `true` means the key's
  member may not see costs. Only when the flag is `false` does `null` mean "no cost recorded yet".
- Never report a denied cost as zero or as missing data. Say the key is not allowed to see it.
- An outbound shipment has a cost only after it has posted. Every write answers its cost as `null`.

## Warehouse transfers

- `--warehouse` is the warehouse that **starts** the transfer. On a `PUSH` (the default) it is the
  source and the body names `destination_warehouse_code`. On a `PULL` it is the destination and the
  body names `source_warehouse_code`. Ask which the user means when they say "transfer between A and
  B" without saying who is asking.
- Once approved, a transfer generates an outbound shipment in the source warehouse and an inbound
  receipt in the destination. Read each with its own command and **that** warehouse's code — the
  transfer's `get` names both under `generated_outbound_shipment` and `generated_inbound_receipt`.
- Those two documents belong to the transfer. Cancelling one directly is refused (exit 8, `E9680`);
  cancel the transfer instead. The same code is returned for a document with `cancel_locked: true`
  when the caller is not its owner.
- On a transfer line, `qty_exported` and `qty_received` are `null` until the shipment is exported and
  the receipt verified. `null` means "not yet", never 0.

## Availability

- `wms availability check` never returns on-hand stock. It returns a `score` from 0 to 100 per
  warehouse per line, and a `promisable_qty` when the warehouse cannot cover the whole demand. Do not
  present either number as a stock level.
- `score: 100` with `promisable_qty: null` means the warehouse covers the demand in full.
- A row identifies its line only by `line_no`, the 1-based position of the `--line` in your own
  command. Keep the order you sent and map the answer back; the row does not repeat the SKU.
- `found: false` means the variant was not found, and its `warehouses` is empty. That is "unknown
  item", not "out of stock everywhere".

## Lists and counts

- A WMS list returns at most 50 rows per page. Read `meta.total` for the count and page with
  `--page` while `meta.has_more` is `true`; never count `.data[]` and report it as the total.
- A list is always one warehouse's. "All inbound receipts of the tenant" is one call per warehouse —
  say so rather than reporting one warehouse's total as the tenant's.
