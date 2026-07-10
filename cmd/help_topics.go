// Package cmd — help_topics.go
//
// Cross-cutting help topics. Each is a cobra.Command with no Run: cobra never
// executes it, it only prints the Long text, and lists it under "Additional
// help topics" in the root help.
//
// These pages exist so that a fact true of many commands is stated in exactly
// one place. Command help pages reference a topic instead of restating it.
// Restating a cross-cutting contract inside every command page is how the
// `Response: {"data": …}` drift reached nineteen pages before it was caught.
//
// House rules for the text below:
//
//   - Describe what the CLI does. Rules about how a caller should behave
//     ("never call the API directly", "confirm before writing", "stop and ask")
//     belong to the caller's own policy, not to a public SDK's help.
//   - Every literal block declares what it is: `$` prefixes a command you type,
//     an un-prefixed block is what the CLI prints, and a schema says so.
//   - There is one output shape. A sentence that begins "in table mode" or "in
//     json mode" is describing a CLI that no longer exists.
package cmd

import "github.com/spf13/cobra"

var tenancyTopicCmd = &cobra.Command{
	Use:   "tenancy",
	Short: "How --tenant resolves, and which commands require it",
	Long: `How the tenant is chosen for a command, and which commands require one.

RESOLUTION ORDER
  --tenant <code>        the flag, when given
  CAPIGO_TENANT          the environment variable
  default_tenant         the active profile in ~/.capigo/config.json

  The first one set wins. --tenant is a per-command flag, not a global one.

  $ capigo config set-default-tenant acme
  $ capigo config unset-default-tenant

WHICH COMMANDS REQUIRE A TENANT
  Always      products, variants, brands, categories, product-types, units
              — every verb. With no tenant resolved the command exits 5.
  Always      tasks create, tasks subtasks create
  Optional    tasks list/get, boards list/get, members list/get.
              Omitting --tenant reads across every tenant the key can reach.
              meta then names no tenant, because there was no single one to
              name — and the records do not name theirs either. A cross-tenant
              read tells you what exists, not where. Pass --tenant when you
              need to know which tenant a record lives in.
  Never       tenants, auth, health, config, version

SEEING WHICH TENANT WAS USED
  Every response names the tenant it resolved to, and where that came from:

      "meta": { "tenant": "acme", "tenant_source": "flag" }
      "meta": { "tenant": "acme", "tenant_source": "env" }
      "meta": { "tenant": "acme", "tenant_source": "config" }

  Read it after a write. A record created in the wrong tenant is not an error
  the server can raise — the request was valid, and it succeeded.

  $ capigo tenants list        # which tenants this key can reach

SEE ALSO
  capigo help output       the envelope, and which stream carries what
  capigo help exit-codes   what a non-zero exit means`,
}

var outputTopicCmd = &cobra.Command{
	Use:   "output",
	Short: "The JSON envelope, and which stream carries what",
	Long: `The one shape every command emits, and which stream carries what.

THE ENVELOPE
  Every command that succeeds prints exactly this to stdout:

      { "data": …, "meta": { … } }

  data is an array for a list and an object for a single item — a get, or a
  successful create, update or replace. The key is always there, and always
  called data, so a caller never has to know which kind of command it ran.

      $ capigo brands list   | jq '.data[].id'    # list   → array
      $ capigo brands create | jq '.data.id'      # single → object

  There is no output flag. stdout is JSON, always, and nothing else is ever
  written to it.

META
  meta names the tenant the call resolved to, and where that value came from:

      tenant          the tenant code the request was scoped to
      tenant_source   flag, env (CAPIGO_TENANT), or config (default_tenant)

  Both are absent on a command that takes no tenant, and on a cross-tenant
  read that resolved none. How the tenant is chosen: capigo help tenancy

  A list also carries the API's pagination:

      page       current page
      limit      page size
      total      full count across all pages, independent of page size
      has_more   true when further pages remain

  total is the count across every page, not the number of rows in data. Read
  it; do not count data[].

  One further field is added by the CLI, not sent by the API:

      server_time    the server clock at the time of the call, for delta sync

  An endpoint may add meta fields of its own — boards get sends list_count.
  They are documented on that command's page.

FAILURE
  A command that fails prints an object with an error key — still JSON, still
  on stdout, so a caller that parses stdout unconditionally reads a diagnosis
  rather than a parse error:

      { "error": { "code": …, "message": …, "next": …,
                  "request_id": … } }

  A one-line summary also goes to stderr.

A PARTIAL RESULT
  A command may fail after fetching real rows: an --all sweep that aborts on
  page 41 still holds forty pages, and a --ids request that lost two ids still
  found the rest. Those rows are not discarded. The document carries all three
  keys, error first:

      { "error": { … }, "data": [ … ], "meta": { … } }

  The error key is the test. A document that has one is not a complete answer,
  whatever the rows and the meta beside it may look like — a --ids result
  missing two ids reports "total": 1 and "has_more": false, which is exactly
  what success looks like. Check for the key, and you never have to reach for
  the exit code or read stderr to know what you are holding.

  stdout carries ONE JSON document, always. Nothing is ever appended to a
  document already printed.

  What the exit code means, and what each field of the error object carries:
  capigo help exit-codes

STREAMS
  stdout    one JSON document, and nothing else. It carries data, or an error,
            or — when a command failed after fetching rows — both.
  stderr    the one-line error summary, and --verbose HTTP tracing.

  Everything a caller must act on is on stdout: which tenant a write landed in,
  how many records exist, whether the answer is complete. Nothing that matters
  is announced only on a stream the caller may not read, or only in an exit code
  it may not check.

SEE ALSO
  capigo help exit-codes   what a non-zero exit means
  capigo help tenancy      how --tenant resolves`,
}

var exitCodesTopicCmd = &cobra.Command{
	Use:   "exit-codes",
	Short: "What each exit code means",
	Long: `Every command exits with one of these codes. The codes are stable; the wording
of an error message is not.

  0   Success.
  1   General or unexpected failure — a malformed invocation, an unknown
      command, or an error the CLI could not classify.
  2   Authentication failed (HTTP 401). The API key is missing, malformed, or
      rejected. A new key is stored with:
          $ capigo auth login --key csk_...
  3   Permission denied (HTTP 403). The key is valid but not allowed to reach
      that tenant or resource.
  4   Not found (HTTP 404). No such id, or it is outside the resolved tenant.
  5   Validation error (HTTP 400 or 422). The request was rejected: a missing
      required flag, an out-of-range value, or no tenant resolved for a command
      that requires one.
  6   Network error. The API could not be reached, or the connection failed
      before a response arrived.
  7   Rate limited (HTTP 429).
  8   Conflict (HTTP 409). A server-enforced unique value already exists — a
      duplicate variant sku, for example. Note that alias and barcode are not
      server-enforced unique, so duplicates of those do not produce this code.

WHEN A COMMAND FAILS
  stdout carries a JSON object with an error key; stderr carries a one-line
  summary. The error object carries:

      code             the stable error code
      message          the API's own error message
      meaning          an added interpretation, when the message is generic
      next             a concrete step
      capability_note  present on a write failure. It says: a failed write does
                       NOT mean this operation is unsupported
      raw              the verbatim server body
      request_id       identifies the call in the API's server logs
      http_status      the response status, when the server responded

  --verbose prints the full HTTP request and response to stderr, with the
  Authorization header redacted.

SEE ALSO
  capigo help output   the envelope, and which stream carries what`,
}

var softDeleteTopicCmd = &cobra.Command{
	Use:   "soft-delete",
	Short: "How deleted records appear in reads",
	Long: `Capigo deletes records softly: a deleted row is retained and marked, not
removed. What the CLI exposes about that state differs by resource.

PRODUCTS
  A soft-deleted product still appears in products list and products get. The
  product object carries "is_deleted": true.

  The "status" field alone does NOT reveal deletion — a deleted product may
  still read "status": "ACTIVE". Check is_deleted, not status.

  Searching for tombstones is narrower than searching live products: for a
  soft-deleted product, --query matches its name and aliases only. Variant
  fields — variant name, sku, barcode — are not indexed for deleted products.
  A full sweep therefore needs products list --all or --updated-since rather
  than --query.

VARIANTS
  A soft-deleted variant is not returned and marked — it is simply absent.
  variants get on one exits 4, the same as a variant that never existed.

EVERY OTHER RESOURCE
  brands, categories, product-types, units, members, tasks and boards do not
  expose a deletion flag through this CLI. Their responses carry no is_deleted
  field; there is no table output.

SEE ALSO
  capigo help output   the envelope, and which stream carries what`,
}

var versioningTopicCmd = &cobra.Command{
	Use:   "versioning",
	Short: "How this CLI relates to the API it calls",
	Long: `How this CLI relates to the API it calls.

THIS BUILD
  Help text ships inside the binary, so it describes this build and no other.

      $ capigo version     # version, commit, and build date

THE API
  This CLI is a client for the Capigo Public API. The API's specification is
  published at https://platform.capigo.app/api/openapi

  The CLI does not expose every endpoint that specification describes. A
  capability present in the specification and absent from this CLI is a gap in
  the CLI, not in the platform.

ADDITIVE CHANGES
  The API is additive: released capabilities are not removed or redefined. A
  capability absent from this binary may exist in a newer release, and help
  text from an older binary describes the older surface.

SEE ALSO
  capigo help exit-codes   what a non-zero exit means
  capigo help output       the envelope, and which stream carries what`,
}

func init() {
	rootCmd.AddCommand(
		tenancyTopicCmd,
		outputTopicCmd,
		exitCodesTopicCmd,
		softDeleteTopicCmd,
		versioningTopicCmd,
	)
}
