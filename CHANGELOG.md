# Changelog

All notable changes to Capigo CLI SDK are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Added

- Body-field coverage guard (`cmd/openapi_body_coverage_test.go`): for each write command (POST/PATCH/PUT), reads the OpenAPI requestBody schema and asserts that every field either has a corresponding cobra flag or the command registers `--from-json` (the generic escape hatch). A documented alias map handles non-obvious renames (`follower_ids`→`follower-id`, `tenant_code`→`tenant`, `assignee_id`→`assignee`, etc.). The `intentionallyUnexposedBodyFields` allowlist documents server-managed fields with no flag. Guards against the class of bugs where a request-body field exists in the spec but is never wired to a CLI flag (e.g. the original `tasks create` `--follower-id` omission). Verified: temporarily removing the `--follower-id` StringArrayVar registration causes the test to fail with a clear message identifying the missing flag.
- New-path detection guard (`cmd/openapi_path_coverage_test.go`): asserts that every path in `api/openapi.json` is listed in either `implementedPaths` (CLI wraps it) or `unimplementedPaths` (deliberately skipped, with rationale). Does NOT enforce 1:1 coverage — the CLI is a curated subset. Fails only when `make update-spec` pulls a new endpoint that is in neither set, requiring a conscious decision. Integrity checks prevent the allowlists from rotting: both sets must be disjoint, and every listed path must actually exist in the spec.
- `tasks list` / `tasks get`: surface task `code` field (e.g. "TASK-123") in output. Added `Code string` to `output.Task`, populated it in `toOutputTask`, and added a `Code` column as the first column in the task table renderer (before Title). JSON and quiet modes unaffected (quiet still emits ID only).
- `boards list`: surface `is_public` and `description` columns. Added `IsPublic bool` and `Description string` to `output.Board`, populated both in the `boards list` mapping, and added `Public` (rendered as "yes"/"no") and `Description` columns to the board table renderer. JSON mode already rendered the full `api.Board`; quiet mode (ID only) unaffected.
- `internal/output.WriteJSONList(w, data, meta)`: shared helper that marshals `{"data":[...],"meta":{...}}` for every list command; forces `data` to `[]` when nil/empty.
- `internal/output.WriteJSONObject(w, v)`: shared helper that marshals a bare object for every single-item command (get, create, update, replace).
- `auth login --output json`: now emits `{"profile":"<name>","status":"logged_in"}` when `--output json` is set, instead of a human string.
- README: added "JSON output contract" subsection documenting the stable machine-readable shape; updated all jq examples that assumed a bare array to use `.data[]`.

### Changed

- **Breaking (JSON shape):** All `list` commands (`tasks list`, `boards list`, `tenants list`, `brands list`, `categories list`, `product-types list`, `units list`, `variants list`, `products list`) now emit `{"data":[...],"meta":{...}}` in `--output json` mode, replacing the former bare JSON array. Callers must change `.[]` → `.data[]` in jq expressions and `json.loads(stdout)` → `json.loads(stdout)["data"]` in scripts.
- `tasks get` / `tasks create`: JSON output now emits the bare full `api.Task` object (no array wrapper), consistent with all other single-item commands. Previously `tasks get` and `tasks create` routed through `output.Render` which wrapped the item in `[{...}]`.
- `config set` / `config get` / `config set-default-tenant` / `config unset-default-tenant`: validation and not-found errors now exit with standard codes (5 / 4) via `output.RenderError`, and use UPPER_SNAKE error codes (`VALIDATION_ERROR`, `NOT_FOUND`, `CONFIG_LOAD_ERROR`, `CONFIG_SAVE_ERROR`). Previously these commands used `os.Exit(1)` with lowercase codes (`config_load_error`, `profile_not_found`, `unknown_key`).
- `--page` default for `boards list`, `brands list`, `categories list`, `product-types list`, `units list`, `variants list` changed from `1` to `0` (meaning "omit → server default"), consistent with `tasks list` and `products list`. The existing `if page > 0` guard means the param is only sent when the flag is explicitly set.
- `products list` / `products list --all`: JSON now uses the shared `WriteJSONList` helper (no behaviour change; logic extracted from the now-deleted `renderProductListJSON`).
- `boards get`: JSON now uses the shared `WriteJSONObject` helper (no behaviour change; logic extracted from the inline `json.NewEncoder`).
- `output.Render` now rejects `json` mode with an error directing callers to `WriteJSONList` / `WriteJSONObject`. Render is the human-facing (table/quiet) path; this enforces the JSON contract by making it impossible for a new command to silently emit the wrong (display-model array) shape. The dead `renderJSON` helper was removed.

### Removed

- `internal/api/paginate.go` and `internal/api/paginate_test.go`: deleted `FetchAll[T]` and its test. The function was dead code — the only caller was its own test; the `products --all` path uses its own inline pagination loop in `cmd/products.go`. Removing the footgun eliminates a latent tenant-propagation bug (the dead `FetchAll` passed `nil` tenant unconditionally).
- `renderProductJSON` and `renderProductListJSON` private helpers in `cmd/products.go` — replaced by the shared `output.WriteJSONObject` / `output.WriteJSONList`.
- `CAPIGO_PROFILE` removed from the README configuration precedence table. The env var was never bound at runtime (the `--profile` flag and runtime profile override were removed in v0.4.0); documenting it was a doc bug.

---

## [0.5.0] — 2026-06-04

### Fixed

- `boards list`: now registers and honors `--page` / `--limit` flags. Previously the command printed the hint "Use --page / --limit to paginate." but rejected those flags with `unknown flag: --page` and never sent pagination params to `GET /mission/boards`, even though the endpoint supports them. All other list commands already paginated; only `boards` was missed.
- `tasks list`: added `--query` / `-q` flag to search tasks by title via the `q` query param on `GET /mission/tasks`. Previously the endpoint supported this param but the command had no way to set it. No length constraints in the OpenAPI spec; the value is passed through as-is.
- `products list --ids`: added client-side validation that rejects more than 50 comma-separated UUIDs before making the HTTP request (exit 5). The OpenAPI spec declares `maxItems: 50` on the `ids` param; the server already enforces this, but the preflight avoids an unnecessary round-trip and produces a clear message.
- `brands list`, `categories list`, `product-types list`, `units list`, `products list`, `variants list`: added client-side `--limit` upper-bound check (maximum 100) matching the `maximum: 100` declared in the OpenAPI spec for all `/pcms/*` list endpoints. Exceeding the limit now exits with code 5 and a clear message before the HTTP call. Mission endpoints (`tasks list`, `boards list`) are unaffected — their spec has no limit maximum.

### Added

- Regression tests (`cmd/pagination_test.go`): assert every list command that prints the pagination hint also registers `--page`/`--limit`, including a dynamic source scan that catches future list commands forgetting the flags.
- Systemic guard test (`cmd/openapi_coverage_test.go`): parses `api/openapi.json` at test time and asserts that every `in:query` parameter of each list endpoint's GET operation has a corresponding cobra flag registered on the matching list command. A documented alias map handles non-obvious renames (`q`→`query`, `filters`→`status`); params deliberately not exposed must be added to the `intentionallyUnexposed` allowlist. Prevents the whole class of "endpoint supports a query param but the CLI never exposes it" bugs.
- `tasks create --follower-id`: repeatable flag (StringArrayVar) to set `follower_ids` on `POST /mission/tasks`. The `FollowerIDs` field already existed in `api.CreateTaskRequest`; it was just never wired to a CLI flag. Use `--follower-id <uuid>` one or more times; the field is omitted from the request body when the flag is not provided.
- `boards get <id>`: new subcommand that calls `GET /mission/boards/{id}` and renders the board detail (id, title, list count). Mirrors `tasks get` in structure. Renders a table with columns ID/Title/Lists in table mode, the full JSON API response in `--output json` mode, and the board ID in quiet mode. Optional `--tenant` flag consistent with `boards list`.

---

## [0.4.0] — 2026-06-03

### Added

- `brands create`: create a brand with `--name` (required) and optional `--logo-url`; supports `--from-json`
- `brands get <id>`: fetch a single brand by ID (GET /pcms/brands/{id}); tenant required
- `brands replace <id>`: full replace (PUT) of a brand; `--name` required, one of `--logo-url` / `--no-logo` required; supports `--from-json`
- `categories create`: create a category with `--name` (required) and optional `--parent-id`; supports `--from-json`
- `categories get <id>`: fetch a single category by ID (GET /pcms/categories/{id}); tenant required
- `categories replace <id>`: full replace (PUT) of a category; `--name` required, one of `--parent-id` / `--root` required; supports `--from-json`
- `product-types create`: create a product type with `--name` (required) and optional `--description`; supports `--from-json`
- `product-types get <id>`: fetch a single product type by ID (GET /pcms/product-types/{id}); tenant required
- `product-types replace <id>`: full replace (PUT) of a product type; `--name` required, one of `--description` / `--no-description` required; supports `--from-json`
- `units create`: create a unit with `--name` and `--abbreviation` (both required); supports `--from-json`
- `units get <id>`: fetch a single unit by ID (GET /pcms/units/{id}); tenant required
- `units replace <id>`: full replace (PUT) of a unit; `--name` and `--abbreviation` both required; supports `--from-json`
- New request models: `CreateBrandRequest`, `UpdateBrandRequest`, `ReplaceBrandRequest`, `CreateCategoryRequest`, `UpdateCategoryRequest`, `ReplaceCategoryRequest`, `CreateProductTypeRequest`, `UpdateProductTypeRequest`, `ReplaceProductTypeRequest`, `CreateUnitRequest`, `UpdateUnitRequest`, `ReplaceUnitRequest` in `internal/api/models.go`
- New client methods: `CreateBrand`, `UpdateBrand`, `CreateCategory`, `UpdateCategory`, `CreateProductType`, `UpdateProductType`, `CreateUnit`, `UpdateUnit` in `internal/api/client.go`
- `api.ProductType` struct: added `Description *string` field (API now returns description on product type responses)
- OpenAPI spec (`api/openapi.json`): added `GET /pcms/{brands,categories,product-types,units}/{id}`, `PATCH /pcms/{brands,categories,product-types,units}/{id}`, and updated `PUT /:id` schemas to require all fields; updated `PublicProductTypeResponse` schema to include `description`

### Changed

- **Breaking:** `--tenant` is now a per-command flag instead of a global flag. Position changes: `capigo --tenant acme products list` → `capigo products list --tenant acme`. Applies to all commands that accept a tenant.
- **Breaking:** `--no-tenant` global flag removed. PCMS commands (`/pcms/*`) always require a tenant and reject requests without one. Mission commands (`tasks`, `boards`) still accept requests without a tenant (API returns cross-tenant results).
- **Breaking:** `--profile` global flag removed. The active profile is always read from `~/.capigo/config.json` (`active_profile` field); runtime override is not supported.
- `brands update <id>`: now uses `PATCH` (was `PUT`) — partial update, at least one field required; supports `--from-json`
- `categories update <id>`: now uses `PATCH` (was `PUT`) — partial update, at least one field required; supports `--from-json`
- `product-types update <id>`: now uses `PATCH` (was `PUT`) — partial update, at least one field required; supports `--from-json`
- `units update <id>`: now uses `PATCH` (was `PUT`) — partial update, at least one field required; supports `--from-json`

---

## [0.3.4] — 2026-06-02

### Fixed

- `variants list`: restore tenant guard — global mode now rejected with exit 5 (was silently passing nil tenant to API)
- `variants list`: `--limit` default corrected from `1` to `20` to match API default
- `products list`: `--query` description now shows `2–500 chars` (max was missing)
- `products variants`: removed invented "Maximum 50 items per call" constraint from help text (not in OpenAPI spec)

---

## [0.3.3] — 2026-06-01

### Fixed

- macOS Gatekeeper: Homebrew Cask now strips quarantine attribute after install so the binary runs without an Apple notarization dialog

---

## [0.3.2] — 2026-06-01

### Fixed

- Remove stale Homebrew Formula from tap; Cask is now the canonical distribution

---

## [0.3.1] — 2026-06-01

### Fixed

- Homebrew distribution: switched from Cask to Formula so `brew install vtech-com/tap/capigo` correctly places the binary in PATH

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

[Unreleased]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.4...v0.4.0
[0.3.4]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/vtech-com/capigo-api-sdk/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/vtech-com/capigo-api-sdk/releases/tag/v0.1.0
