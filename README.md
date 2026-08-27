# Capigo CLI SDK

[![CI](https://github.com/vtech-com/capigo-api-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/vtech-com/capigo-api-sdk/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/vtech-com/capigo-api-sdk)](https://github.com/vtech-com/capigo-api-sdk/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/vtech-com/capigo-api-sdk)](https://goreportcard.com/report/github.com/vtech-com/capigo-api-sdk)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A CLI tool and Go SDK for the [Capigo](https://capigo.app) platform — built for third-party AI agents, automation scripts, and developers who need a scriptable interface to Capigo's Public API.

```bash
# List tasks (omit --tenant to see tasks across all your tenants)
capigo tasks list

# Create a task in a specific tenant
capigo tasks create --title "Fix login bug" --tenant acme

# Pipe to jq for AI agent processing
capigo tasks list | jq '.data[] | select(.status=="To-Do")'
```

## Why Capigo CLI?

Capigo exposes a stable Public API, but integrating it today requires implementing your own HTTP client, handling auth, pagination, and error codes from scratch. This SDK removes that friction:

- **Zero-config for AI agents** — `shell exec("capigo tasks list")` just works
- **One output shape** — every command prints `{"data": …, "meta": …}` on stdout; nothing else, ever
- **Deterministic exit codes** — agents branch on exit code, not error message text
- **Single binary** — no Node.js, Python, or any runtime required
- **Cross-platform** — Linux, macOS, Windows on amd64 and arm64

## Installation

### Linux (install script) — recommended for servers and VPS hosts

```bash
curl -fsSL https://raw.githubusercontent.com/vtech-com/capigo-api-sdk/main/scripts/install.sh | sh
```

The script detects your OS and architecture, downloads the matching release archive, **verifies it
against the release's `checksums.txt`**, and installs the binary to `/usr/local/bin/capigo`. It
refuses to install anything that fails verification.

To upgrade, run the same command again — the binary is replaced in place.

If piping a script into a shell is against your policy, download it, read it, then run it:

```bash
curl -fsSL -O https://raw.githubusercontent.com/vtech-com/capigo-api-sdk/main/scripts/install.sh
less install.sh
sh install.sh
```

Two environment variables control it:

| Variable | Purpose |
|----------|---------|
| `CAPIGO_VERSION` | Install a specific version (e.g. `v0.20.3`) instead of the latest release |
| `CAPIGO_BIN_DIR` | Install somewhere other than `/usr/local/bin` (useful to avoid `sudo`) |

```bash
# Pin a version, install without sudo
CAPIGO_VERSION=v0.20.3 CAPIGO_BIN_DIR="$HOME/.local/bin" sh install.sh
```

Works on Linux and macOS, amd64 and arm64.

### Debian / Ubuntu (`.deb`)

Every release ships a `.deb` alongside the tarballs, so the system package manager tracks the
binary (clean uninstall, `dpkg -l capigo`):

```bash
VERSION=0.20.3   # without the leading "v"
curl -fsSLO "https://github.com/vtech-com/capigo-api-sdk/releases/download/v${VERSION}/capigo_${VERSION}_linux_amd64.deb"
sudo apt install "./capigo_${VERSION}_linux_amd64.deb"
```

Upgrading means installing the newer `.deb` the same way. There is no APT repository, so
`apt upgrade` will not pull new versions on its own.

To remove it: `sudo apt remove capigo`.

### Go install

```bash
go install github.com/vtech-com/capigo-api-sdk@latest
```

Go names the installed binary after the module path, so it lands as `capigo-api-sdk`, not
`capigo`. Rename it:

```bash
mv "$(go env GOPATH)/bin/capigo-api-sdk" "$(go env GOPATH)/bin/capigo"
```

Requires Go 1.26.5+.

### GitHub Releases (recommended for most users)

Download the pre-built binary for your platform from [Releases](https://github.com/vtech-com/capigo-api-sdk/releases), extract, and place it in your `PATH`.

Binaries are provided for:
- Linux: `amd64`, `arm64`
- macOS (darwin): `amd64`, `arm64`
- Windows: `amd64`, `arm64`

### Build from source

```bash
git clone https://github.com/vtech-com/capigo-api-sdk.git
cd capigo-api-sdk
make build
# binary at ./dist/capigo
```

Requires Go 1.26.5+.

### Homebrew (macOS only)

The tap ships a **cask**, and casks are macOS-only — `brew install --cask` does not work on
Linuxbrew. On Linux, use the install script or the `.deb` above.

```bash
brew install --cask vtech-com/tap/capigo
```

To update an existing installation:

```bash
brew update
brew upgrade --cask capigo
capigo version
```

If you previously installed `capigo` as a formula (pre-cask migration), remove it first or the
cask install will conflict:

```bash
brew uninstall --formula capigo
brew install --cask vtech-com/tap/capigo
```

## Quick Start

```bash
# 1. Login with your API key (generated at platform.capigo.app)
capigo auth login --key csk_abc123...

# 2. Preflight: confirm the API is reachable and the key is accepted
capigo health

# 3. List your tenants
capigo tenants list

# 4. List tasks scoped to a tenant
capigo tasks list --tenant acme

# 5. (Optional) Set a default tenant so you don't need --tenant every time
capigo config set-default-tenant acme

# 6. Use JSON output for AI agent or script consumption
capigo tasks list | jq '.data[] | select(.status=="To-Do")'
```

> **Preflight tip:** `capigo auth whoami` calls `GET /me`, which the API does not implement — no
> such route exists, so it 404s (exit `4`) with any key. That is not a rejected key; a rejected key
> is exit `2`. `capigo health` is the preflight that works: exit `0` means the API is reachable
> *and* the key is accepted.

## Commands

```
capigo auth login        Login with a csk_ API key
capigo auth logout       Remove credentials from config
capigo auth whoami       Show current user (GET /me — not implemented; always 404s. Use `health`)
capigo health            Preflight: check API connectivity + key (exit 2 if auth fails)

capigo config set <key> <value>           Set a config value
capigo config get <key>                   Get a config value
capigo config set-default-tenant <code>   Set default tenant for the active profile
capigo config unset-default-tenant        Clear the default tenant

capigo tenants list      List tenants you can access

capigo tasks list                    List tasks (--query/-q, --status, --priority, --assignee-id,
                                      --owner-id, --board-id, --board-list-id, --due-after/--due-before,
                                      --created-after/--created-before, --parent-task-id, --page, --limit)
capigo tasks get <id|--code>           Get task by ID or code (--code requires --tenant)
capigo tasks comments <id|--code>      List a task's comment + activity timeline (--type comment|activity,
                                       --sort asc|desc, --page, --limit; --code requires --tenant)
capigo tasks attachments download <task-id|--code> <attachment-id>          Download a task-level attachment
capigo tasks comments attachments download <task-id|--code> <attachment-id> Download a comment/activity attachment
capigo tasks update <id>              Partial update a task (PATCH; --tenant optional; at least one field required)
capigo tasks create                   Create a new task (--title + --tenant required; --follower-id repeatable;
                                       --subtasks-json to create subtasks atomically)
capigo tasks subtasks list <id|--code>     List a task's subtasks (--code requires --tenant)
capigo tasks subtasks create <id|--code>   Add subtask(s) to an existing task (--title, or --from-json for a batch;
                                       --code requires --tenant)

capigo boards list       List boards (supports --query/-q, --page, --limit)
capigo boards get <id>   Get board by ID (includes its `lists` array)
capigo boards create     Create a board (--name + --tenant required, or --from-json)
capigo boards update <id>        Partial update a board (PATCH; --tenant required; at least one field required)
capigo boards lists create <board-id>        Create a list in a board (--name + --tenant required)
capigo boards lists update <board-id> <list-id>  Update a list in a board (PATCH; --tenant required)

capigo members list      List workspace members (supports --query/-q, --page, --limit)
capigo members get <id>  Get a member by ID

capigo products list     List products (supports --query, --updated-since, --ids, --all)
capigo products get <id> Get a product by ID
capigo products create   Create a product (--name required, or --from-json; --aliases/--tags repeatable)
capigo products update   Update a product (partial; --aliases/--tags repeatable, or --from-json)
capigo products variants Upsert product variants

capigo brands list           List reference brands (supports --query)
capigo brands get <id>       Get a brand by ID
capigo brands create         Create a brand (--name required)
capigo brands update <id>    Partial update a brand (PATCH)
capigo brands replace <id>   Full replace a brand (PUT, all fields required)

capigo categories list           List reference categories (supports --query)
capigo categories get <id>       Get a category by ID
capigo categories create         Create a category (--name required)
capigo categories update <id>    Partial update a category (PATCH)
capigo categories replace <id>   Full replace a category (PUT, all fields required)

capigo product-types list           List reference product types (supports --query)
capigo product-types get <id>       Get a product type by ID
capigo product-types create         Create a product type (--name required)
capigo product-types update <id>    Partial update a product type (PATCH)
capigo product-types replace <id>   Full replace a product type (PUT, all fields required)

capigo units list           List reference units (supports --query)
capigo units get <id>       Get a unit by ID
capigo units create         Create a unit (--name and --abbreviation required)
capigo units update <id>    Partial update a unit (PATCH)
capigo units replace <id>   Full replace a unit (PUT, all fields required)

capigo variants list            List variants by barcode prefix (supports --barcode-prefix, --sort)
capigo variants get <id|--sku>  Get a variant by ID or SKU (--sku requires --tenant)

capigo version           Print version info
```

Run `capigo <group> <command> --help` for the complete, authoritative flag list from your binary.

**Global flags** available on every command:

| Flag | Description |
|------|-------------|
| `--api-url <url>` | Override API base URL (staging / local dev) |
| `-v, --verbose` | Print HTTP request/response details (Authorization header is redacted) |

`--tenant <code>` appears as a local flag on commands that require or accept a tenant scope (e.g. `capigo products list --tenant acme`). It is not a global flag. The active config profile is always read from `~/.capigo/config.json` and cannot be overridden at runtime.

Every PCMS command (`products`, `variants`, `brands`, `categories`, `product-types`, `units`) **requires** a tenant on every verb. `tasks list`/`get`, `boards list`/`get`, and `members list`/`get` accept an *optional* `--tenant` — omit it to read across every tenant you can access (`meta.tenant` is then absent — there is no single tenant to name). `tasks create` and `tasks subtasks create` always require a tenant; `tasks subtasks list` requires a tenant only when addressed by `--code`. Board writes — `boards create`, `boards update`, `boards lists create`, `boards lists update` — always require a tenant.

## Products

```bash
# List products
capigo products list --tenant acme

# Search products by name, alias, tag, variant name, SKU, or barcode
capigo products list --tenant acme --query iphone

# Delta sync — only products updated since a previous call
capigo products list --tenant acme --updated-since 2026-01-01T00:00:00Z

# Fetch all pages into a single JSON stream
capigo products list --tenant acme --all

# Create a simple product with aliases and tags
capigo products create --tenant acme --name "Blue T-Shirt" --sku "SKU-001" --price 299000 \
  --aliases "BT-001" --tags "summer" --tags "sale"

# Partial update — this is products' one write verb for updates (there is no `products replace`)
capigo products update <id> --tenant acme --tags "clearance"
```

| Flag | Description |
|------|-------------|
| `--query`/`-q` | Free-text search (2–500 chars): product name, aliases, tags, variant name, SKU, barcode |
| `--updated-since` | ISO 8601 timestamp for delta sync |
| `--ids` | Comma-separated product UUIDs (max 50); mutually exclusive with `--all` |
| `--all` | Auto-paginate the full catalog |
| `--aliases` | Alternative names / product codes (repeatable: `--aliases foo --aliases bar`) |
| `--tags` | Free-form labels (repeatable: `--tags foo --tags bar`) |
| `--page` | Page number (default 1) |
| `--limit` | Items per page (1–100, default 20) |

Notes:

- `products create`/`update` also accept `--from-json -` for options + variants in one call (mutually exclusive with individual field flags).
- `products update` is the **only** write verb for updates — it sends PUT with partial semantics (only the fields you provide are changed; omitted fields are left as-is). Unlike reference data, products has no separate `replace` command.
- Soft-deleted products still appear in list results. Check `is_deleted` on the product object — the plain `status` field does not reveal deletion on its own.
- `--all` streams every row it fetches even if it fails mid-pagination; the rows appear beneath an `error` key and the command exits non-zero.
- `--ids` exits 4 when a requested id does not come back; the rows that did are still printed, beneath an `error` key naming the ones that did not.

## Reference data

Reference endpoints manage lookup values (brands, categories, product types, units) used to resolve names to UUIDs when creating products. Tenant is **required** on every verb.

```bash
# List brands with optional name search
capigo brands list --tenant acme --query nike

# Get a specific brand by ID
capigo brands get <id> --tenant acme

# Create a brand
capigo brands create --tenant acme --name "Nike" --logo-url "https://example.com/logo.png"

# Partial update (PATCH) — only provided fields change
capigo brands update <id> --tenant acme --name "Nike Inc"

# Full replace (PUT) — all fields required
capigo brands replace <id> --tenant acme --name "Nike" --no-logo

# Categories: --clear-parent promotes to root; replace uses --root or --parent-id
capigo categories update <id> --tenant acme --clear-parent
capigo categories replace <id> --tenant acme --name "Electronics" --root

# Product types: --clear-description removes description; replace uses --no-description
capigo product-types replace <id> --tenant acme --name "Phone" --no-description

# List variants by barcode prefix (top-1 highest barcode for auto-increment)
capigo variants list --tenant acme --barcode-prefix 620111 --sort -barcode --limit 1

# Get a variant by SKU instead of UUID
capigo variants get --sku "SKU-001" --tenant acme
```

| Command | Key flags | Description |
|---------|-----------|-------------|
| `brands list` | `--query`/`-q` | Name-contains search (max 200 chars) |
| `brands get <id>` | | Fetch single brand by UUID |
| `brands create` | `--name` (required), `--logo-url` | Create brand |
| `brands update <id>` | `--name`, `--logo-url`, `--clear-logo` | Partial update (PATCH) |
| `brands replace <id>` | `--name` (required), `--logo-url`/`--no-logo` (one required) | Full replace (PUT) |
| `categories list` | `--query`/`-q` | Name-contains search (max 200 chars) |
| `categories get <id>` | | Fetch single category by UUID |
| `categories create` | `--name` (required), `--parent-id` | Create category |
| `categories update <id>` | `--name`, `--parent-id`, `--clear-parent` | Partial update (PATCH) |
| `categories replace <id>` | `--name` (required), `--parent-id`/`--root` (one required) | Full replace (PUT) |
| `product-types list` | `--query`/`-q` | Name-contains search (max 200 chars) |
| `product-types get <id>` | | Fetch single product type by UUID |
| `product-types create` | `--name` (required), `--description` | Create product type |
| `product-types update <id>` | `--name`, `--description`, `--clear-description` | Partial update (PATCH) |
| `product-types replace <id>` | `--name` (required), `--description`/`--no-description` (one required) | Full replace (PUT) |
| `units list` | `--query`/`-q` | Name-contains search (max 200 chars) |
| `units get <id>` | | Fetch single unit by UUID |
| `units create` | `--name`, `--abbreviation` (both required) | Create unit |
| `units update <id>` | `--name`, `--abbreviation` | Partial update (PATCH) |
| `units replace <id>` | `--name`, `--abbreviation` (both required) | Full replace (PUT) |
| `variants list` | `--barcode-prefix`, `--sort` | Read-only; `--sort` accepts `barcode` or `-barcode` only. All variant writes go through `products variants` — there is no `variants create`/`update`/`replace`. |

## Tasks

```bash
# Filter tasks
capigo tasks list --status To-Do --priority high

# Get a task by its human-readable code instead of UUID
capigo tasks get --code ACMEC-68 --tenant acme

# Read a task's comment + activity timeline
capigo tasks comments <task-uuid> --type comment

# List subtasks
capigo tasks subtasks list <task-uuid> --tenant acme

# Create a task
capigo tasks create --tenant acme --title "Fix login bug" --priority high

# Create a task with subtasks in one atomic call
echo '[{"title":"Subtask A"},{"title":"Subtask B"}]' \
  | capigo tasks create --tenant acme --title "Epic X" --subtasks-json -

# Add subtasks to an existing task
echo '[{"title":"Design"},{"title":"Build","priority":"High"}]' \
  | capigo tasks subtasks create <parent-uuid> --tenant acme --from-json -

# Download an attachment (task-level or from a comment/activity entry)
capigo tasks attachments download <task-uuid> <attachment-uuid> --dest ./downloads/
capigo tasks comments attachments download <task-uuid> <attachment-uuid>
```

Attachment downloads fetch a signed, short-lived URL (5-minute lifetime) and write the bytes to
disk in the same call — there's no separate "get the URL" step, and the CLI never prints the
raw URL. Run the download command right before you need the file; without `--dest`, it lands in
the current directory under its original name.

## Configuration

Credentials and settings are stored in `~/.capigo/config.json` (permissions `600`).

```json
{
  "version": 1,
  "profiles": {
    "default": {
      "api_key": "csk_abc123...",
      "api_url": "https://platform.capigo.app",
      "default_tenant": "acme",
      "known_tenants": ["acme", "globex", "initech"]
    }
  },
  "active_profile": "default"
}
```

There is exactly one active profile (`active_profile`); the CLI does not take a `--profile` flag.

### Configuration precedence

```
CLI flag (--api-url, --tenant, …)
  > Environment variable (CAPIGO_API_KEY, CAPIGO_API_URL, CAPIGO_TENANT)
    > Config file (~/.capigo/config.json)
```

AI agents can inject credentials via environment variables without writing a config file:

```bash
CAPIGO_API_KEY=csk_... CAPIGO_TENANT=acme capigo tasks list
```

### Headless hosts (VPS, CI, agent runners)

On a machine with no human at the keyboard, **do not use `capigo auth login`** — it exists to write
`~/.capigo/config.json` interactively. Provision the environment instead; the env vars above take
precedence over the config file, so no login step is needed at all:

```bash
export CAPIGO_API_KEY=csk_...
export CAPIGO_TENANT=acme
capigo health   # exit 0 = API reachable and key accepted
```

Source the key from whatever your host already uses for secrets (a systemd unit's
`EnvironmentFile=`, the CI secret store, your process manager's env block) rather than committing it
to a shell profile. `capigo health` is the preflight that tells you the provisioning worked.

To point at a different environment (staging, local dev) without touching the config file:

```bash
CAPIGO_API_URL=http://localhost:3999 CAPIGO_API_KEY=csk_dev_... capigo tasks list
```

## Output

There is no output flag and no output modes. Every command that succeeds prints exactly one
shape to stdout:

```json
{ "data": …, "meta": { … } }
```

```bash
capigo tasks list --tenant acme | jq '.data[] | {id, title, status}'

# Omit --tenant on a spanning read to see tasks across every tenant you can access
capigo tasks list

capigo tasks create --title "New task" --tenant acme | jq -r '.data.id'
```

`data` is an array for a `list` command and an object for a single item (`get`, `create`,
`update`, `replace`). The CLI does not unwrap the API's own `{"data": …}` envelope, so
`.data.id` is correct where `.id` used to be. Redirecting (`>`) or piping (`|`) is always
safe — stdout is JSON and nothing else is ever written to it.

### JSON contract

| Command type | Shape | Notes |
|---|---|---|
| `list` commands | `{"data": [...], "meta": {"page", "limit", "total", "has_more", "tenant", "tenant_source", …}}` | Full API objects, never stripped display models. `data` is always a JSON array (never `null`). |
| Single-item commands (`get`, `create`, `update`, `replace`, `variants`) | `{"data": {...}, "meta": {"tenant", "tenant_source", …}}` | Full API object under `data`. |

`meta.tenant` and `meta.tenant_source` (`flag`/`env`/`config`) name the tenant a command
resolved to and where that came from; both are absent on a command that takes no tenant, and
on a cross-tenant read that resolved none. A list also carries `page`/`limit`/`total`/
`has_more`. `total` is the count across every page — count `meta.total`, never `data[]`. The
CLI additionally adds `server_time` (the delta-sync cursor, taken from a response header the
caller never sees).

`meta` carries only what a caller cannot work out for itself. Anything derivable from what you
sent and what you got back — which of your `--ids` came home, whether an `--all` sweep finished
— is left to you and to the exit code.

A failing command prints `{"error": {...}}` — still JSON, still on stdout — plus a one-line
summary on stderr. Parse stdout unconditionally; see [Exit Codes](#exit-codes).

Skill authors: use `.data[]` to iterate list results, e.g.:

```bash
capigo tasks list | jq '.data[] | select(.status=="To-Do")'
capigo brands list --tenant acme | jq '.data[].id'
capigo brands create --tenant acme --name Nike | jq '.data.id'
```

## Exit Codes

AI agents should branch on exit code, **not** on error message text.

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General / unexpected error |
| `2` | Auth error (invalid / expired key) |
| `3` | Permission error (403 — tenant mismatch, no role) |
| `4` | Not found (404) |
| `5` | Validation error (400) |
| `6` | Network error — agent should retry |
| `7` | Rate limit (429) — agent should back off |
| `8` | Conflict (409) — resource already exists or state conflict |

## AI Agent Integration

### Shell / jq

```bash
# List open tasks and filter with jq
capigo tasks list --tenant acme | jq '.data[] | select(.status=="To-Do")'
```

### LangChain / Python

```python
import subprocess, json

result = subprocess.run(
    ["capigo", "tasks", "list", "--tenant", "acme"],
    capture_output=True, text=True
)
if result.returncode == 0:
    tasks = json.loads(result.stdout)["data"]
elif result.returncode == 6:
    # Network error — retry
    ...
elif result.returncode == 7:
    # Rate limit — back off before retrying
    ...
```

### n8n

Use the **Execute Command** node:

```
capigo tasks list --tenant acme
```

### Environment variable injection (CI / secrets manager)

```bash
CAPIGO_API_KEY=${{ secrets.CAPIGO_KEY }} \
CAPIGO_API_URL=${{ vars.CAPIGO_API_URL }} \
CAPIGO_TENANT=acme \
capigo tasks list
```

### Bundled agent skill

This repo ships an agent **skill** under [`skills/capigo-api/`](./skills/capigo-api/) — a
self-contained, CLI-first guide (`SKILL.md` + `references/`) for operating Capigo safely.
The skill first checks whether `capigo` is installed and records the active binary/version. If
the binary is missing, it recommends an installation path appropriate to the environment rather
than assuming the command exists or installing software without approval.

When the binary is available, the skill treats `capigo --help`, group help, leaf-command help,
and the bundled help topics as the authoritative command reference. It deliberately does not
copy the full command tree or every flag into the skill: the Public API and CLI can expand
without making a static command catalogue stale. Bundled references cover only stable shared
contracts (tenant resolution, filters, pagination, JSON output, errors) and genuinely special
workflows such as simple-versus-variants product creation.

For normal tenant operations, the agent stays on the CLI. It consults the public OpenAPI spec
only when the user asks about the API contract, when developing the SDK, when diagnosing a
version mismatch, or after live help confirms a CLI capability gap. How your organisation
assigns product codes, allocates barcodes, or governs reference data remains policy for your own
catalogue skill layered on top of this one.

**Install it** with the [`skills`](https://github.com/vercel-labs/skills) CLI, which pulls the
skill straight from this repo and drops it into your agent's skills directory (Claude Code,
Cursor, Codex, OpenCode, and many more):

```bash
npx skills add vtech-com/capigo-api-sdk --skill capigo-api
```

Add `-g` to install at user level (e.g. `~/.claude/skills/`) instead of per-project, `-a <agent>`
to target a specific agent, or `--list` to see the available skills without installing. `--skill`
matches the `name` in the skill's frontmatter; the tool discovers it under `skills/capigo-api/`
automatically.

The skill content is location-independent (all internal links are relative), so it works the
same whether installed into an agent runtime, read in-repo, or browsed on GitHub. (Contributors:
see [CONTRIBUTING.md](CONTRIBUTING.md) for the `make skill-package` development target.)

## API Reference

OpenAPI spec: [`api/openapi.json`](./api/openapi.json)

Source: `https://platform.capigo.app/api/openapi`. Run `make update-spec` to sync.

View it interactively:

- Paste into [editor.swagger.io](https://editor.swagger.io)
- Or use the VS Code **Swagger Viewer** extension

## Compatibility

The CLI targets Capigo's `/api/v1` Public API. Don't rely on a hardcoded version number in this
file to know what's current — check the [Release badge](#capigo-cli-sdk) at the top of this
README or the [Releases page](https://github.com/vtech-com/capigo-api-sdk/releases) for the
latest tag, and `CHANGELOG.md` for what shipped in it (including any pre-staged commands ahead
of a given tenant's backend deployment).

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

To report a security vulnerability, email **security@capigo.app**. Do **not** open a public GitHub issue. See [SECURITY.md](SECURITY.md) for the full policy.

## License

[MIT](LICENSE) — Copyright © 2026 Capigo / VTech
