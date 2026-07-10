# Changelog

All notable changes to Capigo CLI SDK are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning follows [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Changed

- **Rebuilt the bundled `capigo-api` skill around live CLI discovery.** The skill now begins
  with `capigo version`, recommends an official installation path when the binary is missing,
  and treats root/group/leaf `--help` plus the cross-cutting help topics as the authoritative
  command reference. Removed the duplicated command catalogue and 561-line flag reference in
  favor of a compact stable-conventions reference and a single special-case guide for simple
  versus options/variants product creation. The public OpenAPI specification is now consulted
  only for explicit API work, SDK development, version diagnosis, or a help-confirmed CLI gap.

- **`api/openapi.json` synced to production (v1.26.1).** 25 paths became 39; 57 operations in all.
  Nothing was removed and no existing operation changed shape, so nothing the CLI calls today
  breaks. What arrived: a read-only **WMS** module (warehouses, inbound receipts, outbound
  shipments, internal transfers — list and by-`code` for each), **task addressing by `code`**
  rather than UUID, `GET /pcms/variants/sku/{sku}`, and a `GET` beside the `POST` on
  `/mission/tasks/{id}/subtasks`.

  Sixteen of the fifty-seven operations are unwrapped, each recorded with a reason in
  `unimplementedOps`. None is a gap in the platform; each is a decision this CLI has not made yet.

- **The OpenAPI coverage guard counts operations, not paths.** It used to track "paths not
  method+path pairs" — by design, and documented as such — so a verb appearing on a path the CLI
  already called was invisible to it. That is exactly what happened: prod added
  `GET /mission/tasks/{id}/subtasks` beside the `POST`, and the guard stayed green while a
  capability went unwrapped.

  The guard now enumerates every `METHOD path` in the spec and fails until each is either wrapped
  or listed with a reason. A second test rejects a reason too short to be one.

### Changed — BREAKING

- **`meta` passes the API's own keys through too.** The passthrough rule was applied to `data` and
  forgotten for `meta`. `GET /mission/boards/{id}` sends `list_count`; the CLI dropped it, and every
  field check said "match" — because every field check compared `data`.

  `api.Meta` is deleted with the habit. Pagination was never the CLI's to model: `page`, `limit`,
  `total` and `has_more` are four of the API's meta keys, and they arrive exactly the way
  `list_count` does. The CLI adds `tenant`, `tenant_source` and `server_time`, and merges the rest
  untouched. `verify-api` now checks a page's sample against the API's meta as well as its data, and
  says `SAMPLE OMITS meta.list_count` when it should.

- **The CLI stops re-modelling the API's responses.** Every command decoded the body into a Go
  struct and marshalled it again. That silently dropped every field the struct did not declare,
  and invented every field it declared that the API did not send.

  Both happened, and neither was announced. `tasks get` discarded `parent` and printed a
  `parent_task_id` no response has ever carried. `variants list` truncated its rows to five
  fields, so `manufacturer_code` was readable through `variants get` and absent through
  `variants list` — an agent looking one up through the list would have concluded the product
  had none.

  `data` is now passed through byte for byte. Measured against a running API across nine
  endpoints: zero fields lost, zero invented. Every field the platform adds from now on appears
  in this CLI without a release.

  This is the same defect the display models had, one layer down. Deleting `output.Product` and
  friends removed the second copy of every resource; `internal/api/models.go` was the first.
  Fifteen response types and the generic `Envelope[T]` are gone with it. Request bodies stay —
  the CLI builds those, so it is entitled to a type for them.

  A test now forbids `api.Envelope[` and `Data api.` in any command file. Reading a value the
  command's own logic needs — an id, a tenant code — goes through a narrow local struct that
  never decides what the caller sees.

- **One output shape. Table and quiet output are gone, and so is `-o/--output`.** Every command
  that succeeds prints exactly one thing to stdout:

      { "data": …, "meta": { … } }

  `data` is an array for a list and an object for a single item. There is no flag to ask for
  another shape, and no default to reason about.

- **The CLI no longer unwraps the API's envelope.** `get`, `create`, `update`, `replace` and
  `products variants` used to print a bare object, so `.id` worked on them and `.data[].id` on a
  list. Now every command answers to `.data`. **This breaks every caller that reached for `.id`.**

  The unwrapping was deliberate, and it cost more than it saved: two shapes meant every page had
  to say which one it emitted, and nineteen of them said it wrongly.

- **`meta` names the tenant.** `meta.tenant` and `meta.tenant_source` (`flag`, `env`, `config`)
  are on every response from a tenant-scoped command, read or write.

  This is the one fact the old table mode carried that JSON did not. It used to be a line of
  prose printed in one mode out of three:

      Tenant: acme (from CAPIGO_TENANT)

  A write into the wrong tenant is not an error the server can raise — the request was valid, and
  it succeeded. The only defence is a caller that can *read* which tenant it hit, so the fact now
  lives in the shape a caller parses rather than in prose it might notice.

- **`meta.server_time`** replaces the `Server time:` line. It is the CLI's, not the API's — the
  value comes from a response header the caller never sees.

  `meta` now carries only what a caller cannot work out for itself. The tenant comes from a
  resolution order internal to this CLI; the server clock comes from that header. The old
  `Requested N ids · missing:` line and the `INCOMPLETE — aborted at page N` footer became
  `meta.missing_ids` and `meta.complete` for one release, and are gone: a caller holds the ids it
  passed to `--ids` and the ids that came back, so the difference is its own subtraction, and a
  sweep that aborted is already announced by a non-zero exit.

- **`products list --ids` exits 4 when a requested id does not come back.** It used to exit 0 with
  fewer rows, which answers a different question than the one asked. The rows that did come back
  are printed first — they are real — and the exit code carries the shortfall.

- **stdout carries exactly one JSON document.** A command that had already printed its envelope
  and then failed — a `--all` sweep aborting part-way — appended an error object to it, so
  `json.load` on the pair raised `Extra data`: the precise failure the single-shape contract
  exists to remove.

  A command that fails after fetching real rows now prints one document carrying both, error
  first: `{"error": {…}, "data": […], "meta": {…}}`. The rows are not discarded — an `--all`
  sweep that aborts on page 41 still holds forty real pages — and they are not printed under a
  clean envelope either, because a `--ids` result missing two ids reports `"total": 1` and
  `"has_more": false`, which is exactly what success looks like.

  **The presence of the `error` key is the completeness test.** A caller never has to consult the
  exit code, or read stderr, to know it is holding a prefix of the truth.

- **A failure prints JSON on stdout** — `{"error": {…}}`, carrying `code`, `message`, `meaning`,
  `next`, `capability_note`, `raw`, `request_id` and `http_status` — plus the one-line summary on
  stderr. A caller may now parse stdout unconditionally: a failed command yields a diagnosis, not
  a parse error stacked on top of an API error.

- **`is_deleted` is the only deletion marker.** The `ACTIVE (DELETED)` status suffix was a table
  artifact. `status` alone never revealed deletion and still does not.

### Removed

- **The `-o json` nudge on stderr, and `CAPIGO_NO_HINTS`.** The nudge existed to catch a caller
  redirecting table text and parsing it as JSON. stdout is JSON now, so redirecting it is always
  correct and there is nothing left to warn about. `cmd/output_hint.go` is deleted.

- **`internal/output`'s table renderer, quiet renderer, display models, and list-summary
  footers** — and with them the `go-pretty` dependency. The package is one file: an envelope and
  an error shape.

  The display models were a second definition of every resource, maintained in parallel with the
  API's own. Deleting them deleted a class of bug: a field present in the API response, absent
  from the display struct, and therefore invisible in a mode the caller happened to be using.

### Added

- **Cross-cutting help topics.** `capigo help tenancy`, `help output`, `help exit-codes`,
  `help soft-delete` and `help versioning` are documentation pages that ship inside the binary.
  Cobra lists them under "Additional help topics"; they have no `Run` and never execute.

  They exist so that a fact true of many commands is stated in exactly one place, and referenced
  from a command page rather than restated on it. Restating a cross-cutting contract on every
  command page is precisely how the `Response: {"data": …}` error below reached nineteen pages.

  `help versioning` publishes the OpenAPI specification URL and states the relationship plainly:
  a capability present in the spec and absent from this CLI is a gap in the CLI, not in the
  platform. Withholding that link never prevented a caller from reaching the API directly — the
  key, the base URL and `--verbose` were always at hand — it only made the resulting bug report
  less accurate.

- **Every help page now ends with the build it came from** — `capigo <version> (built <date>) ·
  capigo help versioning`. Help ships inside the binary, so a page cannot disagree with the
  binary that printed it; it can, however, describe an older surface than the one deployed. The
  footer lets the reader tell which. A local `go build` omits the meaningless `(built unknown)`.

- The root help lists the command groups, the global flags, and the topics. The group list and the
  flag list are generated — from the tree and from the flag definitions — because hand-writing
  either would be a second copy of it.

- **Every command page rebuilt on a four-section shape** — `PURPOSE · USAGE · FLAGS · OUTPUT`, in
  that order, which is the order a caller needs them in: what is this for, how do I spell the
  call, what does each flag do, what comes back.

  `USAGE` is a git-style synopsis, so bracket grammar carries what a sentence used to:
  `[--ids <uuid,...> | --all]` says the two cannot be combined. `FLAGS` absorbed the old `INPUT`
  section, which was the flag list under another name, and each flag now carries its own examples
  and its own traps — a caveat about `--query` is met while deciding whether to use `--query`.
  `OUTPUT` shows the real table and the real JSON rather than describing them.

  Cross-cutting mechanics (the JSON envelope, list footers, stream placement, tenant resolution,
  exit codes, soft-delete) are referenced from the help topics rather than restated.

- **Cobra no longer appends `Usage:` and `Flags:` blocks to a command page** (`cmd/help_render.go`
  replaces the help function). Those blocks restated, in different words, what the page had just
  said — the two copies of `--query`'s length limit had already drifted to `2-500` and `2–500`.
  A command page is now its `Long` text and the build footer, and nothing else; a group page adds
  the generated list of its children.

### Removed

- **`GETTING STARTED` from the root page, and `CAVEATS`, `EXAMPLES` and `SEE ALSO` from every
  command page.** They were written for a reader who might not have read a skill file, not for a
  reader of CLI help. A caveat now lives in the flag it qualifies, an example beside the flag it
  demonstrates, and the command tree already lists the siblings that `SEE ALSO` pointed at.

  Facts newly written down, each verified against the code or the API rather than inherited from
  the previous text: `products variants` returns the whole product, not the variants you sent;
  it is not atomic, so a failed call may have applied some items; an omitted field is left
  unchanged while an explicit `null` clears it; table mode never surfaces `variant_id`;
  `option1..option3` are positional against the product's `options[]`; `--aliases` and `--tags`
  replace the array rather than append to it.

- **Reference-data help pages rebuilt on the same shape** — `brands`, `categories`,
  `product-types` and `units`, twenty pages across five verbs. Each now states the object it
  returns.

- **`tasks` help pages rebuilt on the same skeleton**, including the two attachment-download
  commands. `tasks list`, `tasks get` and `tasks create` had no long help at all.

  Three output shapes that no page described are now written down: `tasks create` returns a bare
  task, *unless* `--subtasks-json` is given, in which case it returns `{task, subtasks[]}`;
  `tasks subtasks` returns `{parent_task, subtasks[]}`, which is neither a bare object nor a list
  envelope. A timeline entry's `kind` has four values — `comment`, `activity`, `card`,
  `artifact` — where the page named two.

- **The remaining groups rebuilt on the same shape** — `boards`, `members`, `tenants`, `auth`,
  `config`, `health`, `version`.

  `config get`, `config set`, `version` and `auth logout` say plainly that they ignore
  `--output` and print the same text in every mode — verified by running each. An agent that
  reaches for `.data` on their output is reaching for something that was never there.

- **Commands are listed in the order a caller meets them**, not alphabetically: `list` before
  `create`, read before write (`cobra.EnableCommandSorting = false`). `help` and `completion`,
  which document the CLI rather than the API, no longer sit among the domains.

- **Five tests hold the shape in place**, so it survives the next person who adds a command.
  They assert the four sections are present, as bare headers, in order; that no retired heading
  reappears; that `FLAGS` documents every flag the command defines *and* names no flag it does
  not; and that `USAGE` spells the command it belongs to. The flag checks matter more now that
  cobra prints no flag table: a flag missing from `FLAGS` is a flag documented nowhere.

### Added

- **`make verify-api`.** Every guard in this repo checks the CLI against `api/openapi.json`. Nothing
  checked the document against the API — and the document is hand-written and served statically, so
  it drifts from the routes beside it.

  The target calls a running server and reports, per endpoint, the fields the spec omits (`EXTRA`)
  and the fields it declares that never arrive (`MISSING`). Against the dev API it finds three:
  `description` on product-types, and `manufacturer_code` / `legacy_code` / `extra_data` on variants.
  It also names the two paths the CLI calls that the spec has never heard of — `/health`, which
  answers 200, and `/me`, which answers 404 because no such route exists.

  A second pass checks each help page's `OUTPUT` sample against a real response — thirty-five pages,
  every one that reads a record. It caught two. `variants get` never showed `product`, the nested
  reference that lets a lookup by sku reach the parent product without a second call. `products
  list` named nine of a product's seventeen fields and said nothing about the eight it dropped.
  Every other check had said "match", because every other check compared the CLI to the API and
  never the page to either.

  A page may defer — "the same shape as `products get`" — rather than copy a sample that would then
  drift from it. The guard follows the pointer, because stating a fact once is this repo's rule and
  a guard that punishes it would teach the wrong lesson.

  Forty-five of the forty-nine command pages are checked against a real response. Three are exempt:
  `config set`, `config set-default-tenant` and `config unset-default-tenant` print nothing on
  success, so a sample there would be the lie. `auth login`, `auth logout` and `config get` run under
  a throwaway `HOME` — a script that edited the operator's credentials to check a help page would be
  a poor trade. The two `attachments download` commands are driven for real, into a throwaway
  directory.

  One page is left: `auth whoami`, which calls an endpoint the API does not implement.

  A field the server returns that the spec never declared is reported, and does not fail the run:
  that is the document's defect, and a target that is always red is a target nobody runs. A field
  the spec promises and the server never sends does fail — a page written from the document would
  describe something that never arrives.

- **`tasks subtasks create` said `parent_task` is "trimmed — id, code, and title only, not the full
  task shape".** It is the full task. The OpenAPI document declares the trimmed shape, and the page
  followed it; the handler builds the parent with `toPublicTaskResponse`, and the server returns all
  sixteen fields with `has_subtasks` now true. Confirmed by minting a temporary owner key, posting one
  subtask, reading the response, and deleting both. The page now says which of the two is wrong.

  Read-only. It makes no writes and creates nothing.

- **`tasks subtasks list` — and `tasks subtasks` becomes a group.** Reading a task's children and
  creating them are separate calls to the API, so they are separate commands: `tasks subtasks
  list` and `tasks subtasks create`. Both take `<parent-id>` or `--code`.

  **BREAKING:** `tasks subtasks <parent-id> --title …` is now `tasks subtasks create <parent-id>
  --title …`. One command carrying two verbs, switched by a flag, is a command whose behaviour a
  reader cannot predict from its name.

  `tasks subtasks list` is not `tasks list --parent-task-id <id>`, though it was once dismissed
  here as a duplicate of it. Against a running API: for a parent that does not exist, the endpoint
  exits 4 while the filter returns 200 with zero rows — so through the filter, *"this task has no
  subtasks"* and *"there is no such task"* are the same answer. It also returns every subtask in
  one response, and stamps each row with the `parent` reference the filter does not add.

  The pagination in its `meta` is nominal — `page` is always 1, `has_more` always false, `limit` is
  simply the row count — and the page says so, because a caller who pages it will page forever.

- **Tasks addressed by their code: `--code` on five commands.** `tasks get`, `tasks comments`,
  `tasks subtasks`, and both `attachments download` commands now take `--code ACMEC-68` in place of
  a UUID, reaching the `/mission/tasks/code/{code}/…` routes.

  One flag on each existing command, not five new commands — the same principle `variants get
  --sku` set. A bare argument is never sniffed: give an id or `--code`, never both, and never
  neither. Both addresses are path-escaped.

  **`--code` requires a tenant, and the CLI says so rather than letting the server decide.** A task
  code is unique within a tenant, not across them, and `loadAccessibleTaskByCode` rejects a lookup
  that resolves to none. Whether it resolves depends on the API key: a *tenant-scoped* key carries
  a tenant even with no header, a *global* key does not, and the CLI cannot see which it holds. So
  the tenant is demanded up front, with a message that explains why, instead of a 400 that arrives
  for some keys and not others.

  `PATCH` by code is deliberately absent from the API, so `tasks update --code` does not exist.
  `GET /mission/tasks/code/{code}/subtasks` stays unwrapped, alongside its by-id twin.

- **`variants get --sku <sku>`.** A variant has two addresses — its id, and its sku, unique within
  a tenant — and `GET /pcms/variants/sku/{sku}` returns the same record as `GET /pcms/variants/{id}`,
  field for field, verified against a running API.

  So this is one command with two addresses, not two commands: `capigo variants get (<id> | --sku
  <sku>)`. Giving both, or neither, exits 5. A bare argument is never guessed at — sniffing whether
  it looks like a UUID would send a sku shaped like one to the wrong endpoint, quietly. The sku is
  path-escaped, so a sku containing a slash addresses a variant rather than a different route.

### Fixed

- **README's Go-install instructions now state the real installed binary name.** `go install
  github.com/vtech-com/capigo-api-sdk@latest` produces a binary named `capigo-api-sdk` — Go
  names it after the module path — not `capigo` as the README previously claimed. The section
  now shows the rename step alongside the install command.

- **`capigo help soft-delete` no longer refers to "table footers" in its SEE ALSO line.** The
  CLI has only ever had one output mode — JSON on stdout — so a footer belonging to a mode that
  never shipped could not be a real cross-reference.

- **`capigo tenants list` reported `"total": 0` beside a row of data.** `GET /tenants` sends no
  meta — the OpenAPI document says so, correctly — but `api.Envelope.Meta` was a value type, so an
  absent meta decoded into a meta of zeros and the CLI printed them as fact.

  This CLI's own rule, stated in its help and in the bundled skill, is: *read `meta.total`, do not
  count `data[]`*. A caller following that rule concluded it had access to no tenants. The rule was
  right; the number was invented.

  `Envelope.Meta` is now `*Meta`, so absence is representable, and `listMeta` emits no pagination
  when there is none. A real `total: 0` still survives — that is the answer for an empty tenant,
  and it must not be dropped along with the fabricated one.

- **Mission endpoints cap `--limit` at 50, not 100.** Measured against a running API: `tasks list`,
  `tasks comments`, `boards list` and `members list` reject 51 with exit 5, while the PCMS lists
  accept up to 100. The OpenAPI document declares no maximum at all. Three of those four pages
  never mentioned a bound.

- **The `replace` pages describe PUT correctly, at last.** They used to say a field you do not
  send is "reset rather than preserved". Then, on the authority of the OpenAPI document — which
  declares no required body fields and says "At least one field must be provided" — they were
  rewritten to say the opposite: that PUT changes only what you send, like PATCH.

  Both were wrong, and the second more so. Against a running API:

      PUT /pcms/brands/{id}  {"name":"x"}                  → 400  VALIDATION_ERROR "Required"
      PUT /pcms/brands/{id}  {"name":"x","logo_url":null}  → 200
      PATCH /pcms/brands/{id} {"name":"x"}                 → 200

  PUT is a true replace, enforced by the server: every field must be present, and a nullable one
  must be sent as `null` rather than omitted. `apps/platform/src/lib/api/dto/brand-write-params.ts`
  says so in as many words — `// PUT (full replace) — all fields required`. The same holds for
  categories, units and product-types. The published OpenAPI document is wrong on this point, and
  reading its prose as behaviour is what produced the second error.

  `products` is the exception: its `PUT /pcms/products/{id}` parses with the *partial* schema, so
  the same verb means "full replace" on reference data and "partial update" on products.

  The CLI itself was never wrong — `replace` has always required every field flag, which is
  exactly what the server demands. Only the pages lied. (So did nothing in the bundled skill,
  which has said `PUT — all fields required` throughout.)

- **`auth whoami` calls an endpoint that does not exist.** The pages said `/me` "is not deployed
  on production". There is no `/me` route in the API at all — the request reaches the platform's
  not-found handler and returns HTML. The command exits 4 on every call, and its page now says
  so, rather than describing a success it cannot produce.

- **`product-types` responses carry `description`.** Every sample on those five pages omitted it,
  because the OpenAPI document's `PublicProductTypeResponse` declares only `id` and `name`. The
  server returns the field; the document is incomplete. The pages now show it, and say why.

- **`products update` was labelled PATCH and sends PUT.** `/pcms/products/{id}` exposes only
  `get` and `put`; there is no PATCH to send. The word had been copied from the reference-data
  groups, which do have both.

- **`config set` never accepted `default_tenant`.** Its key switch handles `api_url` and
  `default_profile`, and anything else exits 5 — yet the group page and the `set` page both
  listed `default_tenant` among the recognised keys. It is settable only through
  `config set-default-tenant`. (`config get` does read all three, so that page was right.)

- **`config set`, `config set-default-tenant` and `config unset-default-tenant` print nothing on
  success.** All three pages promised "a one-line confirmation". Only `config get` prints. The
  pages now say: exit 0 and silence, confirm with `config get`.

- **`config set-default-tenant` does not validate the tenant.** It makes no API call and stores
  the code as given, while the old help called it "the same thing, with tenant validation".

- **`tenants list` prints no summary footer**, unlike every other list command. Its page said
  nothing, so a reader who had learned the pattern from `capigo help output` would look for a
  `Total:` line that is never printed.

- **`auth whoami` calls an endpoint that production does not serve.** `GET /me` is absent from the
  OpenAPI document and 404s on prod, which the page now states along with the consequence: exit 4
  here is the endpoint missing, not the key being rejected (that is exit 2). `health` is the
  preflight that works.

- **The attachment-download pages claimed the signed URL is "single-use".** The OpenAPI
  description says short-lived (five minutes); nothing in the spec or the code says single-use.
  The claim is gone.

- **`meta.server_time`, `meta.missing_ids` and `meta.complete` are added by the CLI, not sent by
  the API.** The `MetaPagination` schema carries exactly four fields — `page`, `limit`, `total`,
  `has_more`. The pages that mention the other three now say who produces them.

- **`--page 0` is a CLI sentinel**, not a page. It means "send no page parameter"; pages start at
  1. The old text read `0 = server default`, which invited a caller to believe 0 was a page the
  server understood.

- **`capigo auth logout` emitted a spurious `-o json` nudge**, for the same reason `capigo help`
  did: it ignores `--output` and prints plain text, so the nudge was a false positive. Joined
  `version`, `config` and `help` in `hintExemptGroups`.

- **Twelve reference-data help pages said `--from-json` makes the field flags "ignored". It
  rejects them.** `create`, `update` and `replace` on all four groups exit 5 with "mutually
  exclusive" when both are passed — verified by running each. The only command in the CLI that
  genuinely ignores its field flags is `products create`, and the twelve pages had copied its
  sentence. Each page now describes its own command.

- **`products update` help claimed the wrong behaviour for `--from-json`.** It said "all
  individual field flags are ignored". They are not ignored — passing both `--from-json` and a
  field flag exits 5 with "mutually exclusive". `products create` genuinely does ignore them, and
  the update page had been describing the create page's behaviour. The two now say what each
  does, and each names the other's difference.

- **`variants get` help listed the variant shape but omitted three of its fields** —
  `manufacturer_code`, `legacy_code` and `extra_data`, all present in the response.

- **`capigo help …` emitted a spurious `-o json` nudge on stderr.** The `--help` *flag* never
  reached the hint (cobra short-circuits before `PersistentPreRunE`), but the `help` *command* is
  runnable and did, so asking for documentation produced a warning about parsing JSON. A hint
  that is wrong where it does not belong trains the reader to ignore it where it does. `help` now
  joins `version` and `config` in `hintExemptGroups`. Present since v0.16.0; surfaced by the new
  topics, which make `capigo help <topic>` a first-class path.

- **Help text described the API's HTTP response instead of the CLI's stdout.** Nineteen
  single-item commands (`get` / `create` / `update` / `replace` across `brands`, `categories`,
  `product-types`, `units`, plus `members get`, `products get`, `variants get`) documented their
  result as `Response: { "data": { … } }`. The CLI unwraps that envelope — every one of these
  commands calls `output.WriteJSONObject(os.Stdout, envelope.Data)` and prints the **bare
  object**. An agent that read the help and then reached for `.data.id` got `null`.
  The lines now read `Output (-o json): { … }`, naming the stream that the reader actually sees
  and dropping the wrapper. Doc-only; no behavior change.

  Note the bundled skill was already correct on this point (`single-item commands return the bare
  object`) — the drift was help-side, introduced by restating a cross-cutting contract inside
  nineteen separate command pages instead of stating it once.

## [0.20.1] — 2026-07-08

### Fixed

- **Skill (`capigo-api`): corrected the alias/barcode uniqueness claim.** Write-hygiene and the
  exit-8 row previously implied `sku`, `alias`, and `barcode` are all server-enforced unique. Only
  **`sku`** is — a duplicate fails with `E9445` (exit 8). The platform allows duplicate `alias`
  and `barcode` **by design** (no DB constraint, no error code; PCMS Story 5.4 AC3), so the server
  will not reject them and an agent must not expect an exit-8 conflict for them. The skill now
  says so: alias/barcode dedup is a client-side policy concern, not server-enforced. Doc-only.

## [0.20.0] — 2026-07-08

### Changed

- **Skill `capigo-api` restructured into a decision harness (docs only, no CLI behavior change).**
  Rewrote `SKILL.md` around a command→intent **capability map** (a complete, domain-grouped
  inventory the agent scans by what it wants to do) plus two explicit gates: **Gate 1** (a
  command you can't find is a *CLI gap to report*, never a reason to hand-roll raw HTTP — and you
  may not conclude "unsupported" until `capigo <group> --help` shows nothing), and **Gate 2**
  (self-diagnose on error before guessing). Motivation: a production agent transcript showed the
  agent guess a nonexistent `variants update`, wrongly conclude the API couldn't update variants,
  and fall back to raw `curl` — the map + gates close that path. Added a "Common wrong turns"
  section (variant writes go through `products variants`, no get-by-code, count via `meta.total`).
  Fixed two facts across the skill: `auth whoami` (GET `/me`) is not a reliable preflight (can
  404) — use `capigo health`; and `products update` is a single PUT write with no `products
  replace`. Rewrote `references/cli_basics.md` into a lean flag-level reference the map points
  into: removed OpenAPI as an agent-facing tool (the agent acts only through the CLI and never
  looks up endpoints), de-duplicated the exit-code table and self-diagnosis flow (now single-
  sourced in `SKILL.md`), reordered the command reference to match the map, and de-staled the
  pre-staged-commands notes.

### Fixed

- **`products variants --from-json` silently dropped unknown fields (data loss on write).**
  The command decoded `--from-json` into `[]api.UpsertVariantItem` and re-marshaled that
  struct as the request body — any field the struct didn't declare (e.g.
  `manufacturer_code`/`legacy_code`/`extra_data`) was stripped before the request left the
  machine, with no error or warning. Fixed by validating the input is a JSON array and then
  sending the raw bytes untouched (the same raw-passthrough pattern already used by
  `brands`/`categories`/`product-types`/`units` create/update/replace). `UpsertVariantItem`
  and `ProductVariant` also gained `manufacturer_code`, `legacy_code`, and `extra_data` fields
  so typed API consumers and JSON output (`variants get`, `products get`, `products variants`)
  see them too.

### Added

- **`tasks list` filter flags.** The backend (`query-parser.ts` `ALLOWED_FILTER_COLUMNS`)
  accepts filtering on 8 columns, but the CLI only exposed `--status`. Added `--priority`,
  `--assignee-id`, `--owner-id`, `--board-id`, `--board-list-id`, `--due-after`/`--due-before`,
  and `--created-after`/`--created-before` so every backend-supported filter is reachable
  without pulling the full list and filtering client-side.
- **Unknown-command hints, two layers.** Running a plausible-but-wrong (sub)command now
  points at the right one instead of silently doing nothing. **Layer 1:** group commands
  (`variants`, `products`, `brands`, …) previously weren't "runnable" in cobra's terms, so an
  unmatched subcommand like `variants update foo` fell through to `flag.ErrHelp` and printed
  the group's help with exit 0 — no error, no suggestion; cobra's built-in Levenshtein
  "Did you mean…?" suggestions never fired below the root. Every such group now gets a `RunE`
  that raises cobra's own `unknown command %q for %q` error (reusing cobra's real
  `SuggestionsFor`), so both the error and cobra's own suggestions now reach stdout via the
  existing self-diagnosing error block. **Layer 2:** a small curated registry
  (`cmd/unknown_command.go`) redirects the two evidence-backed conceptual misfires cobra's
  edit-distance can't catch — `variants update/create/replace` → "upsert through
  `products variants --product-id <id> --from-json -`", and `products delete/remove/destroy`
  → "archive via `products update <id> --from-json -` with `{"status":"ARCHIVED"}`" — via
  `ErrorDetail.Next` (exit code stays 5; no `CapabilityNote`, since this is a client-side
  redirect, not a server-side write failure). An anti-rot test asserts every registry entry's
  target command still exists in the live cobra command tree, so a rename/removal fails CI
  instead of silently pointing at a dead command.

---

## [0.19.0] — 2026-07-03

### Added

- **`tasks attachments download <task-id> <attachment-id>`** and **`tasks comments attachments download <task-id> <attachment-id>`** — download a task's own attachment, or an attachment posted on its comment/activity timeline, to a local file. Fetches a fresh, short-lived (5 minute) signed URL and downloads the bytes immediately; the URL is never printed or reusable across invocations. `--dest`/`-d` names a destination file or directory (default: original file name in the current directory). `tasks get`/`tasks list` table mode now show a `Files` attachment-count column (mirroring the existing `tasks comments` `Files` column), and `tasks get -o json` now includes the task's `attachments[]` array. The backing public API endpoints are live on prod as of 2026-07-03.

### Changed

- **Synced `api/openapi.json` to production via `make update-spec`.** Pulls PCMS drift the
  repo was behind on: `PublicProductVariantResponse` and the product-create / variants-upsert
  request bodies now document `extra_data`, `legacy_code`, and `manufacturer_code`; ref-data
  `{id}` endpoints document their `X-Server-Time` response headers and `422` responses;
  `/pcms/products` query params are aligned with prod. The attachment-download endpoints
  are now confirmed on prod, so their provisional "pre-staged" annotations are dropped.
  **`make update-spec` now pretty-prints** the fetched spec (prod serves minified JSON on
  one line) so the committed file stays reviewable and successive runs produce stable diffs.
  Spec-only sync; wiring the new variant fields into the CLI's output model / a
  `boards list --query` flag is tracked separately (issue #53).
- **Skill (`capigo-api`): `--query` is now taught as the first pass for finding products.**
  Removed the remaining steer toward `--all` for "alias/Product-Code checks" in
  `cli_basics.md` (which pushed the agent into full-catalogue scans + local filtering on
  4000+ product tenants) and reframed both `cli_basics.md` and `SKILL.md` to reach for
  `products list --query` first — it matches name, aliases, tags, SKU and barcode server-side.
  Documented the substring direction gotcha: the match is `stored ILIKE '%term%'`, so a
  shortened stored alias (`VVD013`) is **not** found by the full code (`SLM-DS-VVD013`) —
  search the short fragment or fall back to `--all`. Doc-only; no CLI behavior change.
  (Follow-up to v0.18.0, which removed the outright-wrong "`--query` does not index aliases"
  line but left the `--all`-first steer and lacked the substring caveat.)

---

## [0.18.0] — 2026-07-01

### Added

- **Product tags.** Products now carry a free-form `tags` string array alongside `aliases`
  (backend `feature/pcms-tags`, platform v1.25.x). `tags` is returned by `products get`/`list`
  and accepted on `products create`/`update` via a repeatable `--tags` flag or inside
  `--from-json` (`"tags": [...]`). The product table gains a **Tags** column (joined with
  `, `), mirroring Aliases, so tag values are visible where an agent reads. OpenAPI spec, the
  bundled `capigo-api` skill, and tests updated in lockstep.
- **`products create --aliases` flag.** Aliases were previously settable on create only via
  `--from-json`; a repeatable `--aliases` flag now matches `products update` and the new
  `--tags`, removing the create/update asymmetry.
- **Subtasks.** Two new endpoints from the mission API are now wrapped:
  - `tasks subtasks <parent-id>` — batch-create subtasks under an existing task
    (`POST /mission/tasks/{id}/subtasks`). One subtask via `--title` (+ `--description`,
    `--assignee`, `--due-date` `YYYY-MM-DD`, `--priority`, `--status`), or a batch via
    `--from-json -` (a JSON array of subtask items).
  - `tasks create --subtasks-json <file>` — create a parent task **and** its subtasks in one
    atomic call (`POST /mission/tasks/with-subtasks`); the parent is built from the existing
    create flags. Both are all-or-nothing (max 25 subtasks/request). These endpoints are
    **pre-staged**: they exist on `develop` but may ship ahead of production, so a call can
    return 404 until `develop` reaches prod (consistent with prior pre-staged CLI releases).
- **OpenAPI spec: three mission-task endpoints documented.** `api/openapi.json` now includes
  `GET /mission/tasks/{id}/comments` (already implemented by `tasks comments`, previously
  undocumented in the bundled spec), plus the two new subtask endpoints and their schemas
  (`SubtaskItem`, `CreateSubtasksRequest`, `CreateTaskWithSubtasksRequest`,
  `CreateTaskWithSubtasksTask`, `PublicTaskCommentResponse`), spliced from the platform spec.
  Path/body coverage guards updated in lockstep.

### Changed

- **`products list --query` help + skill now list all matched columns.** The search matches
  product name, aliases, tags, variant name, SKU, and barcode (per the merged `pcms_search_products`
  RPC). The CLI `-q` help text previously omitted aliases, and the skill wrongly claimed
  `--query` "does not index aliases" (aliases have been searchable since 2026-06-13) — both
  corrected, and the tombstone-only-matches-name/aliases caveat documented.

---

## [0.17.0] — 2026-06-29

### Added

- `boards list --query/-q` flag for case-insensitive board name search, matching the `q`
  parameter now served by `GET /mission/boards` (#TBD).

---

## [0.16.0] — 2026-06-18

### Added

- **Non-TTY `-o json` nudge.** When stdout is not a terminal (output was redirected or piped)
  and `--output` was not set, the CLI now prints a one-line reminder to **stderr** that table
  output is text, not JSON, and to re-run with `-o json` if it will be parsed. This is the only
  guard for the redirect-table-as-JSON mistake, which exits 0 (a valid table was printed) so
  the on-error diagnosis block never fires. The hint goes to stderr only — it never pollutes
  the captured stdout stream — is silenced by `CAPIGO_NO_HINTS=1`, and is suppressed for an
  explicit `--output`, on a real terminal, and for command groups that ignore `-o json`
  (`version`, `config`). Skill (`cli_basics.md`) updated in lockstep. Design note:
  `docs/tty-output-guard.md`. (Origin: Tấm redirected default table output into a `.json` file
  and hit JSONDecodeError; the v0.14.1 skill rule addressed it by instruction, this adds a
  tool-level backstop.)

---

## [0.15.0] — 2026-06-18

### Added

- **Self-diagnosis for auth errors.** The on-error diagnosis block now carries a `Next:` line
  for the public API's auth codes (`AUTH_INVALID_KEY`, `AUTH_INVALID_KEY_PREFIX`,
  `AUTH_MISSING_HEADER`, `AUTH_INVALID_FORMAT`, `AUTH_TENANT_MISMATCH`, `AUTH_INTERNAL_ERROR`),
  sourced from `apps/platform` `auth-guard.ts` / `proxy.ts`. The guidance distinguishes the
  **deterministic** failures (a rejected/malformed key or tenant mismatch — the CLI sends the
  same key every call, so retrying is futile) from the one genuinely **transient** case
  (`AUTH_INTERNAL_ERROR`, a 500 from the auth service, worth one retry). Phrasing directs the
  agent to **ask the user to re-authenticate**, never to source or echo a secret itself. The
  `capigo-api` skill's Authentication note is updated in lockstep. (Origin: Tấm read
  `AUTH_INVALID_KEY` as "token expired temporarily, just retry" — wrong on both counts.)

### Changed

- Corrected the stale `E0004` catalog entry (it is the internal web app's auth code, not a
  code this public-API CLI sees) and softened its `Next:` to the "ask the user" phrasing.

---

## [0.14.1] — 2026-06-18

### Changed

- **`capigo-api` skill — output-mode rules sharpened.** Converted the passive "use json to
  parse" guidance into an active prohibition stated at the point of action: *the moment you
  put `>` or `|` after a command, you must also pass `-o json`*. Added the `Server time:`
  placement invariant (stdout in table mode, stderr under `-o json`) as the direct antidote to
  a fabricated "strip the first line" (`lines[1:]`) workaround, plus a short reading-vs-
  processing decision guide in `references/cli_basics.md`. (Origin: Tấm redirected default
  *table* output into a `.json` file, hit `JSONDecodeError`, then invented a scene — a JSON
  body *with* a `Server time:` prefix, which cannot co-occur — and proposed stripping the
  first line, which would corrupt real `-o json` output.) No CLI behavior change.

---

## [0.14.0] — 2026-06-15

### Added

- **Self-diagnosing errors.** Every command failure now prints a diagnosis block on **stdout** (where an AI agent reads), not just a one-line `Error:` on stderr. The block resolves the error code to a meaning, a concrete next step, and the verbatim server response — so a caller no longer has to re-run with `--verbose` or recall the skill to understand what failed. JSON mode carries the same as `error.meaning` / `error.next` / `error.capability_note` / `error.raw`; quiet mode is unchanged. (Origin: Tấm hit `E9426` ("create product variant failed"), misread the opaque code as "the API does not support adding variants," and proposed a destructive recreate-and-archive workaround — when the operation is fully supported and the failure was about the request.)
- **Capability brake.** Server-side write failures (validation/conflict/business codes) print `Note: a failed write does NOT mean this operation is unsupported.` It is scoped to real server round-trips — never shown for client-side or cobra arg-validation errors.
- **A hand-maintained error catalog** (`internal/api/error_catalog.go`) for the common PCMS codes, sourced from the backend `error-codes.ts`. Each entry supplies the next step and the capability brake; it adds an explanatory `Meaning` **only** for codes whose server message is a generic fallback (E9426/E9427), letting the server message remain the single source for the rest (no drift-prone second copy). To be replaced by an OpenAPI-published catalog once available.

### Fixed

- **`--verbose` now works.** The flag was declared but never wired, so the skill's "re-run with `--verbose`" advice was a no-op. It now traces the redacted HTTP request and response.
- **Corrected stale error-code comments** in `internal/api/errors.go` (`E9445` is SKU-exists, `E9446` is duplicate-option-combination — not "currency" / "options pairing").

### Changed

- `capigo-api` skill updated in lockstep: the self-diagnosis section now leads with "a failed write is not a missing capability," points at the on-stdout diagnosis block, and warns against destructive workarounds.

---

## [0.13.0] — 2026-06-11

### Changed

- **`products list --all` no longer discards everything on a mid-pagination failure.** Previously a rate-limit or network blip at page N exited with empty stdout, so a partial catalogue read looked like an empty one (and an agent could conclude "0 products — the alias is free"). Now the rows already fetched are still rendered, the table footer reads `… · INCOMPLETE — aborted at page N — results are PARTIAL`, JSON meta carries `"complete": false` (with the server's real `total` and `has_more: true`), and the command still exits with the underlying error's code.
- **`products list --ids` now reports requested IDs the server did not return.** Asking for 5 UUIDs and getting 3 rows was a clean exit 0 with `Total: 3 (all rows shown)` — the two missing products simply vanished. Table mode now prints `Requested 5 ids · 3 found · missing: <id>, <id>`; JSON meta carries `missing_ids`.
- **`Server time:` (the delta-sync cursor) moved to stdout in table mode** — the same stderr-salience trap as the v0.11.0 pagination fix. json/quiet modes keep it on stderr so stdout stays machine-parseable, and products list JSON now also carries it as `meta.server_time`. Applied via a shared `emitServerTime` helper across all 20 emit sites.
- `capigo-api` skill updated to match: `--all` partial semantics ("check `complete` / the footer"), the `--ids` missing-IDs rule, and the new server-time placement.

---

## [0.12.0] — 2026-06-11

### Changed

- **Soft-deleted products are now visibly dead in table mode.** The Status cell carries a tombstone — `ACTIVE (DELETED)` — whenever `is_deleted` is true. Previously the table dropped `is_deleted` entirely and a soft-deleted product rendered exactly like a live one, so an agent could report deleted stock as available or write to a tombstoned record. JSON already carried `is_deleted` and is unchanged.
- **Product tables now show an `Aliases` column** (alias codes joined with `, `). Aliases were previously JSON-only, which silently broke alias-uniqueness checks done from table output — the column the dedup workflow depends on simply didn't exist.
- **Every tenant-scoped command now echoes the tenant it actually used on stdout.** List footers gain a `Tenant: acme · …` prefix and successful table-mode writes print a `Tenant: acme` line after the result. When the tenant was resolved implicitly, the echo names the source — `Tenant: acme (from CAPIGO_TENANT)` or `(from config default_tenant)` — so a silently defaulted tenant can no longer route a read or write to the wrong tenant without it being visible. New `output.WriteTenantLine` helper; `ListSummary` gains `Tenant`/`TenantNote`.
- `capigo-api` skill updated to match all three: the tenant-echo rule ("read that line; wrong tenant → stop"), the `(DELETED)` / `is_deleted` caveat on products, and the `Aliases` column note.

---

## [0.11.0] — 2026-06-11

### Changed

- Every `list` command now prints a pagination **summary footer to stdout** in table mode — e.g. `Total: 43 · showing 20 (page 1/3) · more rows — use --page/--limit (max 100)`, or `Total: 12 (all rows shown)` when the page is complete. Previously the only signal that more rows existed was a `Showing N of M` nudge on **stderr**, which an agent reading stdout never saw: it would render a 20-row page of a 43-row collection and report "20". Surfacing the total on the same stream the agent reads — on every list, not just when more pages exist — removes that trap. JSON mode is unchanged (`meta.total`/`meta.has_more` already carried this). New `output.WriteListSummary` helper centralises the footer; `products list` advertises `--all` in its hint and prints `Total: N (all rows shown)` after `--all` fetches the full set.
- `capigo-api` skill updated to match: a new **"Counting how many X are there?"** recipe in `cli_basics.md` (read `meta.total`, never count visible rows), the pagination note rewritten around the stdout footer, and a counting reminder added to the `SKILL.md` fundamentals.

---

## [0.10.1] — 2026-06-11

### Fixed

- `capigo-api` skill now documents pagination explicitly. Previously it mentioned `--page`/`--limit`/`--all` only in passing, so an agent could run a `list`, see the first 20 rows, and treat them as the complete set — silently breaking uniqueness/existence checks. Added a **Pagination** section to `cli_basics.md` (the `meta.has_more`/`total` signal, `--all` is `products`-only, and the key caveat that JSON mode gives no stderr "Showing N of M" nudge so the agent must check `meta` itself), plus a fundamentals bullet in `SKILL.md` and a pagination caveat on the collision-check write-hygiene rule.

---

## [0.10.0] — 2026-06-11

### Changed

- The bundled `capigo-api` skill is now scoped to **CLI mechanics only** — auth, tenant handling, exit codes, output modes, and the full command surface (`SKILL.md` + `references/cli_basics.md`). The organisation-specific catalogue policy it previously carried (Product Code / Barcode generation rules, brand decision trees, `coding_references` governance, and the manage_product / manage_brand / manage_product_type / sync_check workflows) moved to the internal `manage-capigo-product` skill in `vtech-com/agent-skills`, which layers on top of this skill. Rationale: the SDK skill is public and partner-facing; how a given organisation codes its catalogue is policy, not CLI usage.
- Skill install is now solely via the [`skills`](https://github.com/vercel-labs/skills) CLI: `npx skills add vtech-com/capigo-api-sdk --skill capigo-api` pulls the skill straight from this repo into any supported agent (Claude Code, Cursor, Codex, OpenCode, …) with no extra infrastructure. Dropped the release-asset `curl`/`unzip` fallback and the release-workflow step that attached `capigo-api-skill.zip` to each release — the `skills` CLI reads the repo directly, so no published artifact is needed. `make skill-package` / `skill-install-tam` remain for the internal Tấm host and manual copies.

---

## [0.9.0] — 2026-06-10

### Added

- Bundled agent skill `skills/capigo-api/` — a self-contained `SKILL.md` + `references/` guide that teaches an AI agent how to operate the `capigo` CLI (auth, tenant handling, exit codes, output modes, PCMS catalogue workflows incl. Product Code / Barcode generation). The skill ships alongside the CLI it documents so anyone with repo access — including partners using the public API with their own token — gets the intended operating instructions. New `make skill-package` target zips it to `dist/capigo-api-skill.zip` for distribution to skill-aware runtimes (e.g. openclaw). README **AI Agent Integration** section documents usage.
- Skill install paths: `make skill-install-tam` packages and installs the skill onto the Tấm openclaw host over SSH (idempotent — clears the old copy first; `TAM_HOST`/`TAM_SKILLS_DIR` overridable). The release workflow now attaches `capigo-api-skill.zip` to each GitHub release (`gh release upload --clobber`), giving partners and Tấm a stable per-version download URL without cloning the repo.

---

## [0.8.0] — 2026-06-08

### Added

- `tasks comments <id>` — `GET /mission/tasks/{id}/comments`: list a task's conversation + activity timeline (human comments interleaved with system activity such as status/assignment/title/description/due-date/create events). Each entry carries a `kind` (`comment` for a person/agent message, `activity` for a system event), a resolved `author` (`{id, name, type}`; name never exposes email), flat `attachments` metadata, raw structured `ui_data`, `parent_id`, and `created_at`. Flags: `--type comment|activity` (default both), `--sort asc|desc` (default `desc`, newest first), `--page`, `--limit` (max 50), optional `--tenant`. A task with no comments yet returns an empty list (exit 0), not an error — the authoritative current status lives on the task itself (`tasks get`); this command provides the history/narrative. **UUID-addressed only.** Spec pre-staged from the monorepo `develop` branch ahead of prod deployment; the endpoint returns 404 until deployed, and will reconcile on `make update-spec` after deploy (at which point it joins the OpenAPI param-coverage guard).

---

## [0.7.0] — 2026-06-04

### Added

- `products get <id>` — `GET /pcms/products/{id}`: fetch a single product by UUID with full detail (variants, options, brand, category, product type, unit). Tenant required. Returns same shape as `products list` items. **UUID-addressed only.** Spec pre-staged from the monorepo `develop` branch ahead of prod deployment; will reconcile automatically on `make update-spec` after deploy.
- `tasks update <id>` — `PATCH /mission/tasks/{id}`: partial update of a task. Flags: `--title`, `--description` (empty string clears), `--status`, `--assignee` (UUID; `--assignee ""` sends `assignee_id: null` to unassign), `--board` + `--list` (sent together — pass `--board "" --list ""` to send both as `null` and remove the task from its board), `--follower-id` (repeatable; additive — removes not supported). At least one flag required. Optional `--tenant`. Returns updated task. The PATCH body is built as a tri-state map so absent fields are omitted and cleared fields serialize as JSON `null` (the API rejects empty-string UUIDs). **UUID-addressed only.** Spec pre-staged from `develop`; not yet on prod.
- `members get <id>` — `GET /members/{id}`: fetch a single workspace member by UUID. Optional `--tenant`; if omitted resolves across all accessible tenants. Returns 404 for inaccessible or cross-tenant members. **UUID-addressed only.** Spec pre-staged from `develop`; not yet on prod.
- `variants get <id>` — `GET /pcms/variants/{id}`: fetch a single product variant by UUID with full variant detail (sku, barcode, price, compare_at_price, currency, weight, dimensions, option1/2/3, variant_type, timestamps). Tenant required. Orphaned, soft-deleted, or cross-tenant variants return 404. Decodes into `api.ProductVariant` so `--output json` emits the complete `PublicProductVariantResponse`; table/quiet mode shows id, name, sku, barcode, price, and type. **UUID-addressed only.** Spec pre-staged from `develop`; not yet on prod.
- `health`: new top-level command calling `GET /health` as a preflight. Exit 0 confirms the API is reachable and the API key is accepted; a non-zero exit (e.g. 2 for auth) tells an automated caller why it failed before running real work. Not tenant-scoped. JSON mode emits `{"ok":bool,"timestamp":string}`.
- `docs/api-coverage-gaps.md`: living backlog of Public API endpoints/actions the CLI can't yet wrap because the server-side endpoint doesn't exist (e.g. `GET /pcms/products/{id}`, `PATCH /mission/tasks/{id}`), assessed against the monorepo `develop` route handlers.

---

## [0.6.0] — 2026-06-04

### Added

- `members list`: list workspace members via `GET /members` with `--query`/`-q` (name/email search), `--page`, `--limit`, and optional `--tenant`. Emits the standard `{"data":[...],"meta":{...}}` JSON contract. The `api.Member` model and `member` table renderer already existed; this wires up the command. Registered in the openapi path/query coverage guards (`/members` moved from `unimplementedPaths` to `implementedPaths`).

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
