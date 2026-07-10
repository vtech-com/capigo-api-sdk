# Stable `capigo` CLI conventions

Read this after `../SKILL.md`. These are cross-command contracts that are useful even as the
command surface expands. For the exact command, flags, accepted values, and response fields,
always read the installed binary's help.

## Command and flag discovery

See `../SKILL.md` section 2 for the three-level discovery workflow (`capigo --help`,
`capigo <group> --help`, `capigo <group> <command> --help`).

Do not translate API query-parameter names directly into CLI flags. The CLI may expose a clearer
name, validate the value, combine several API fields, or intentionally omit a parameter.

## Queries and filters

- Use `--query` or `-q` for text search only when the leaf help exposes it.
- Use typed filter flags such as `--status`, `--priority`, or date bounds exactly as documented
  by that command. Do not invent a generic `--filter` or append raw `?filters[...]` syntax.
- Quote values containing spaces or shell metacharacters. For example, pass `--status "To-Do"`
  and quote human text.
- Preserve the documented casing and format for enums, UUIDs, dates, timestamps, comma-separated
  lists, and literal sentinel values such as `null`.
- Combine filters only when the command help permits their combination. Respect mutually
  exclusive forms shown in `USAGE`.

Search narrows a result; it does not automatically make a page complete. Inspect pagination
metadata before concluding that a record does not exist.

## Tenant resolution

Run `capigo help tenancy` instead of memorizing which future groups require a tenant.

`--tenant` is a leaf-command flag, not a root-global flag. A configured or environment tenant
may be used when the flag is omitted. On every write, read `meta.tenant` and
`meta.tenant_source`; a successful write can still land in a different tenant than intended.

When a read spans tenants, do not attribute a returned record to a specific tenant unless the
response actually identifies it.

## Output contract

Run `capigo help output` for the contract of the installed build. Current builds write one JSON
document to stdout:

```json
{ "data": [], "meta": {} }
```

- Lists place an array at `.data`; single-resource reads and writes place an object there.
- Pagination and CLI-added context live under `.meta`.
- There is no output-mode flag in current builds; do not add `--output` or `-o` from memory.
- stdout is the machine-readable stream. stderr carries a short error summary and optional
  verbose tracing.

Parse stdout as JSON before acting. Avoid brittle text parsing and never strip an assumed prefix
line.

## Errors and partial results

A failure still writes JSON to stdout:

```json
{ "error": { "code": "...", "message": "...", "next": "...", "request_id": "..." } }
```

The error object can carry more fields than shown above — `meaning`, `capability_note`, `raw`,
and `http_status` — depending on the failure; `capigo help exit-codes` documents the full set.

Some operations can return real rows and an error in the same document:

```json
{ "error": { "code": "..." }, "data": [], "meta": {} }
```

Therefore, check for `.error` before treating `.data` as complete. Preserve useful rows, but
label them partial and do not present them as the whole answer.

Branch on the process exit code or the error object, not on changing prose. Use
`capigo help exit-codes` for the current mapping and recovery advice. Surface `request_id` when
escalating an unexplained server error.

## Pagination and counting

List calls are paginated unless their leaf help says otherwise.

- Read `meta.total` for a full count; never count only the visible `.data[]` page.
- Continue while `meta.has_more` is true when a complete set is required.
- Use `--page`, `--limit`, or `--all` only if the specific command exposes them.
- If an auto-pagination call returns an error with rows, treat those rows as a truthful prefix,
  not a complete collection.

Prefer a server-side search or typed filter over fetching an entire large collection. Fall back
to exhaustive paging only when the answer truly requires it.

To answer "how many X are there?", request a 1-row page and read the total instead of fetching
everything:

```bash
capigo <group> list --tenant <code> --limit 1 | jq '.meta.total'
```

## Structured JSON input

When leaf help exposes `--from-json <path|->`, use it for nested objects, arrays, or payloads
that are clearer and safer as JSON than as many flags.

- `-` means stdin; a path means a file.
- Validate the JSON shape before sending it.
- Include only fields the user intends to change.
- Omitted and explicit `null` fields can have different meanings; read the leaf help.
- Check whether `--from-json` is mutually exclusive with field flags or takes precedence over
  them. This differs between commands.

Do not place credentials in JSON payloads, examples, logs, or shell history.

## Write safety

- Treat an explicit user request containing the intended resource, mutation, and tenant as
  authorization for that scoped write. Ask when any of those are materially ambiguous.
- Search or read the target before an update when identity or current state matters.
- Do not silently change stable identifiers or expand a single requested write into bulk work.
- After success, verify `.data` and `meta.tenant`.
- After an ambiguous network failure, read the target before retrying; the server may have
  committed the first attempt even though the response was lost.
