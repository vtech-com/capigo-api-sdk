---
name: capigo-api-sync
description: Developer-side workflow for keeping the `capigo` CLI in the `capigo-api-sdk` repo in step with the Capigo API — refresh the OpenAPI spec, let the coverage guards report the drift, agree the scope with whoever owns the work, verify against a live server, release, and roll the build out to the agent host. Activate ONLY when working inside a `capigo-api-sdk` checkout — changing the SDK's own Go source, its committed spec, or its bundled skill. This is a repo gate, NOT a topic trigger — a request to *use* the CLI against Capigo data belongs to `capigo-api` plus the relevant domain skill, even when it names the same commands. Signals include "thêm command cho capigo CLI", "CLI chưa có field này", "cập nhật spec", "sync SDK với API mới", "release SDK", "make update-spec", "openapi coverage test fail", "add a flag to products list".
compatibility: Needs a `capigo-api-sdk` checkout and its Go toolchain. The verify step additionally needs a reachable Capigo API with a key and a tenant; the rollout step needs SSH access to the host that runs the agent.
---

# capigo-api-sync

The SDK wraps the Capigo Public API. When the API grows a field, a parameter, or
a whole endpoint, the CLI does not learn about it by itself — and neither does
the agent that drives the CLI. This skill is the route from "the API changed" to
"the change is in a released build and the agent uses it correctly".

Two things make this safer than it sounds, and both are worth knowing before you
start:

- **The repo already checks itself.** A committed OpenAPI document is compared
  against the registered commands and flags by tests in `cmd/`. Refresh the
  document and the test suite will tell you what is missing. You do not hunt.
- **The document itself can be wrong.** It is hand-maintained upstream and
  served statically, so it drifts from the routes it claims to describe. There
  is a separate check that asks a live server instead. Use it before you believe
  the document.

## Hard invariants

These are non-negotiable. If you cannot satisfy one, stop and ask — do not
invent a workaround.

- **Never hand-edit the committed spec to make tests pass.** It is the thing
  everything else measures against; editing it to fit the code is forgery.
- **Never delete or loosen a coverage guard to get green.** That is how the
  class of bug the guard exists to catch comes back, with the alarm disabled.
  Deliberate non-exposure has a supported form — see step 3.
- **A spec refresh is committed on its own.** Mixed into a feature commit it is
  impossible to review and impossible to revert.
- **A change to CLI behaviour ships with the matching edit to the repo's bundled
  agent skill, in the same PR.** Nothing in CI compares the two, so a drift here
  is a bug with no alarm on it.

## Ground yourself first

Work from the SDK checkout; every command below is run from its root. Do not
assume a path — find the checkout the way you normally would, and confirm you
are in it (`git remote -v` should show the SDK repo, and a `Makefile` and
`api/` directory should be present). If there is no such checkout on this
machine, ask for it rather than working from a remembered layout or from the
monorepo — this skill has nothing useful to say outside the SDK repo.

Then read, in the repo, rather than trusting this file's summary of them:

- `Makefile` — the real definition of every target named here, including which
  URL the spec is fetched from and what the verify script needs. Targets carry
  `##` comment lines explaining themselves.
- `CONTRIBUTING.md` — house rules, the numbered recipe for adding a command, the
  commit/PR conventions, the release process.
- `AGENTS.md` / `CLAUDE.md` — the must-read list and where logic is allowed to
  live (`cmd/` is glue only; real behaviour lives under `internal/`).
- `docs/` — the living notes, including the running list of API coverage gaps.

If something in this skill disagrees with those files, they win. They sit next
to the code and get updated with it; this file does not.

## The loop

### 1. Find out where the change actually lives

The spec refresh pulls from **production**. An endpoint that has only merged to
the monorepo's working branch is invisible to it.

- Already on production → step 2 works normally.
- Merged upstream but not deployed → read the real route handlers in the Capigo
  monorepo and write the CLI against those. Do not guess where that checkout
  lives or whether one exists on this machine — ask whoever is running the task
  to point you at it, and ask which branch to read while you are there, since
  "merged" can mean several different branches. Inside it, the handlers are the
  files under the platform app's `api/v1` route tree that export HTTP verbs;
  confirm the current layout rather than trusting a remembered path.

  Say in the PR that the change is written against handlers rather than the
  published spec, and that it reconciles on the next spec refresh after deploy.

Do not use the monorepo's checked-in spec file for this judgement. It has been
observed stale in ways that silently narrow the CLI.

### 2. Refresh the spec, and commit that alone

Run the update-spec target. It fetches the published document and pretty-prints
it so the diff is reviewable.

Read the diff before anything else — it is the clearest statement you will get
of what changed. Then commit it on its own, referencing the upstream change
(the repo's history shows the convention).

### 3. Let the tests name the drift — then STOP and agree on scope

Run the full test suite. The coverage guards in `cmd/` compare the refreshed
document against the commands: paths with no command, query parameters with no
flag, request-body fields with no flag or `--from-json` escape hatch.

What comes back is an **inventory of drift, not a work order**. A spec refresh
routinely surfaces more than the change you came for — a parameter added months
ago on an endpoint nobody asked about, a field on a resource this team does not
use yet. Wrapping all of it silently turns a small sync into a sprawling PR that
nobody can review, and quietly commits the team to supporting surface area it
never chose.

**STOP here.** Before writing any command code, put the inventory in front of
whoever owns this piece of work and get a decision. Something like:

> The spec refresh surfaced N gaps. `<the thing we came for>` is clearly in
> scope. Also newly visible: `<gap>`, `<gap>`. My read is A and B are cheap and
> belong in this PR, C is a separate command family and should be its own, D
> looks deliberate and should be recorded as such. How do you want to split it?

Bring a recommendation, not just a list — you have the context from the diff and
they may not. But the split is theirs to make, and each gap has three honest
homes:

- **This PR** — it is what the work is for, or it is a one-flag addition on a
  command that already exists.
- **The gaps document in `docs/`** — real, wanted, not now. Recording it is what
  keeps it from being rediscovered from scratch next quarter.
- **A guard exemption** — deliberately not exposed. The guards have
  `intentionallyUnexposed`-style lists that take an entry **with a written
  reason**; that reason is the whole value, because it tells the next person
  this is a decision rather than an oversight.

Write the agreed split into the PR description. Six months on, "why does the CLI
not expose this?" is answered by that paragraph or by nothing.

### 4. Wire it up

Follow the recipe in `CONTRIBUTING.md` (types → client method → tenant
requirement → command → output columns → tests → docs). Beyond it, two habits
this repo has learned the hard way:

- **Put the truth where the agent will read it.** The CLI's main consumer is an
  LLM agent reading stdout. Counts, the tenant in effect, deletion state,
  "this result is partial" — if such a fact only exists in an exit code, in
  stderr, or in a JSON meta block, the agent will confidently report the wrong
  thing. Ask of every new field: if this matters, does it show up where the
  reader is actually looking?
- **New error codes need a handled meaning.** Errors are rendered from a catalog
  under `internal/api/`. An unexplained code invites the agent to invent a
  cause and a destructive workaround. Give a new code a concrete next action,
  and only give it a hand-written meaning when the server's own message is too
  generic to serve — otherwise you have created a second source of truth that
  will drift.

### 5. Verify against something that cannot lie

- The test suite proves the CLI matches the document.
- The verify-api target proves the document — and the help pages — match a
  running server. It is read-only and needs a reachable API, a key, and a
  tenant; the Makefile comment states exactly which environment variables. Its
  most valuable output is fields the server returns that nothing declared,
  because a field the CLI never prints is a field its caller cannot know exists.

Findings from that second check are drift too, so **STOP again** and give them
the same treatment as step 3: report, recommend a split, let the owner decide.
This is the gate most likely to be skipped under momentum, because the work
feels nearly done by the time you reach it. They are also the findings most
likely to belong upstream rather than here — a field the server returns but the
published spec never mentions is a bug in the spec, and the useful move is
telling the API team, not quietly hand-wrapping it.

Lint runs in CI and is stricter than a local build: unchecked returns from
writes to stdout will fail there and pass on your machine.

### 6. Ship it

Changelog entry, PR, green CI, merge, tag. The release workflow builds and
publishes from the tag, and the package manager formula follows automatically —
check `CONTRIBUTING.md` for the current mechanics rather than doing it by hand.

### 7. Hand it to the agent

The build being released is not the same as the agent using it. Ask where the
agent runs rather than reading it out of the SDK repo — that repo is public and
deliberately names no internal host, and the agent has moved before.

**Find out how the binary got onto that host before you try to upgrade it.** The
two shapes need opposite actions, and doing the wrong one looks like it worked:

- **Installed by a package manager**, natively on the host. Upgrade through that
  channel. If it reports no update, its metadata is probably stale — refresh
  that first.
- **Baked into a container image at build time.** There is nothing to upgrade in
  place. Change the version input in the image definition, rebuild, and recreate
  the container. Installing the new binary into the running container instead
  will work perfectly until the next rebuild silently reverts it.

Then update the skill through whatever mechanism installed *it*. There is more
than one path (the repo's own install target, and the package-based installer
used for published skills); check which one is in place before pushing, so you
do not end up with two copies fighting. On a containerised host, note where each
path writes: an install that lands on the host filesystem is invisible to an
agent running inside a container, and the failure is silent — the old skill just
keeps being used.

Confirm the versions where the agent actually runs — inside the container on a
containerised host, not on the host and not on your laptop. A rebuild that was
never brought up leaves the previous container serving, with a fresh image
sitting unused beside it.

Then the step that is skipped most often and matters most: **have a real
conversation with the agent that exercises the change.** The CLI behaving
correctly under your own hands proves the tool works. It does not prove the
agent reads what the tool now says. Every ergonomics fix in this repo exists
because that gap was real — the truth was on screen and the agent still
reported something else. Ask a person who uses the agent to try the new path and
say what they got back.

## Done means

Guards green · verify-api shows nothing undeclared · the bundled skill matches
the binary · changelog and release out · the host upgraded · someone has seen
the agent handle the new capability correctly in a real exchange.

Anything short of the last two is "merged", not "done". Say which one you mean.

## Traps worth naming

- **Spec sync is not code generation.** Nothing in the repo generates Go from
  the document; the client and its types are written by hand. Refreshing the
  spec moves the measuring stick, nothing else.
- **Green tests after a spec refresh are not always good news.** If you expected
  a new endpoint and nothing failed, the endpoint probably is not on production
  yet — go back to step 1.
- **A rollout that cannot survive a rebuild is not a rollout.** On a host where
  the agent runs from an image, anything installed into the running container —
  the binary, a system package, a Python dependency — disappears the next time
  that image is rebuilt, without a word. The change has to go into the image
  definition to count. This is the one place where "I verified it on the host
  and it worked" is not evidence.
- **The gaps document is a shared memory.** When you close a gap it lists, or
  discover a new one, edit it in the same PR. It is the file people read before
  asking "can the CLI do X yet", and a stale answer there costs someone a day.

## What this skill does NOT do

- **Run Capigo operations.** Creating products, moving tasks, looking up
  members — that is `capigo-api` for CLI mechanics plus the domain skill
  (`manage-capigo-product`, `capigo-taskflow`). This skill only changes the tool
  those skills use.
- **Change the backend.** If the API is missing an endpoint or its spec is
  wrong, the fix belongs upstream. Raise it; do not compensate in the CLI.
- **Generate code from the spec.** See the first trap above.
- **Decide scope.** Steps 3 and 5 hand that back to the owner, with a
  recommendation attached.
