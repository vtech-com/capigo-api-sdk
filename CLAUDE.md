# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Must-read documents

Before doing anything non-trivial in this repo, read these in order. Do not restate or summarize them here — read the actual files.

1. **`docs/project-context.md`** — the authoritative product spec (goals, architecture, UX contract, security model, roadmap). When this file disagrees with anything else, it wins.
2. **`api/openapi.json`** — source of truth for every endpoint, schema, parameter, and tenant requirement. When the spec narrative and the OpenAPI file disagree, OpenAPI wins.
3. **`docs/tasks/0.1-implementation-plan.md`** — the current milestone plan. **This is only the first plan; expect follow-up plans (`0.2-*`, `0.3-*`, …) as the SDK evolves.** Always look for the latest `docs/tasks/*-implementation-plan.md` to know what's actively being built.
4. **`README.md`** — user-facing install, quickstart, commands, exit codes, and AI-agent integration examples.
5. **`CONTRIBUTING.md`** — branch/PR workflow, coding conventions, commit format, how to add a new command, release process.
6. **`CODE_OF_CONDUCT.md`** — Contributor Covenant; applies to all interactions.
7. **`SECURITY.md`** — vulnerability disclosure; do not open public issues for security bugs.
8. **`CHANGELOG.md`** — Keep a Changelog format; update `[Unreleased]` for non-trivial changes.

If you find yourself guessing at a rule (tenant headers, exit codes, output shape, command layout), the answer is in one of the files above — go read it instead of inferring.

## Repository state

The repo is currently **pre-code**. There is no Go source yet — only specs and boilerplate. The plan to bring up the first working binary lives in `docs/tasks/0.1-implementation-plan.md`. Subsequent milestone plans will be added under `docs/tasks/` over time; treat the highest-numbered plan as the current scope of work.

## Language

All artifacts written to disk in this repo are in **English** — code, comments, commits, docs, plans, CHANGELOG entries. Chat replies may mirror the user's language; files on disk may not.

## Where to put new logic

`cmd/` contains CLI glue only (flag parsing, arg validation, dispatching to `internal/`). All real logic — HTTP, tenant resolution, error mapping, output formatting, config I/O — lives under `internal/`. If you're about to write business logic inside a `cmd/*.go` file, stop and move it to `internal/`. See `CONTRIBUTING.md` for the full structure rule.
