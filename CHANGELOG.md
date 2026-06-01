# Changelog

All notable changes to Capigo CLI SDK are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

---

## [0.3.0] — 2026-06-01

### Added

- `brands list` — list reference brands for a tenant; optional `--query`/`-q` name-contains search (case-insensitive, max 200 chars), `--page`, `--limit`
- `categories list` — list reference categories for a tenant; optional `--query`/`-q`, `--page`, `--limit`
- `product-types list` — list reference product types for a tenant; optional `--query`/`-q`, `--page`, `--limit`
- `units list` — list reference units for a tenant; optional `--query`/`-q`, `--page`, `--limit`
- `variants list` — query variants by `--barcode-prefix`; `--sort barcode|-barcode` (validated, default `-barcode`); default `--limit 1` for top-barcode-in-namespace lookup; cross-tenant without `--tenant`
- `products list --query`/`-q` — free-text search (min 2 chars, max 500) across product name, variant name, SKU, and barcode; composes with `--updated-since` and `--ids`
- `api/openapi.json`: added `/pcms/brands`, `/pcms/categories`, `/pcms/product-types`, `/pcms/units`, `/pcms/variants` GET paths and component schemas; `?q` parameter on `/pcms/products`

### Fixed

- `Brand.LogoURL`, `VariantRecord.Barcode`, `VariantRecord.SKU`: changed to `*string` to correctly represent nullable API fields (previously unmarshalled `null` as `""`)
- `output.Category.ParentID`: changed to `*string` without `omitempty` so root categories emit `"parent_id": null` in JSON instead of omitting the field
- `products list --query`: length validation now uses `utf8.RuneCountInString` instead of `len` (byte count), correctly handling multi-byte Vietnamese characters

---

## [0.2.0] — 2026-06-01

### Added

- `products list` — paginated catalog sync with `--updated-since` delta sync, `--ids` UUID filter (max 50), `--all` auto-paginate
- `products create` — simple mode (flags) or JSON mode (`--from-json`)
- `products update <id>` — partial update via flags or `--from-json`
- `products variants <id>` — mixed create/update upsert via `--from-json` (max 50 items)
- `make update-spec` — fetch latest OpenAPI spec from `https://platform.capigo.app/api/openapi`

### Fixed

- Fixed 6 defects in `api/openapi.json`
- `--output json` now renders full product object instead of stripped display model
- HTTP 409 mapped to exit code 8 (SKU conflict)
- Cobra validation errors now respect `--output json` mode

---

## [0.1.1] — 2026-05-23

### Added

- Automated Homebrew Cask publishing to `vtech-com/homebrew-tap` via GoReleaser
- Integration smoke tests with `httptest.NewTLSServer` covering exit-code mapping and header assertions

### Fixed

- Added the generated `go.sum` so the Go module resolves dependencies during local and CI verification
- Fixed version ldflags wiring so `capigo version` prints injected version, commit, and build date
- Updated the golangci-lint config for v2 and fixed lint findings from the first local verify pass

---

## [0.1.0] — 2026-05-23

### Added

- `auth login --key <csk_...>` — save API key to `~/.capigo/config.json`; key value is scrubbed from `os.Args` immediately after parsing to avoid leaking via `ps`
- `auth logout` — remove credentials from the active profile
- `auth whoami` — display the authenticated user (calls `GET /api/v1/me`)
- `config set` / `config get` — manage config values in the active profile
- `config set-default-tenant <code>` / `config unset-default-tenant` — manage the default tenant for the active profile
- `tenants list` — list tenants accessible by the current key; discovered tenant codes are cached in `known_tenants`
- `tasks list` — list tasks with optional `--tenant` / `--no-tenant` scope; supports `--status`, `--parent-task-id`, `--page`, `--limit` filters
- `tasks get <id>` — fetch a single task by ID
- `tasks create` — create a task (`--title` required; tenant required — rejected in global mode because `POST /mission/tasks` requires `tenant_code` in the request body)
- `boards list` — list mission boards
- `version` — print version, commit, and build date (injected via ldflags)
- Global flags on every command: `--tenant`, `--no-tenant`, `--profile`, `--output`, `--api-url`, `--verbose`
- Output modes: `table` (default), `json`, `quiet`; global mode (`--no-tenant`) adds a `Tenant` column to table output automatically
- Standardized exit codes 0–7 for AI agent integration (0 success / 1 general / 2 auth / 3 permission / 4 not found / 5 validation / 6 network / 7 rate limit)
- JSON error format on stderr when `--output json`: `{"error":{"code","message","request_id"}}`
- Automatic `X-Tenant-Code` header injection following resolution precedence: `--tenant` > `--no-tenant` > `$CAPIGO_TENANT` > `config.default_tenant` > global mode
- `X-Request-Id` (UUID) and `User-Agent: capigo-api-sdk/<version> (<os>; <arch>)` headers on every request
- Config file created at `~/.capigo/config.json` with `chmod 0600`; atomic write via temp file + rename
- Multi-profile config schema (version 1) with `active_profile` and per-profile `api_key`, `api_url`, `default_tenant`, `known_tenants`
- `CAPIGO_API_KEY`, `CAPIGO_TENANT`, `CAPIGO_PROFILE`, `CAPIGO_API_URL` environment variable support
- Single binary distribution via GoReleaser for Linux, macOS, Windows × amd64 + arm64; SHA256 checksums in every release
- GitHub Actions CI: lint + test matrix on Linux, macOS, Windows with Go 1.22
- GitHub Actions release workflow: triggered on `v*` tags, runs `goreleaser release --clean`
- CodeQL security scanning workflow
- Dependabot configured for Go modules and GitHub Actions (weekly)

---

[Unreleased]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/vtech-com/capigo-api-sdk/releases/tag/v0.1.0
