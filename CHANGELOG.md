# Changelog

All notable changes to Capigo CLI SDK are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

---

## [0.1.0] — 2026-05-23

### Added

- `auth login` — save API key to `~/.capigo/config.json`
- `auth logout` — remove credentials from config
- `auth whoami` — display current authenticated user
- `config set` / `config get` — manage config values
- `config set-default-tenant` / `config unset-default-tenant`
- `tenants list` — list tenants accessible by the current key
- `tasks list` — list tasks with optional `--tenant` / `--no-tenant` scope
- `tasks get <id>` — fetch a single task by ID
- `tasks create` — create a task (tenant required)
- `boards list` — list mission boards
- Global flags: `--tenant`, `--no-tenant`, `--profile`, `--output`, `--api-url`
- Output modes: `table`, `json`, `quiet`
- Standardized exit codes (0–7) for AI agent integration
- Automatic `X-Tenant-Code` header injection based on resolution precedence
- Config file created with `chmod 600` to protect API key at rest
- `CAPIGO_API_KEY`, `CAPIGO_TENANT`, `CAPIGO_PROFILE`, `CAPIGO_API_URL` environment variable support
- Single binary distribution via GoReleaser for Linux, macOS, Windows (amd64 + arm64)
- GitHub Actions CI (lint + test on 3 OSes)
- Docker image published to `ghcr.io/vtech-com/capigo-api-sdk`

---

[Unreleased]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vtech-com/capigo-api-sdk/releases/tag/v0.1.0
