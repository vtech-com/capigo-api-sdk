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
//   - State the output mode a behaviour depends on. Several of these lines are
//     table-mode only.
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
  Always      tasks create, tasks subtasks
  Optional    tasks list/get, boards list/get, members list/get.
              Omitting --tenant resolves across every tenant the key can
              reach, and table output gains a Tenant column.
  Never       tenants, auth, health, config, version

SEEING WHICH TENANT WAS USED   (table mode only)
  A list footer and a successful write both name the tenant:

      Tenant: acme
      Tenant: acme (from CAPIGO_TENANT)
      Tenant: acme (from config default_tenant)

  The parenthetical appears only when --tenant was not given, and names where
  the value came from. In json and quiet mode neither line is printed.

  $ capigo tenants list        # which tenants this key can reach

SEE ALSO
  capigo help output       output modes and the JSON contract
  capigo help exit-codes   what a non-zero exit means`,
}

var outputTopicCmd = &cobra.Command{
	Use:   "output",
	Short: "Output modes, the JSON contract, and which stream carries what",
	Long: `Output modes, the JSON contract, and which stream carries what.

MODES
  -o table   (default) human-readable rows and summary lines
  -o json    machine-readable; the form to parse, store, or pipe
  -o quiet   resource ids only, one per line

  Any other value is rejected with an error.

THE JSON CONTRACT
  List commands emit an envelope. Shape:

      { "data": [ … ], "meta": { … } }

  The rows are at .data[] — not at the top level.

  Single-item commands — get, create, update, replace, and products variants —
  emit the BARE object on stdout. The CLI unwraps the API's {"data": …}
  envelope; there is no .data to reach for.

      $ capigo brands list   -o json | jq '.data[].id'    # list   → envelope
      $ capigo brands create -o json | jq '.id'           # single → bare

  meta always carries:

      page       current page
      limit      page size
      total      full count across all pages, independent of page size
      has_more   true when further pages remain

  A command may add fields to meta. Those are documented on that command's own
  help page (for example meta.complete under products list --all).

LIST FOOTERS   (table mode only)
  After the rows, a list prints one summary line on stdout:

      Tenant: acme · Total: 137 · showing 20 (page 1/7) · more rows — use --page/--limit (max 100)
      Tenant: acme · Total: 12 (all rows shown)

  Total is the count across all pages, not the number of rows displayed. A
  command may append its own hint to that line; its help page shows the exact
  form. In json mode there is no footer — the same numbers are in meta.

TENANT ECHO   (table mode only)
  After a successful write, the CLI prints the tenant it resolved:

      Tenant: acme
      Tenant: acme (from CAPIGO_TENANT)

  In json and quiet mode nothing is printed.
  How the tenant is resolved: capigo help tenancy

STREAMS
  stdout    results · list footers · the tenant echo · and, when a command
            fails, the diagnosis block (Server / Means / Next / Response,
            including request_id)
  stderr    the server timestamp in json and quiet mode · the one-line error
            summary · advisory hints

  Commands that support delta sync report the server timestamp. It prints on
  stdout in table mode and on stderr in json and quiet mode, and is carried as
  meta.server_time in JSON list output. A -o json stream is therefore pure
  JSON: there is no prefix line to strip.

  When stdout is not a terminal and --output was not given, the CLI writes a
  one-line reminder to stderr that table output is text, not JSON. It never
  writes that reminder to stdout. CAPIGO_NO_HINTS=1 silences it.

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
  The CLI prints a diagnosis block on stdout and a one-line summary on stderr.
  The block carries:

      Server:     the API's own error message
      Means:      an added interpretation, when the server message is generic
      Note:       shown on write failures
      Next:       a concrete step
      Response:   the verbatim server body, including request_id

  request_id identifies the call in the API's server logs.

  -o json adds error.meaning, error.next, error.capability_note and error.raw.
  -o quiet prints only the stderr summary.

  --verbose re-runs the call and prints the full HTTP request and response,
  with the Authorization header redacted.

SEE ALSO
  capigo help output   which stream carries what`,
}

var softDeleteTopicCmd = &cobra.Command{
	Use:   "soft-delete",
	Short: "How deleted records appear in reads",
	Long: `Capigo deletes records softly: a deleted row is retained and marked, not
removed. What the CLI exposes about that state differs by resource.

PRODUCTS
  A soft-deleted product still appears in products list and products get.

      -o json     the product object carries "is_deleted": true.
                  The "status" field alone does NOT reveal deletion — a deleted
                  product may still read "status": "ACTIVE".
      table       the Status cell carries a suffix:

                      ACTIVE (DELETED)

  Searching for tombstones is narrower than searching live products: for a
  soft-deleted product, --query matches its name and aliases only. Variant
  fields — variant name, sku, barcode — are not indexed for deleted products.
  A full sweep therefore needs products list --all or --updated-since rather
  than --query.

EVERY OTHER RESOURCE
  variants, brands, categories, product-types, units, members, tasks and boards
  do not expose a deletion flag through this CLI. Their responses carry no
  is_deleted field and their table output carries no deletion marker.

SEE ALSO
  capigo help output   the JSON contract and table footers`,
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
  capigo help output       output modes and the JSON contract`,
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
