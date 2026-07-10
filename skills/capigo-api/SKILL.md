---
name: capigo-api
description: >
  Drive the Capigo platform through the installed `capigo` CLI — the entry point for any
  Capigo work an agent does for a tenant. Use this whenever a request maps to Capigo data
  or capabilities — tasks, boards, members, products, variants, brands, categories, product
  types, units, tenants, authentication, configuration — even when the user never names the
  CLI or Capigo itself, and in whatever language they ask: "create a task and assign it to
  someone", "what's on the board this week", "read the comments on that task", "list
  products", "look up this SKU or barcode", "how many brands do we have". Also the
  reference for how the CLI itself works: detecting the install, logging in, picking a
  tenant, exit codes, JSON output, and self-diagnosing a misbehaving command. If a request
  touches Capigo and you are unsure how to talk to it, reach for this skill first instead
  of guessing raw API calls.
---

# Capigo CLI operator

Use `capigo` as the normal interface to Capigo. The installed binary is self-documenting and
the Public API will continue to grow, so this skill teaches a discovery workflow rather than
duplicating the command tree or API reference.

## 1. Preflight the environment and authentication

Start each new environment or session by resolving the executable with the shell's native lookup
(`command -v capigo` in a POSIX shell or `Get-Command capigo` in PowerShell), then run:

```bash
capigo version
```

Use the resolved executable consistently for version, help, preflight, and execution; mixing help
from one build with commands sent through another produces false flag and capability errors.

If the shell cannot find `capigo`:

1. Inspect the OS, architecture, shell, and available package managers using read-only checks.
2. Tell the user that the Capigo CLI is not installed and recommend the official installation
   guide: <https://github.com/vtech-com/capigo-api-sdk#installation>.
3. Prefer the installation path that fits the detected machine. For example, when Homebrew is
   present, suggest `brew install --cask vtech-com/tap/capigo`; the guide also covers GitHub
   Releases and Go-based installation with their platform-specific caveats.
4. Do not install software or change the machine merely because the binary is missing. Wait
   for the user's authorization, then verify the result with `capigo version`.

After the version check succeeds, run the read-only authentication and connectivity probe:

```bash
capigo health
```

Interpret it by exit code and its stdout `error` object:

- Exit `0`: the API is reachable and the configured key was accepted. Continue.
- Exit `2`: the key is missing, malformed, or rejected. Run `capigo auth login --help`, tell the
  user to create or retrieve their key while signed in at <https://platform.capigo.app>, and ask
  them to run the supported login command locally. Then retry `capigo health`.
- Exit `6`: connectivity failed. Diagnose DNS, network access, or the configured API URL; do not
  mislabel it as an authentication problem or ask the user to log in again.
- Any other non-zero exit: read the error object and `capigo help exit-codes` before recommending
  a recovery step.

Do not use `capigo auth whoami` as a preflight. Its `/me` route is not implemented, so a 404 does
not say whether the configured key is valid.

Authentication secrets belong to the user. Never ask them to paste an API key into chat, inspect
credential files or environment variables to recover it, print it, place it in examples, or expose
it through shell tracing. Guide the user through `capigo auth login --help` and let them enter the
key locally through the supported flow.

## 2. Discover the live command surface

Once `capigo version` succeeds, use built-in help progressively:

```bash
capigo --help
capigo <group> --help
capigo <group> <command> --help
```

Read only the group and command relevant to the user's intent. A runnable command page includes
its purpose, usage, flags, constraints, examples, and output. Do not reconstruct a command from
memory, guess a flag, or treat this skill as a command inventory.

Pull cross-cutting contracts from the binary when they matter:

```bash
capigo help tenancy
capigo help output
capigo help exit-codes
capigo help soft-delete
capigo help versioning
```

The installed help wins for CLI spelling and behavior because it ships with that exact build.
If help and this skill disagree, follow help and report the mismatch for the skill maintainer.

## 3. Plan, execute, and verify

Before a call:

1. Translate the request into one mechanical Capigo intent.
2. Discover the group and command from help.
3. Read the leaf command's help before constructing flags or JSON.
4. Resolve the tenant. If a write's tenant is ambiguous, ask before writing. Do not add a
   redundant confirmation when the user already authorized the exact resource, change, and
   tenant. Task creation still follows the separate assignee-choice gate below.
5. Use the conventions in [`references/cli-conventions.md`](./references/cli-conventions.md).

After a call:

1. Parse the single JSON document on stdout.
2. Check for an `error` key before trusting `data`; partial results can contain both.
3. On writes, verify `meta.tenant` and the returned object rather than assuming the intended
   tenant or mutation was used.
4. Report the result without exposing credentials, raw auth headers, or unrelated tenant data.

## 4. Handle missing commands and failures

If an expected command is absent, re-check the root and relevant group help. A name you guessed
is not evidence that the capability is unavailable. The operation may live under a neighboring
group, or the installed CLI may be older than the current API.

If a command fails:

1. Read the stdout `error` object, especially `message`, `next`, `request_id`, and any
   `capability_note`.
2. Read the leaf command help and `capigo help exit-codes`.
3. Use `--verbose` only when the redacted HTTP trace is actually needed.
4. Retry only when the error class supports it, such as a transient network or rate-limit
   response. Do not retry validation, auth, permission, conflict, or ambiguous write outcomes
   blindly.

A failed write does not prove the API lacks the capability. Do not invent destructive recovery
steps such as recreating resources, changing identifiers, or bypassing tenant handling.

## 5. Use the Public API specification sparingly

Public specification: <https://platform.capigo.app/api/openapi>

For ordinary user operations, stop at CLI help and execute through `capigo`. Open the spec only
when at least one of these is true:

- the user explicitly asks about the Public API contract or is building an integration;
- you are developing or reviewing this SDK;
- live help confirms a CLI gap and you need to distinguish "not exposed by this CLI" from
  "not present in the Public API";
- a version mismatch or inaccurate help/spec claim must be diagnosed.

When inside the SDK repository, prefer its versioned `api/openapi.json` for the release being
worked on; use the public URL to check the currently published contract. The spec defines API
paths and schemas, not the installed CLI's flag spelling. Finding an endpoint in the spec does
not authorize bypassing the CLI for a routine tenant operation. Report the gap or recommend an
upgrade. Call raw HTTP only when the user's task is explicitly to build or test a Public API
integration and that work is separately authorized.

## 6. Load special-case guidance only when needed

Most operations need no bundled domain guide: leaf help is sufficient.

For **creating a product**, read
[`references/product-creation.md`](./references/product-creation.md) after reading
`capigo products create --help`. It explains the two materially different request shapes:
simple products and products with options/variants.

For **creating a task or subtask**, read
[`references/task-creation.md`](./references/task-creation.md) after reading
the relevant leaf help. It defines the required pause when the assignee is omitted or unclear,
the safe way to resolve the user when they choose self-assignment, and how to place a task on
a board — resolving the board's lists, defaulting sensibly when the user names no list, and
reporting the placement so the user can ask for a move.

Do not load these references for listing, searching, or updating unrelated resources.

## Scope boundary

This skill covers Capigo CLI operation and Public API discovery. It does not define an
organization's product-code, barcode, naming, approval, or data-governance policies. Apply those
from the user's own policy or a separate organization-specific skill.
