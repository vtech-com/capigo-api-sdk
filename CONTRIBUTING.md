# Contributing to Capigo CLI SDK

Thank you for your interest in contributing! This guide covers everything you need to get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Before You Start](#before-you-start)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Making Changes](#making-changes)
- [Commit & PR Guidelines](#commit--pr-guidelines)
- [Adding New Commands](#adding-new-commands)
- [Testing](#testing)
- [Release Process](#release-process)

---

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold it.

---

## Before You Start

**For bug fixes and small improvements:** feel free to open a PR directly.

**For new features or significant changes:** please open a GitHub Issue first and wait for a maintainer to apply the `approved` label. This avoids wasted effort on PRs that won't be accepted.

**For new command bindings** (wrapping additional API endpoints): check `api/openapi.json` for the endpoint contract, then open an issue to discuss scope before implementing. Source: `https://platform.capigo.app/api/openapi`. Run `make update-spec` to sync.

---

## Development Setup

### Prerequisites

- Go 1.22+
- `golangci-lint` (`brew install golangci-lint` / [install guide](https://golangci-lint.run/usage/install/))
- `make`

### Clone and build

```bash
git clone https://github.com/vtech-com/capigo-api-sdk.git
cd capigo-api-sdk
go mod download
make build
```

### Common make targets

```bash
make build       # Build binary to ./dist/capigo
make test        # Run unit tests
make lint        # Run golangci-lint
make fmt         # Format code (gofmt + goimports)
make check       # lint + test (run before pushing)
make clean       # Remove build artifacts
```

### Running against a local API

```bash
CAPIGO_API_URL=http://localhost:3999 ./dist/capigo tasks list
```

---

## Project Structure

```
cmd/            Cobra commands — one file per command group
internal/
  api/          HTTP client, error mapping, response types
  config/       Read/write ~/.capigo/config.json
  output/       table | json | yaml | quiet renderer
pkg/            Public packages (empty in Phase 1)
api/
  openapi.json  OpenAPI spec — source of truth for all endpoints
examples/       Sample scripts (bash, python, n8n)
docs/           Additional documentation
```

**Key rule:** `cmd/` files contain only CLI glue (flag parsing, arg validation, calling `internal/`). Business logic lives in `internal/`.

---

## Making Changes

1. Fork the repo and create a feature branch from `main`:
   ```bash
   git checkout -b feat/your-feature-name
   ```

2. Make your changes. Follow the [coding conventions](#coding-conventions) below.

3. Add or update tests for any new logic.

4. Update `CHANGELOG.md` under the `[Unreleased]` section (see format below).

5. Run `make check` — all checks must pass before opening a PR.

6. Open a PR against `main`.

### Coding conventions

- Standard Go formatting — `gofmt` enforced by CI
- Error messages: lowercase, no period at end (Go convention)
- Exit codes: always use the constants in `internal/api/errors.go` — never hardcode integers
- Output: all user-facing output goes through `internal/output/formatter.go`; never `fmt.Println` directly in commands
- Tenant handling: follow the precedence rules in `internal/config/store.go` — do not duplicate logic in command files
- Secrets: never log or print API key values, even in `--verbose` mode

---

## Commit & PR Guidelines

### Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add tasks update command
fix: correct exit code for rate limit errors
docs: update quick start example
chore: bump go-pretty to v6.5.4
test: add pagination edge case coverage
```

### PR requirements

- [ ] All CI checks pass (lint + tests on Linux, macOS, Windows)
- [ ] Tests added for new logic
- [ ] `CHANGELOG.md` updated under `[Unreleased]`
- [ ] No secrets or internal data in the diff
- [ ] PR description explains **what** and **why** (not just what the diff shows)

PRs are squash-merged — one commit per PR lands on `main`.

### CHANGELOG format

We use [Keep a Changelog](https://keepachangelog.com/) format. Add your entry under `## [Unreleased]`:

```markdown
## [Unreleased]

### Added
- `tasks update` command to update task title, status, and assignee (#42)

### Fixed
- Exit code for rate-limit errors now correctly returns 7 instead of 1 (#38)
```

---

## Adding New Commands

To wrap a new Capigo API endpoint:

1. **Verify the endpoint** exists in `api/openapi.json`. Do not call endpoints not in the spec.

2. **Add response types** to `internal/api/models.go` if needed.

3. **Add the HTTP method** to the appropriate client file in `internal/api/`.

4. **Add tenant requirement** metadata if the endpoint requires a tenant (see the table in `internal/api/` for the pattern).

5. **Create or extend** the command file in `cmd/`. Add the subcommand to the parent in `cmd/root.go`.

6. **Add table columns** to `internal/output/formatter.go` for the new resource type.

7. **Write tests** for the new client method and command flags.

8. **Add an example** to `docs/commands/` and optionally `examples/`.

---

## Testing

```bash
# All tests
go test ./...

# A specific package
go test ./internal/api/...

# With verbose output
go test -v ./cmd/...

# With race detector
go test -race ./...
```

Tests must not make real HTTP calls. Use the `httptest` package to mock the Capigo API. See existing tests in `internal/api/` for the pattern.

---

## Release Process

Releases are managed by maintainers via [GoReleaser](https://goreleaser.com/) and GitHub Actions.

1. Update `CHANGELOG.md` — move `[Unreleased]` entries to a new versioned section.
2. Create and push a tag: `git tag v1.2.0 && git push origin v1.2.0`
3. The `release.yml` workflow builds and publishes binaries automatically.

If you notice the project is missing a tag or a release is broken, open an issue rather than trying to trigger releases yourself.

---

## Questions?

Open a [GitHub Discussion](https://github.com/vtech-com/capigo-api-sdk/discussions) or file an issue. We try to respond within 48 hours.
