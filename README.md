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
- **Standardized output** — `table | json | yaml | quiet` for human and machine consumption
- **Deterministic exit codes** — agents branch on exit code, not error message text
- **Single binary** — no Node.js, Python, or any runtime required
- **Cross-platform** — Linux, macOS, Windows on amd64 and arm64

## Installation

### Homebrew (macOS / Linux)

```bash
brew install vtech-com/tap/capigo
```

### Install script

```bash
curl -sSL https://capigo.app/install.sh | sh
```

To install a specific version:

```bash
curl -sSL https://capigo.app/install.sh | VERSION=v1.2.0 sh
```

### Docker

```bash
docker run --rm -e CAPIGO_API_KEY=csk_... ghcr.io/vtech-com/capigo-api-sdk:latest tasks list
```

### Manual (GitHub Releases)

Download the binary for your platform from [Releases](https://github.com/vtech-com/capigo-api-sdk/releases), extract, and place it in your `PATH`.

### Build from source

```bash
git clone https://github.com/vtech-com/capigo-api-sdk.git
cd capigo-api-sdk
go build -o capigo .
```

Requires Go 1.22+.

## Quick Start

```bash
# 1. Login with your API key (generated at platform.capigo.app)
capigo auth login --key csk_abc123...

# 2. Verify
capigo auth whoami
# ✅ Logged in as: alice@example.com

# 3. List your tenants
capigo tenants list

# 4. (Optional) Set a default tenant
capigo config set-default-tenant acme

# 5. Start using
capigo tasks list
capigo tasks list --tenant globex   # override for one call
capigo tasks list --no-tenant       # global view across all tenants
```

## Commands

```
capigo auth login        Login with a csk_ API key
capigo auth logout       Remove credentials from config
capigo auth whoami       Show current user

capigo config set        Set a config value
capigo config get        Get a config value
capigo config set-default-tenant <code>   Set default tenant
capigo config unset-default-tenant        Clear default tenant
capigo config list-profiles               List config profiles

capigo tenants list      List tenants you can access

capigo tasks list        List tasks
capigo tasks get <id>    Get task by ID
capigo tasks create      Create a new task

capigo boards list       List boards
capigo boards get <id>   Get board with its lists

capigo members list      List tenant members

capigo products list     List products (supports delta sync)

capigo version           Print version info
capigo completion        Generate shell completions (bash/zsh/fish)
```

**Global flags** available on every command:

| Flag | Description |
|------|-------------|
| `--tenant <code>` | Scope call to a specific tenant |
| `--no-tenant` | Force global mode (override configured default) |
| `--profile <name>` | Use a specific config profile |
| `--output table\|json\|yaml\|quiet` | Output format |
| `--api-url <url>` | Override API base URL (staging / local dev) |

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

### Configuration precedence

```
CLI flag (--api-key, --tenant)
  > Environment variable (CAPIGO_API_KEY, CAPIGO_TENANT, CAPIGO_PROFILE)
    > Config file (~/.capigo/config.json)
```

AI agents can inject credentials via environment variables without writing a config file:

```bash
CAPIGO_API_KEY=csk_... CAPIGO_TENANT=acme capigo tasks list
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

# Global mode adds a Tenant column automatically
capigo tasks list --no-tenant

# JSON for AI agents
capigo tasks list --output json

# Quiet mode — returns ID only (for piping)
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

## AI Agent Integration

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
```

### n8n

Use the **Execute Command** node:

```
capigo tasks list --output json
```

### Environment variable injection (CI / secrets manager)

```bash
CAPIGO_API_KEY=${{ secrets.CAPIGO_KEY }} \
CAPIGO_TENANT=acme \
capigo tasks list
```

## API Reference

OpenAPI spec: [`api/openapi.json`](./api/openapi.json)

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
