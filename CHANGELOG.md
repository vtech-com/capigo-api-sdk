# Changelog

All notable changes to Capigo CLI SDK are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added

- `products list` — list products from PCMS catalog with optional `--updated-since` (ISO 8601) for delta sync, `--page`, `--limit`, and `--all` (auto-paginate). Tenant required; server timestamp surfaced to stderr for subsequent delta sync calls.
- `products list --ids <uuid,...>` — fetch up to 50 specific products by UUID (comma-separated); composable with `--updated-since`; mutually exclusive with `--all`
- `products create` — create a new product via individual flags (simple mode) or `--from-json <file|->` (variant mode with options+variants array); extended `Long` help with concrete JSON examples for both simple and variant modes
- `products update <id>` — update product core fields (name, description, status, currency, brand/category/product-type/unit IDs); now also supports `--from-json <file|->` for full JSON body (mutually exclusive with individual field flags)
- `products variants --product-id <uuid> --from-json <file|->` — upsert variants via JSON array; items with `variant_id` are updated, items without are created (max 50 per call)
- `internal/api/models.go`: `CreateProductRequest`, `CreateProductOptionItem`, `CreateProductVariantItem`, `UpdateProductRequest`, `UpsertVariantItem` structs for PCMS write endpoints
- `internal/api/errors.go`: product-domain error code constants (`E9417`–`E9447`, `E9103`, `E0004`, `E0102`)
- `internal/api/client.go`: `Response.ServerTime` field populated from `X-Server-Time` response header (enables delta sync checkpoint)
- `internal/output/types.go`: `Product` display type with `VariantCount` field
- `internal/output/formatter.go`: `"product"` renderer (columns: ID, Name, Status, SKU, Price, Variants)
- Exit code 8 for HTTP 409 Conflict errors; `ExitCodeFor` updated; README exit-code table updated

### Fixed

- `products list/create/update/variants --output json`: JSON output now renders the full `api.Product` struct (all fields) instead of the stripped 5-field `output.Product` display model; list output uses a `{"data":[...],"meta":{...}}` envelope so empty results include pagination context rather than a bare `[]`
- `products list --output json` empty result: returns `{"data":[],"meta":{"page":1,"limit":20,"total":0,"has_more":false}}` instead of `[]`
- `X-Server-Time` header: now always printed to stderr regardless of output mode (was previously suppressed in `--output json` mode)
- Cobra arg-validation errors (e.g. missing positional arg): now rendered via `output.RenderError` respecting `--output json` and mapped to exit code 5 instead of printing plain text and exiting with code 1
- `--output yaml` and any other unknown format: now returns a clear error (`unknown output format "yaml": supported formats are table, json, quiet`) instead of silently falling back to table
- `internal/output/formatter.go`: product table renderer now includes a `Variants` column showing the total variant count; `VariantCount` added to `output.Product`
- `README.md`: removed `yaml` from output mode list (never implemented); updated `--output` flag description and exit-code table
- `api/openapi.json`: fixed 6 defects — path double-prefix on 3 write endpoints, broken `$ref` to non-existent `Product`/`ErrorResponse` schemas, missing `X-Tenant-Code` parameter on write endpoints, incorrect lowercase status enum on `PublicProductResponse`, `variant_id` incorrectly required in variants upsert body; added `option1/2/3` fields to create-variants schema

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

[Unreleased]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/vtech-com/capigo-api-sdk/releases/tag/v0.1.0
