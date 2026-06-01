# Capigo CLI SDK

[![CI](https://github.com/vtech-com/capigo-api-sdk/actions/workflows/ci.yml/badge.svg)](https://github.com/vtech-com/capigo-api-sdk/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/vtech-com/capigo-api-sdk)](https://github.com/vtech-com/capigo-api-sdk/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/vtech-com/capigo-api-sdk)](https://goreportcard.com/report/github.com/vtech-com/capigo-api-sdk)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A CLI tool and Go SDK for the [Capigo](https://capigo.app) platform — built for third-party AI agents, automation scripts, and developers who need a scriptable interface to Capigo's Public API.

```bash
# List tasks across all your tenants
capigo tasks list

# Create a task in a specific tenant
capigo tasks create --title "Fix login bug" --tenant acme

# Pipe to jq for AI agent processing
capigo tasks list --output json | jq '.[] | select(.status=="To-Do")'
```

## Why Capigo CLI?

Capigo exposes a stable Public API, but integrating it today requires implementing your own HTTP client, handling auth, pagination, and error codes from scratch. This SDK removes that friction:

- **Zero-config for AI agents** — `shell exec("capigo tasks list")` just works
- **Standardized output** — `table | json | quiet` for human and machine consumption
- **Deterministic exit codes** — agents branch on exit code, not error message text
- **Single binary** — no Node.js, Python, or any runtime required
- **Cross-platform** — Linux, macOS, Windows on amd64 and arm64

## Installation

### Go install

```bash
go install github.com/vtech-com/capigo-api-sdk@latest
```

The binary is named `capigo`. Requires Go 1.26.3+.

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

Requires Go 1.26.3+.

### Homebrew (macOS / Linux)

```bash
brew install --cask vtech-com/tap/capigo
```

### Docker

```bash
docker run --rm -e CAPIGO_API_KEY=csk_... ghcr.io/vtech-com/capigo-api-sdk:latest tasks list
```

## Quick Start

```bash
# 1. Login with your API key (generated at platform.capigo.app)
capigo auth login --key csk_abc123...

# 2. List your tenants
capigo tenants list

# 3. List tasks scoped to a tenant
capigo tasks list --tenant acme

# 4. (Optional) Set a default tenant so you don't need --tenant every time
capigo config set-default-tenant acme

# 5. Use JSON output for AI agent or script consumption
capigo tasks list --output json | jq '.[] | select(.status=="To-Do")'
```

## Commands

```
capigo auth login        Login with a csk_ API key
capigo auth logout       Remove credentials from config
capigo auth whoami       Show current user

capigo config set <key> <value>           Set a config value
capigo config get <key>                   Get a config value
capigo config set-default-tenant <code>   Set default tenant for the active profile
capigo config unset-default-tenant        Clear the default tenant

capigo tenants list      List tenants you can access

capigo tasks list        List tasks (supports --status, --page, --limit)
capigo tasks get <id>    Get task by ID
capigo tasks create      Create a new task (--title required; --tenant required)

capigo boards list       List boards

capigo products list     List products (supports --query, --updated-since, --ids, --all)
capigo products get      (coming soon)
capigo products create   Create a product (--name required, or --from-json)
capigo products update   Update a product
capigo products variants Upsert product variants

capigo brands list           List reference brands (supports --query)
capigo categories list       List reference categories (supports --query)
capigo product-types list    List reference product types (supports --query)
capigo units list            List reference units (supports --query)
capigo variants list         List variants by barcode prefix (supports --barcode-prefix, --sort)

capigo version           Print version info
```

**Global flags** available on every command:

| Flag | Description |
|------|-------------|
| `--tenant <code>` | Scope call to a specific tenant |
| `--no-tenant` | Force global mode (override configured default) |
| `--profile <name>` | Use a specific config profile |
| `--output table\|json\|quiet` | Output format (unknown values are rejected with an error) |
| `--api-url <url>` | Override API base URL (staging / local dev) |
| `--verbose` | Print HTTP request/response details (Authorization header is redacted) |

## Products

```bash
# List products
capigo products list --tenant acme

# Search products by name, variant name, SKU, or barcode
capigo products list --tenant acme --query iphone

# Delta sync — only products updated since a previous call
capigo products list --tenant acme --updated-since 2026-01-01T00:00:00Z

# Fetch all pages into a single JSON stream
capigo products list --tenant acme --all --output json

# Create a simple product
capigo products create --tenant acme --name "Blue T-Shirt" --sku "SKU-001" --price 299000
```

| Flag | Description |
|------|-------------|
| `--query`/`-q` | Free-text search (2–500 chars): product name, variant name, SKU, barcode |
| `--updated-since` | ISO 8601 timestamp for delta sync |
| `--ids` | Comma-separated product UUIDs (max 50); mutually exclusive with `--all` |
| `--all` | Auto-paginate the full catalog |
| `--page` | Page number (default 1) |
| `--limit` | Items per page (1–100, default 20) |

## Reference data

Reference endpoints return bounded sets of lookup values (brands, categories, product types, units). Use them to resolve names to UUIDs before creating products.

```bash
# List brands (name search)
capigo brands list --tenant acme --query nike

# List categories
capigo categories list --tenant acme

# List product types
capigo product-types list --tenant acme --query shirt

# List units
capigo units list --tenant acme

# List variants by barcode prefix (top-1 highest barcode for auto-increment)
capigo variants list --tenant acme --barcode-prefix 620111 --sort -barcode --limit 1
```

| Command | Key flags | Description |
|---------|-----------|-------------|
| `brands list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `categories list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `product-types list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `units list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `variants list` | `--barcode-prefix`, `--sort` | `--sort` accepts `barcode` or `-barcode` only |

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

### Products

```bash
# List products
capigo products list --tenant acme

# Search products by name, variant name, SKU, or barcode
capigo products list --tenant acme --query iphone

# Delta sync — only products updated since a previous call
capigo products list --tenant acme --updated-since 2026-01-01T00:00:00Z

# Fetch all pages into a single JSON stream
capigo products list --tenant acme --all --output json

# Create a simple product
capigo products create --tenant acme --name "Blue T-Shirt" --sku "SKU-001" --price 299000
```

| Flag | Description |
|------|-------------|
| `--query`/`-q` | Free-text search (2–500 chars): product name, variant name, SKU, barcode |
| `--updated-since` | ISO 8601 timestamp for delta sync |
| `--ids` | Comma-separated product UUIDs (max 50); mutually exclusive with `--all` |
| `--all` | Auto-paginate the full catalog |
| `--page` | Page number (default 1) |
| `--limit` | Items per page (1–100, default 20) |

## Reference data

Reference endpoints return bounded sets of lookup values (brands, categories, product types, units). Use them to resolve names to UUIDs before creating products.

```bash
# List brands (name search)
capigo brands list --tenant acme --query nike

# List categories
capigo categories list --tenant acme

# List product types
capigo product-types list --tenant acme --query shirt

# List units
capigo units list --tenant acme

# List variants by barcode prefix (top-1 highest barcode for auto-increment)
capigo variants list --tenant acme --barcode-prefix 620111 --sort -barcode --limit 1
```

| Command | Key flags | Description |
|---------|-----------|-------------|
| `brands list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `categories list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `product-types list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `units list` | `--query`/`-q` | Name-contains search (case-insensitive, max 200 chars) |
| `variants list` | `--barcode-prefix`, `--sort` | `--sort` accepts `barcode` or `-barcode` only |

## Configuration precedence

```
CLI flag (--api-url, --tenant, …)
  > Environment variable (CAPIGO_API_KEY, CAPIGO_API_URL, CAPIGO_TENANT, CAPIGO_PROFILE)
    > Config file (~/.capigo/config.json)
```

AI agents can inject credentials via environment variables without writing a config file:

```bash
CAPIGO_API_KEY=csk_... CAPIGO_TENANT=acme capigo tasks list
```

To point at a different environment (staging, local dev) without touching the config file:

```bash
CAPIGO_API_URL=http://localhost:3999 CAPIGO_API_KEY=csk_dev_... capigo tasks list
```

## Output Modes

```bash
# Default: human-readable table
capigo tasks list --tenant acme
# ┌──────────────┬───────────────────┬────────┬──────────┐
# │ ID           │ Title             │ Status │ Assignee │
# ├──────────────┼───────────────────┼────────┼──────────┤
# │ task_abc123  │ Fix login bug     │ To-Do  │ Alice    │
# └──────────────┴───────────────────┴────────┴──────────┘

# Global mode (--no-tenant) adds a Tenant column automatically
capigo tasks list --no-tenant

# JSON for AI agents and scripts
capigo tasks list --output json

# Quiet mode — prints the resource ID only (useful for shell piping)
capigo tasks create --title "New task" --tenant acme --output quiet
# task_def456
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
capigo --output json tasks list --tenant acme | jq '.[] | select(.status=="To-Do")'
```

### LangChain / Python

```python
import subprocess, json

result = subprocess.run(
    ["capigo", "tasks", "list", "--tenant", "acme", "--output", "json"],
    capture_output=True, text=True
)
if result.returncode == 0:
    tasks = json.loads(result.stdout)
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
capigo tasks list --tenant acme --output json
```

### Environment variable injection (CI / secrets manager)

```bash
CAPIGO_API_KEY=${{ secrets.CAPIGO_KEY }} \
CAPIGO_API_URL=${{ vars.CAPIGO_API_URL }} \
CAPIGO_TENANT=acme \
capigo tasks list --output json
```

## API Reference

OpenAPI spec: [`api/openapi.json`](./api/openapi.json)

Source: `https://platform.capigo.app/api/openapi`. Run `make update-spec` to sync.

View it interactively:

- Paste into [editor.swagger.io](https://editor.swagger.io)
- Or use the VS Code **Swagger Viewer** extension

## Compatibility Matrix

| CLI version | Capigo API | Status | Support until |
|-------------|------------|--------|---------------|
| v1.x | /api/v1 | ✅ Supported | TBD |
| v0.x | /api/v1 | ⚠️ Beta, no SLA | v1.0 release |

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

To report a security vulnerability, email **security@capigo.app**. Do **not** open a public GitHub issue. See [SECURITY.md](SECURITY.md) for the full policy.

## License

[MIT](LICENSE) — Copyright © 2026 Capigo / VTech
