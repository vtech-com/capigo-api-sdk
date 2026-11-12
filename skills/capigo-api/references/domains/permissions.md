# Permissions and warehouse access

Read this before granting or revoking anything: a tenant permission key, a warehouse scope, or a
person's access to a warehouse. Leaf help is authoritative for flags and output shapes; this document
defines what the agent must not get wrong.

## Two separate levels

| Level     | What it is                                         | Read with                                        | Change with                                                     | Addressed by                  |
| --------- | -------------------------------------------------- | ------------------------------------------------ | --------------------------------------------------------------- | ----------------------------- |
| Tenant    | A key held across the whole tenant: `catalog_*`, `po:*`, `wms_admin` | no read command; see below                       | `capigo members permissions add/remove <user-id>`               | the member's **user id**      |
| Warehouse | A `scope.*` held in one warehouse only             | `capigo wms warehouses members list --warehouse <code>` | `capigo wms warehouses members add`, `scopes add/remove`, `remove` | the **warehouse membership id** |

The two levels do not touch each other. Changing a tenant key never changes a warehouse membership,
and the reverse. Pick the level from what the user asked for: "can manage the catalog" is a tenant
key; "can receive goods in WH-HN" is a warehouse scope in WH-HN.

`capigo permissions list` is the only source of valid keys. Use `--scope-type tenant` or
`--scope-type warehouse` to see one level, `--module catalog|po|wms` for one module. Never invent a
key, and never build one from its parts: keys are opaque.

## The two ids are different

- Tenant commands take the **user id**: `.data[].id` from `capigo members list`.
- `wms warehouses members scopes add|remove` and `wms warehouses members remove` take the
  **membership id**: `.data[].id` from `capigo wms warehouses members list`. That list prints
  `user_id` beside it; match on `user_id` to find the person's membership.
- Only `wms warehouses members add` takes a user id (`--user-id`), because the membership does not
  exist yet. Keep `.data.id` from its answer.

Passing a user id where a membership id belongs answers exit 4. Do not retry with other ids: list
the warehouse's members and match on `user_id`.

## Hierarchy: read `implies`, do not grant twice

Some keys include others (ADR-074 in the Capigo platform defines the warehouse scope hierarchy).
Every entry in `capigo permissions list` carries `implies`: every key it already includes,
transitively. Before granting, check whether something the person already holds implies the key; if
it does, say so and grant nothing.

What to know without looking it up:

- An owner holds everything. Tenant permission changes on an owner are refused (exit 8,
  `PERMISSIONS_NOT_APPLICABLE`); do not try to grant an owner anything.
- `wms_admin` is a tenant key that covers **every warehouse**. Someone who holds it needs no warehouse
  membership, and does not appear in a warehouse's member list for that reason. Granting `wms_admin`
  is a much larger change than granting `scope.admin` in one warehouse: confirm with the user which
  one they mean.
- `scope.admin` covers every scope in its own warehouse only.
- Every operational scope (`scope.inbound`, `scope.outbound`, …) already lets the person enter the
  warehouse and read what `scope.view` reads. Do not add `scope.view` beside it.
- The API stores only the scopes that are not implied by the others, so the `scopes` it answers may
  be shorter than what you sent. That is not a failure. Removing the last scope leaves `scope.view`.

## Doing it

1. Resolve the person to a user id with `capigo members list -q <name>`. If several match, ask.
2. Read what they hold now. For a warehouse, `wms warehouses members list`. For tenant keys there is
   no read: `members get` and `members list` do not carry permissions. The answer to
   `members permissions add|remove` is the only place the full set appears. If the user needs to know
   the current set before deciding, say it cannot be read through the CLI; do not send a write just
   to look.
3. Pick keys from `capigo permissions list` and drop any that something known to be held already
   implies.
4. Confirm the tenant, the person, the level (and warehouse), and the keys with the user unless they
   already stated exactly those. Revoking access and granting `wms_admin` always get a confirmation.
5. Write with the atomic commands. `members permissions add/remove` and
   `wms warehouses members scopes add/remove` change only the keys named. Do not use
   `members update --permission` to add one key: it replaces the whole set.
6. Report from the answer: `.data.permissions` for a tenant change, `.data.scopes` for a warehouse
   change.

Adding a key already held, or removing one not held, succeeds and writes nothing. One unknown key
refuses the whole call (exit 5) and the error names it; fix that key and send again. Only an active
tenant owner may change tenant keys (exit 3 otherwise). Warehouse memberships may be managed by an
owner, a `wms_admin` holder, or a `scope.admin` holder in that warehouse. Nobody may remove their own
warehouse membership (exit 3).
