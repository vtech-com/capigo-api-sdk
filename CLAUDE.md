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

The repo ships a full working Go CLI binary (`capigo`). All commands documented in the README are implemented and ship in production. **Do not trust any hardcoded version number in prose — the current version is the latest tag (`git tag --sort=-creatordate | head -1`) and the top dated entry in `CHANGELOG.md`.** New milestone plans are added under `docs/tasks/` over time; treat the highest-numbered plan as the current scope of work.

## Agent ergonomics — output design rule

The primary consumer of this CLI is an AI agent (Tấm, DeepSeek-based) that reads **stdout** and routinely forgets its instruction skill. Settled principle: **truth must be salient on stdout** — totals, the resolved tenant, deleted state, partial/incomplete results, and missing-ID reconciliation are printed where the agent actually looks (on stdout, in the JSON document itself — salient `data`/`meta`/`error` fields), never only on stderr, in an exit code, or in documentation. When changing any command's output, ask: *if an agent read only stdout, could it reach a confidently wrong conclusion?* If yes, fix the output, not the manual. (Background: a stderr-only pagination nudge made the agent report a 43-brand tenant as "20" — see CHANGELOG v0.11.0–v0.13.0.)

## Bundled skill stays in sync

`skills/capigo-api/` is the agent-facing operating manual for this CLI and is distributed straight from this repo (`npx skills add vtech-com/capigo-api-sdk --skill capigo-api`). **Any PR that changes CLI behavior (flags, output, exit semantics) must update the skill in the same PR** — a skill that describes the binary wrongly is itself a bug, and no CI check catches the drift.

## Language

All artifacts written to disk in this repo are in **English** — code, comments, commits, docs, plans, CHANGELOG entries. Chat replies may mirror the user's language; files on disk may not.

## Where to put new logic

`cmd/` contains CLI glue only (flag parsing, arg validation, dispatching to `internal/`). All real logic — HTTP, tenant resolution, error mapping, output formatting, config I/O — lives under `internal/`. If you're about to write business logic inside a `cmd/*.go` file, stop and move it to `internal/`. See `CONTRIBUTING.md` for the full structure rule.

## Spawning agents / teammates

When spawning agents (via the `Agent` tool, `TeamCreate`, or any subagent), **default the model to Sonnet** (`model: "sonnet"` parameter) unless the user explicitly asks for a different model (Opus, Haiku, etc.) or the task obviously demands it (e.g. heavy reasoning that justifies Opus). Sonnet is the team default for this project — cheaper and fast enough for the Go work, doc edits, infra files, and review passes we run here. Pass `model: "sonnet"` on every `Agent` spawn.
