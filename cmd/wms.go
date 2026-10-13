package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

var wmsCmd = &cobra.Command{
	Use:   "wms",
	Short: "Warehouses, locations and stock documents (WMS)",
	Long: `The Capigo Warehouse Management System (WMS): warehouses, their locations, and
the documents that plan stock in, out and between them.

Every record here is addressed by its code, not by a UUID, and every command
requires a tenant. Everything but a warehouse itself also lives inside one
warehouse, so those commands take --warehouse <code> as well: a document's code
is unique within its warehouse, not within the tenant.

  $ capigo wms warehouses list --tenant acme       # the warehouse codes

The four documents are written the same way. A create body can be sent to
preview or validate first, which run the create's own checks and write nothing.
A created document is a DRAFT; update edits it, submit hands it on, and cancel
ends it. update, submit and cancel take --if-match, the document's current
updated_at as get prints it, and exit 8 with code E0602 when someone else has
changed the document since — read it again and retry with the new value.

These commands plan; they do not post stock. Receiving, picking, exporting,
moving and approving are done in the Capigo app, and no command here performs
them. A document's status tells you how far the warehouse has taken it.

USAGE
  capigo wms <group> <command> --tenant <code> [--warehouse <code>] [<args>]`,
}

// wmsFlags is the flag state of one wms command. Every command built here owns
// its own, so two commands never share a variable by accident.
type wmsFlags struct {
	tenant     string
	warehouse  string
	query      string
	statuses   []string
	isActive   string
	sort       string
	page       int
	limit      int
	costMethod string
	fromJSON   string
	ifMatch    string
	reason     string
	idemKey    string
	lines      []string
	warehouses []string
}

// wmsSession builds the client and resolves the tenant every wms command needs.
func wmsSession(tenantFlag, subject string) (*api.Client, *string, error) {
	client, cfg, err := buildClient()
	if err != nil {
		return nil, nil, err
	}
	tenant := resolveTenant(tenantFlag, activeProfileOrEmpty(cfg))
	requireTenant(tenant, subject)
	return client, tenant, nil
}

// requireWarehouse exits 5 when a warehouse-scoped command was given no
// warehouse. The server would answer WAREHOUSE_CODE_REQUIRED, which is true and
// does not say where a code comes from.
func requireWarehouse(warehouse, subject string) string {
	code := strings.TrimSpace(warehouse)
	if code == "" {
		failValidation("%s: --warehouse <code> is required; capigo wms warehouses list prints the codes", subject)
	}
	return code
}

func requireWMSCode(code, subject string) {
	if strings.TrimSpace(code) == "" {
		failValidation("%s: the code must not be blank", subject)
	}
}

func validateWMSLimit(limit int) {
	if limit > api.MaxWMSLimit {
		failValidation("--limit must be at most %d for a wms list (got %d)", api.MaxWMSLimit, limit)
	}
}

// requireIfMatch returns the concurrency token to send, refusing a call that
// has none: the API answers a missing one with a 400 that does not say where
// the value comes from.
func requireIfMatch(value, subject, getCommand string) string {
	token := strings.TrimSpace(value)
	if token == "" {
		failValidation("%s: --if-match is required; pass the document's current updated_at, as %s prints it", subject, getCommand)
	}
	return token
}

// readWMSBody reads the request body a document write is built from. The body
// holds an array of lines, which no set of flags describes, so the file is the
// only way in.
func readWMSBody(path, subject string) (json.RawMessage, error) {
	if path == "" {
		failValidation("%s: --from-json <path|-> is required; the body is described under FLAGS in --help", subject)
	}
	raw, err := readJSONInput(path)
	if err != nil {
		return nil, fmt.Errorf("read --from-json: %w", err)
	}
	if !json.Valid(raw) {
		failValidation("%s: --from-json does not hold valid JSON", subject)
	}
	return json.RawMessage(raw), nil
}

func writeWMSList(resp *api.Response, tenant *string, tenantFlag string) error {
	var envelope api.RawEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		return handleErr(fmt.Errorf("decode response: %w", err))
	}
	return output.Write(os.Stdout, rawList(envelope.Data), listMeta(tenant, tenantFlag, envelope.Meta))
}

func writeWMSItem(resp *api.Response, tenant *string, tenantFlag string) error {
	data, meta := api.SplitAnswer(resp.Body)
	return output.Write(os.Stdout, rawItem(data), itemMeta(tenant, tenantFlag, meta))
}

// writeWMSWrite prints the answer to a write. replayed marks a create the
// server did not perform again, which only the status code says.
func writeWMSWrite(resp *api.Response, tenant *string, tenantFlag string, replayed bool) error {
	data, apiMeta := api.SplitAnswer(resp.Body)
	meta := itemMeta(tenant, tenantFlag, apiMeta)
	meta.ServerTime = resp.ServerTime
	if replayed {
		if meta.Extra == nil {
			meta.Extra = map[string]any{}
		}
		meta.Extra["replayed"] = true
	}
	return output.Write(os.Stdout, rawItem(data), meta)
}

const wmsTenantFlagHelp = `  --tenant <code>
      Tenant the warehouse belongs to. Required; a default tenant counts. See
      capigo help tenancy.`

const wmsWarehouseFlagHelp = `  --warehouse <code>
      Warehouse to act in, by its code as wms warehouses list prints it.
      Required. A warehouse that does not exist in the tenant exits 4; one
      this key holds no scope in exits 3.`

const wmsPagingFlagHelp = `  --page <n>
      Page to fetch, from 1. The default, 0, lets the server choose.

  --limit <n>
      Rows per page, 1 to 50. The default, 0, lets the server apply its own
      default of 20. Over 50 exits 5 before any call.`

// fill replaces {name} placeholders in a help page. Pairs are name, value.
func fill(text string, pairs ...string) string {
	args := make([]string, 0, len(pairs))
	for i := 0; i+1 < len(pairs); i += 2 {
		args = append(args, "{"+pairs[i]+"}", pairs[i+1])
	}
	// Twice, so a value may itself hold a placeholder.
	r := strings.NewReplacer(args...)
	return r.Replace(r.Replace(text))
}

// --------------------------------------------------------------------------
// wms warehouses
// --------------------------------------------------------------------------

var wmsWarehousesCmd = &cobra.Command{
	Use:   "warehouses",
	Short: "List and read warehouses",
	Long: `Warehouses: the places stock is kept, and the scope every other wms command
acts in.

A warehouse's code is what --warehouse takes everywhere else under wms. Only the
warehouses this key's member may see are listed: an owner or a WMS admin sees
all of them, anyone else only those they hold a scope in.

USAGE
  capigo wms warehouses <command> --tenant <code> [<args>]`,
}

var wmsWarehousesList wmsFlags

var wmsWarehousesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the warehouses of a tenant",
	Long: `List the warehouses this key can see in a tenant.

PURPOSE
  Find a warehouse's code: every other wms command acts inside one warehouse
  and takes that code as --warehouse. For one warehouse with its tree of
  locations, use wms warehouses get.

USAGE
  capigo wms warehouses list --tenant <code> [-q <term>] [--is-active <true|false>]
          [--sort <column>:<asc|desc>] [--page <n>] [--limit <n>]

FLAGS
  --tenant <code>
      Tenant to read. Required; a default tenant counts. See capigo help
      tenancy.

        capigo wms warehouses list --tenant acme

  -q, --query <term>
      Search on code and name. Max 200 characters.

  --is-active <true|false>
      Only active, or only inactive, warehouses. Left out, both are listed.

  --sort <column>:<asc|desc>
      Order by created_at, updated_at, name or code. The default is
      created_at:desc.

        capigo wms warehouses list --tenant acme --sort code:asc

` + wmsPagingFlagHelp + `

OUTPUT
  The warehouses are at .data[]; read meta.total for the full count:

      { "data": [
          { "id": "79a08e42-9817-4965-8ac5-97a41fd9d787", "code": "WH-HN",
            "name": "Kho Ha Noi", "type": "user",
            "address": { "city": "Ha Noi", "state_code": "01",
                         "postal_code": "100000", "country_code": "VN",
                         "address_line_1": "116 Le Duan" },
            "currency": "VND", "location_strategy": "ZONE_BIN",
            "is_active": true, "settings": {}, "extra_data": null,
            "created_at": "2026-09-18T07:43:06.269333+00:00",
            "updated_at": "2026-09-18T07:43:06.269333+00:00" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false } }

  An empty .data[] does not mean the tenant has no warehouses: it lists only
  the ones this key's member holds a scope in.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		f := &wmsWarehousesList
		validateWMSLimit(f.limit)
		isActive := wmsIsActive(f.isActive, "wms warehouses list")

		client, tenant, err := wmsSession(f.tenant, "wms warehouses list")
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.ListWMS(context.Background(), api.WMSWarehouses, api.WMSListQuery{
			Query: f.query, IsActive: isActive, Sort: f.sort, Page: f.page, Limit: f.limit,
		}, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSList(resp, tenant, f.tenant)
	},
}

var wmsWarehousesGet wmsFlags

var wmsWarehousesGetCmd = &cobra.Command{
	Use:   "get <code>",
	Short: "Get a warehouse and its tree of locations",
	Long: `Get one warehouse by its code, with its locations as a tree.

PURPOSE
  Read a warehouse's settings and see how its locations nest: zones, and the
  bins inside them. For a flat, searchable list of the same locations, use wms
  locations list.

USAGE
  capigo wms warehouses get <code> --tenant <code>

FLAGS
  <code>
      Warehouse code, as wms warehouses list prints it. Positional, required.

  --tenant <code>
      Tenant the warehouse belongs to. Required; a default tenant counts.

        capigo wms warehouses get WH-HN --tenant acme

OUTPUT
  The warehouse is at .data. location_hierarchy[] holds the top-level
  locations, each with its own children[] nested to any depth; location_count
  is the number of locations in the whole tree:

      { "data": {
          "id": "79a08e42-9817-4965-8ac5-97a41fd9d787", "code": "WH-HN",
          "name": "Kho Ha Noi", "type": "user",
          "address": { "city": "Ha Noi", "state_code": "01",
                       "postal_code": "100000", "country_code": "VN",
                       "address_line_1": "116 Le Duan" },
          "currency": "VND", "location_strategy": "ZONE_BIN",
          "is_active": true, "settings": {}, "extra_data": null,
          "created_at": "2026-09-18T07:43:06.269333+00:00",
          "updated_at": "2026-09-18T07:43:06.269333+00:00",
          "location_count": 2,
          "location_hierarchy": [
            { "id": "683d3724-268e-4680-bfd9-e9da268eb944", "code": "ZONE-A",
              "name": "Zone A", "category": "ZONE", "type": "USER",
              "is_active": true, "parent_id": null,
              "children": [
                { "id": "6a5a9f94-de5d-4273-8e35-01ff867a4c5a", "code": "A-1-1",
                  "name": "Bin A-1-1", "category": "BIN", "type": "USER",
                  "is_active": true,
                  "parent_id": "683d3724-268e-4680-bfd9-e9da268eb944",
                  "children": [] } ] } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  Exit 4 when no such warehouse exists in the tenant.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		f := &wmsWarehousesGet
		requireWMSCode(args[0], "wms warehouses get")

		client, tenant, err := wmsSession(f.tenant, "wms warehouses get")
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.GetWMS(context.Background(), api.WMSWarehouses, args[0], "", "", tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSItem(resp, tenant, f.tenant)
	},
}

// wmsIsActive checks the one filter that is a boolean written as a word.
func wmsIsActive(value, subject string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v != "" && v != "true" && v != "false" {
		failValidation("%s: --is-active takes true or false (got %q)", subject, value)
	}
	return v
}

// --------------------------------------------------------------------------
// wms locations
// --------------------------------------------------------------------------

var wmsLocationsCmd = &cobra.Command{
	Use:   "locations",
	Short: "List and read the locations inside a warehouse",
	Long: `Locations: the zones, racks and bins inside one warehouse.

A location's code is what an internal transfer's lines name as
from_location_code and to_location_code. Codes are unique within a warehouse,
so every command here takes --warehouse.

USAGE
  capigo wms locations <command> --tenant <code> --warehouse <code> [<args>]`,
}

var wmsLocationsList wmsFlags

var wmsLocationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the locations of a warehouse",
	Long: `List the locations inside one warehouse, as a flat page.

PURPOSE
  Find a location's code, by searching its code or name. The list is flat:
  parent_id says where a location sits, and wms warehouses get shows the same
  locations as a tree.

USAGE
  capigo wms locations list --tenant <code> --warehouse <code> [-q <term>]
          [--is-active <true|false>] [--sort <column>:<asc|desc>]
          [--page <n>] [--limit <n>]

FLAGS
` + wmsTenantFlagHelp + `

` + wmsWarehouseFlagHelp + `

        capigo wms locations list --tenant acme --warehouse WH-HN

  -q, --query <term>
      Search on code and name. Max 200 characters.

        capigo wms locations list --tenant acme --warehouse WH-HN -q A-1

  --is-active <true|false>
      Only active, or only inactive, locations. Left out, both are listed.

  --sort <column>:<asc|desc>
      Order by created_at, updated_at, code or name. The default is
      created_at:desc.

` + wmsPagingFlagHelp + `

OUTPUT
  The locations are at .data[]; read meta.total for the full count:

      { "data": [
          { "id": "6a5a9f94-de5d-4273-8e35-01ff867a4c5a", "code": "A-1-1",
            "name": "Bin A-1-1", "category": "BIN", "type": "USER",
            "is_active": true,
            "parent_id": "683d3724-268e-4680-bfd9-e9da268eb944",
            "created_at": "2026-10-12T13:16:02.872286+00:00" } ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 14, "has_more": false } }

  Exit 5 without --warehouse.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		f := &wmsLocationsList
		warehouse := requireWarehouse(f.warehouse, "wms locations list")
		validateWMSLimit(f.limit)
		isActive := wmsIsActive(f.isActive, "wms locations list")

		client, tenant, err := wmsSession(f.tenant, "wms locations list")
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.ListWMS(context.Background(), api.WMSLocations, api.WMSListQuery{
			Warehouse: warehouse, Query: f.query, IsActive: isActive, Sort: f.sort, Page: f.page, Limit: f.limit,
		}, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSList(resp, tenant, f.tenant)
	},
}

var wmsLocationsGet wmsFlags

var wmsLocationsGetCmd = &cobra.Command{
	Use:   "get <code>",
	Short: "Get a location by its code",
	Long: `Get one location of a warehouse by its code.

PURPOSE
  Read a location with its parent and its direct children — enough to confirm
  a code before naming it on an internal transfer's line.

USAGE
  capigo wms locations get <code> --tenant <code> --warehouse <code>

FLAGS
  <code>
      Location code, as wms locations list prints it. Positional, required.

` + wmsTenantFlagHelp + `

` + wmsWarehouseFlagHelp + `

        capigo wms locations get A-1-1 --tenant acme --warehouse WH-HN

OUTPUT
  The location is at .data. parent is null for a top-level location, and
  children[] holds only the locations directly beneath this one:

      { "data": {
          "id": "6a5a9f94-de5d-4273-8e35-01ff867a4c5a", "code": "A-1-1",
          "name": "Bin A-1-1", "category": "BIN", "type": "USER",
          "is_active": true,
          "parent_id": "683d3724-268e-4680-bfd9-e9da268eb944",
          "capacity_units": null, "extra_data": null,
          "parent": { "id": "683d3724-268e-4680-bfd9-e9da268eb944",
                      "code": "ZONE-A", "name": "Zone A", "category": "ZONE",
                      "type": "USER" },
          "children": [],
          "created_at": "2026-10-12T13:16:02.872286+00:00",
          "updated_at": "2026-10-12T13:16:02.872286+00:00" },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  Exit 4 when the warehouse holds no such location; a location of another
  warehouse answers the same way.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		f := &wmsLocationsGet
		requireWMSCode(args[0], "wms locations get")
		warehouse := requireWarehouse(f.warehouse, "wms locations get")

		client, tenant, err := wmsSession(f.tenant, "wms locations get")
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.GetWMS(context.Background(), api.WMSLocations, args[0], warehouse, "", tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSItem(resp, tenant, f.tenant)
	},
}

// --------------------------------------------------------------------------
// wms availability check
// --------------------------------------------------------------------------

var wmsAvailabilityCmd = &cobra.Command{
	Use:   "availability",
	Short: "Ask how well warehouses can cover a demand",
	Long: `Availability: a supply signal for a caller who may not see stock.

The answer never carries on-hand quantities. It scores, from 0 to 100, how well
each named warehouse covers each line, and says how much can be promised where
a warehouse falls short.

USAGE
  capigo wms availability check --tenant <code> --warehouse <code>... --line <line>...`,
}

var wmsAvailabilityCheck wmsFlags

var wmsAvailabilityCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Score how well each warehouse covers each line of demand",
	Long: `Score how well each named warehouse covers each line of demand.

PURPOSE
  Before quoting or planning an order, learn which warehouse can supply it.
  Each line names one variant and a quantity; the answer scores every named
  warehouse for every line. It reads and writes nothing, and it never reveals
  how much stock a warehouse holds.

USAGE
  capigo wms availability check --tenant <code>
          (--warehouse <code>... --line <line>... | --from-json <path|->)

FLAGS
  --tenant <code>
      Tenant to check. Required; a default tenant counts.

  --warehouse <code>   (repeatable)
      A warehouse to score, by its code. At least one, at most 100. A code
      that is unknown, inactive or of another tenant exits 5 with code E9647
      and scores nothing.

  --line <line>   (repeatable)
      One variant and the quantity wanted, written sku=<sku>,demand=<n>.
      barcode=<code> and variant-id=<uuid> name the variant the other two ways.
      The demand is in the variant's base unit and must be above 0. At least
      one line, at most 100. Lines are answered in the order given.

        capigo wms availability check --tenant acme \
            --warehouse WH-HN --warehouse WH-HCM \
            --line sku=SKU-A1,demand=5 --line barcode=8930000000011,demand=2

  --from-json <path|->
      Send the whole request body from a file, or - for stdin. Mutually
      exclusive with --warehouse and --line: passing both exits 5.

      Body:

          { "warehouses": ["WH-HN", "WH-HCM"],
            "lines": [ { "sku": "SKU-A1", "demand": 5 },
                       { "variant_id": "8fc55e09-8c25-4c60-90a5-87b310c33b4a",
                         "demand": 2 } ] }

OUTPUT
  One row per line is at .data[], in the order the lines were given:

      { "data": [
          { "line_no": 1, "found": true,
            "warehouses": [
              { "code": "WH-HN", "name": "Kho Ha Noi", "score": 100,
                "promisable_qty": null },
              { "code": "WH-HCM", "name": "Kho Ho Chi Minh", "score": 40,
                "promisable_qty": 2 } ] },
          { "line_no": 2, "found": false, "warehouses": [] } ],
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  A row names its line only by line_no, the 1-based position of the line in
  the request: match it back to what you sent. found false means the variant
  was not found, and its warehouses[] is empty — that is not a score of 0.

  score 100 with promisable_qty null means the warehouse covers the whole
  demand. A lower score means it cannot, and promisable_qty is how much of the
  demand it can promise; 0 means none.

  Exit 5 when no warehouse or no line is given, or a line is malformed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		f := &wmsAvailabilityCheck
		const subject = "wms availability check"

		var body any
		if f.fromJSON != "" {
			for _, name := range []string{"warehouse", "line"} {
				if cmd.Flags().Changed(name) {
					failValidation("--from-json and --%s are mutually exclusive", name)
				}
			}
			raw, err := readWMSBody(f.fromJSON, subject)
			if err != nil {
				return handleErr(err)
			}
			body = raw
		} else {
			warehouses := dedupeCodes(f.warehouses)
			if len(warehouses) == 0 {
				failValidation("%s: give at least one --warehouse <code>", subject)
			}
			if len(f.lines) == 0 {
				failValidation("%s: give at least one --line, written sku=<sku>,demand=<n>", subject)
			}
			if len(warehouses) > 100 || len(f.lines) > 100 {
				failValidation("%s: a check takes at most 100 warehouses and 100 lines", subject)
			}
			lines := make([]map[string]any, 0, len(f.lines))
			for _, raw := range f.lines {
				line, err := api.ParseAvailabilityLine(raw)
				if err != nil {
					failValidation("%s: %v", subject, err)
				}
				lines = append(lines, line)
			}
			body = map[string]any{"warehouses": warehouses, "lines": lines}
		}

		client, tenant, err := wmsSession(f.tenant, subject)
		if err != nil {
			return handleErr(err)
		}
		resp, err := client.CheckWMSAvailability(context.Background(), body, tenant)
		if err != nil {
			return handleErr(err)
		}
		return writeWMSList(resp, tenant, f.tenant)
	},
}

// --------------------------------------------------------------------------
// The four documents
// --------------------------------------------------------------------------

// wmsDocument describes one document family. The four share every command's
// mechanics — the same flags, the same headers, the same lifecycle verbs — and
// differ in their nouns, their statuses, and what a body and an answer hold.
// Those differences are stated here once and the commands are built from them.
type wmsDocument struct {
	use      string // the group's name: "inbound"
	resource string // the API's path segment: "inbound-receipts"
	noun     string // "inbound receipt"
	plural   string // "inbound receipts"
	code     string // a sample code, for the examples
	short    string
	about    string // the group page's prose

	search    string // what -q matches
	sortBy    string // the columns --sort accepts
	listRow   string // one row of the list sample
	listNote  string // a sentence after the list sample, or ""
	getSample string // the whole .data of a get
	getNote   string

	body        string // an example create body
	bodyNote    string // what the body's fields mean
	updateNote  string // what an update may change
	previewOut  string
	validateOut string

	// costMethod is set where get reads --cost-method.
	costMethod bool
	// submit is false where the document has no submit action.
	submit     bool
	submitNote string
	// cancelReason is false where cancel sends no body.
	cancelReason bool
	// cancelIfMatch is false where cancel takes no concurrency token.
	cancelIfMatch bool
	cancelNote    string
}

// wmsDocumentCommands are one family's commands, kept so the coverage guards
// can ask each for its flags.
type wmsDocumentCommands struct {
	group    *cobra.Command
	list     *cobra.Command
	get      *cobra.Command
	create   *cobra.Command
	preview  *cobra.Command
	validate *cobra.Command
	update   *cobra.Command
	submit   *cobra.Command // nil where the document has no submit
	cancel   *cobra.Command
}

func (d wmsDocument) fill(text string, pairs ...string) string {
	statuses := api.WMSStatuses(d.resource)
	base := []string{
		"use", d.use, "noun", d.noun, "a", d.a(), "plural", d.plural, "code", d.code,
		"statuses", strings.Join(statuses, ", "),
		"status1", statuses[0], "status2", statuses[1],
		"tenantFlag", wmsTenantFlagHelp, "warehouseFlag", wmsWarehouseFlagHelp,
		"paging", wmsPagingFlagHelp,
	}
	return fill(text, append(pairs, base...)...)
}

// a is the noun with its article: "an inbound receipt", "a warehouse transfer".
func (d wmsDocument) a() string {
	if strings.ContainsRune("aeiou", rune(d.noun[0])) {
		return "an " + d.noun
	}
	return "a " + d.noun
}

func (d wmsDocument) subject(verb string) string {
	return "wms " + d.use + " " + verb
}

func (d wmsDocument) newList() *cobra.Command {
	f := &wmsFlags{}
	c := &cobra.Command{
		Use:   "list",
		Short: "List the " + d.plural + " of a warehouse",
		Long: d.fill(`List the {plural} of one warehouse.

PURPOSE
  Read the {plural} of one warehouse,
  newest first, optionally narrowed by status or by a search term. A row is a
  summary: the lines and the history of one are on the get command.

USAGE
  capigo wms {use} list --tenant <code> --warehouse <code>
          [-q <term>] [--status <status>]... [--sort <column>:<asc|desc>]
          [--page <n>] [--limit <n>]

FLAGS
{tenantFlag}

{warehouseFlag}

        capigo wms {use} list --tenant acme --warehouse WH-HN

  -q, --query <term>
      {search} Max 200 characters.

  --status <status>   (repeatable)
      Only documents in this status, one of:
      {statuses}
      Repeat the flag, or separate several with commas, to match any of them.
      Case is ignored. A status this document does not have exits 5 before
      any call.

        capigo wms {use} list --tenant acme --warehouse WH-HN \
            --status {status1},{status2}

  --sort <column>:<asc|desc>
      Order by {sortBy}.
      The default is created_at:desc. Any other column exits 5.

{paging}

OUTPUT
  The {plural} are at .data[]; read meta.total for the full count:

      { "data": [
{listRow} ],
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "page": 1, "limit": 20, "total": 1, "has_more": false } }
{listNote}
  Exit 5 without --warehouse.`,
			"search", d.search, "sortBy", d.sortBy, "listRow", d.listRow, "listNote", d.listNote),
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			subject := d.subject("list")
			warehouse := requireWarehouse(f.warehouse, subject)
			validateWMSLimit(f.limit)
			statuses, err := api.NormalizeWMSStatuses(d.resource, f.statuses)
			if err != nil {
				failValidation("%s: --status: %v", subject, err)
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.ListWMS(context.Background(), d.resource, api.WMSListQuery{
				Warehouse: warehouse, Query: f.query, Statuses: statuses, Sort: f.sort, Page: f.page, Limit: f.limit,
			}, tenant)
			if err != nil {
				return handleErr(err)
			}
			return writeWMSList(resp, tenant, f.tenant)
		},
	}
	c.Flags().StringVar(&f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	c.Flags().StringVar(&f.warehouse, "warehouse", "", "warehouse code (required)")
	c.Flags().StringVarP(&f.query, "query", "q", "", "search term")
	c.Flags().StringArrayVar(&f.statuses, "status", nil, "only this status; repeatable, or comma-separated")
	c.Flags().StringVar(&f.sort, "sort", "", "column:asc or column:desc")
	c.Flags().IntVar(&f.page, "page", 0, "page to fetch, from 1")
	c.Flags().IntVar(&f.limit, "limit", 0, "rows per page, 1 to 50")
	return c
}

func (d wmsDocument) newGet() *cobra.Command {
	f := &wmsFlags{}
	usage, costFlag := "", ""
	if d.costMethod {
		usage = "\n          [--cost-method <fifo|average>]"
		costFlag = `

  --cost-method <fifo|average>
      How unit_cost and total_value are measured. fifo, the default, is the
      cost of the stock layers each line consumed, first in, first out.
      average is the variant's weighted-average cost in the warehouse when the
      shipment posted. Any other value exits 5.`
	}
	c := &cobra.Command{
		Use:   "get <code>",
		Short: "Get one " + d.noun + " with its lines and history",
		Long: d.fill(`Get one {noun} by its code, with its lines and its history.

PURPOSE
  Read {a} in full:
  every line, and what has happened to it. This is also where updated_at
  comes from — the value the writes on this document take as their
  concurrency token.

USAGE
  capigo wms {use} get <code> --tenant <code> --warehouse <code>{costUsage}

FLAGS
  <code>
      The document's code, as the list command prints it. Positional,
      required.

{tenantFlag}

{warehouseFlag}

        capigo wms {use} get {code} --tenant acme \
            --warehouse WH-HN{costFlag}

OUTPUT
  The document is at .data:

{getSample}

{getNote}

  Exit 4 when the warehouse holds no such document.`,
			"costUsage", usage, "costFlag", costFlag, "getSample", d.getSample, "getNote", d.getNote),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			subject := d.subject("get")
			requireWMSCode(args[0], subject)
			warehouse := requireWarehouse(f.warehouse, subject)
			costMethod := strings.ToLower(strings.TrimSpace(f.costMethod))
			if costMethod != "" && costMethod != "fifo" && costMethod != "average" {
				failValidation("%s: --cost-method takes fifo or average (got %q)", subject, f.costMethod)
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.GetWMS(context.Background(), d.resource, args[0], warehouse, costMethod, tenant)
			if err != nil {
				return handleErr(err)
			}
			return writeWMSItem(resp, tenant, f.tenant)
		},
	}
	c.Flags().StringVar(&f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	c.Flags().StringVar(&f.warehouse, "warehouse", "", "warehouse code (required)")
	if d.costMethod {
		c.Flags().StringVar(&f.costMethod, "cost-method", "", "fifo (default) or average")
	}
	return c
}

// newPost builds create, preview and validate: one body, three destinations.
func (d wmsDocument) newPost(stage string) *cobra.Command {
	f := &wmsFlags{}
	verb := stage
	if stage == api.WMSStageCreate {
		verb = "create"
	}

	var short, long string
	switch stage {
	case api.WMSStageCreate:
		short = "Create " + d.a() + " as a draft"
		long = `Create {a}, as a DRAFT.

PURPOSE
  Plan {a} in one warehouse.
  The header and every line are written together or not at all. The new
  document is a DRAFT; nothing has happened to stock. To check a body first
  without writing it, send the same file to the validate or preview command
  beside this one.

USAGE
  capigo wms {use} create --tenant <code> --warehouse <code>
          --from-json <path|-> [--idempotency-key <key>]

FLAGS
{tenantFlag}

{warehouseFlag}

  --from-json <path|->
      The request body, from a file, or - for stdin. Required: a document is
      its lines, and no flag describes a list of them.

      Body:

{body}

{bodyNote}

        capigo wms {use} create --tenant acme --warehouse WH-HN \
            --from-json {use}.json

  --idempotency-key <key>
      Make this create safe to retry. Send the same key and the same body
      again after a call that failed mid-flight, and the API answers with the
      document it already created instead of creating a second one. The same
      key with a different body exits 8 with code E0601.

        capigo wms {use} create --tenant acme --warehouse WH-HN \
            --from-json {use}.json \
            --idempotency-key "restock-$(date +%s)"

OUTPUT
  The created document is at .data, in the same shape as
  wms {use} get.
  Read .data.code: it is how every later command names this document, and
  .data.updated_at is the --if-match its first write will need.

      { "data": { "id": "99b4ebd7-8413-4b1e-af61-4d87592419cc",
                  "code": "{code}", "status": "DRAFT", … },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-12T08:40:05Z" } }

  meta.replayed is true when nothing was created: an earlier call with this
  --idempotency-key had already created the document, and .data is that one.

  If .data holds "echo": "unavailable" in place of a document, the write
  succeeded and only the read-back failed. Do not create it again: read it
  with the get command, using the code in .data.code.

  Exit 5 for a body the API refuses. A line naming a variant that does not
  exist in the tenant is refused too, with exit 4 or 5 and the reason.`
	case api.WMSStagePreview:
		short = "Show what " + d.a() + " body would create, writing nothing"
		long = `Show what a create body would produce, without writing anything.

PURPOSE
  Send a create body and read back how the API understood it. Nothing is
  written and no event is emitted. For a yes-or-no answer on the same body,
  use the validate command beside this one.

USAGE
  capigo wms {use} preview --tenant <code> --warehouse <code>
          --from-json <path|->

FLAGS
{tenantFlag}

{warehouseFlag}

  --from-json <path|->
      The body a create would be given, from a file, or - for stdin.
      Required. Its fields are described on the create command's page.

        capigo wms {use} preview --tenant acme --warehouse WH-HN \
            --from-json {use}.json

OUTPUT
{previewOut}`
	default:
		short = "Check " + d.a() + " body, writing nothing"
		long = `Check a create body against the create's own rules, without writing anything.

PURPOSE
  Learn whether a create would accept a body, before sending it. Nothing is
  written and no event is emitted. To see how the body was understood, use
  the preview command beside this one.

USAGE
  capigo wms {use} validate --tenant <code> --warehouse <code>
          --from-json <path|->

FLAGS
{tenantFlag}

{warehouseFlag}

  --from-json <path|->
      The body a create would be given, from a file, or - for stdin.
      Required. Its fields are described on the create command's page.

        capigo wms {use} validate --tenant acme --warehouse WH-HN \
            --from-json {use}.json

OUTPUT
{validateOut}`
	}

	c := &cobra.Command{
		Use:   verb,
		Short: short,
		Long: d.fill(long, "body", d.body, "bodyNote", d.bodyNote,
			"previewOut", d.previewOut, "validateOut", d.validateOut),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			subject := d.subject(verb)
			warehouse := requireWarehouse(f.warehouse, subject)
			key := ""
			if stage == api.WMSStageCreate {
				key = requireUsableKey(cmd.Flags().Changed("idempotency-key"), f.idemKey, "idempotency-key")
			}
			body, err := readWMSBody(f.fromJSON, subject)
			if err != nil {
				return handleErr(err)
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.PostWMSDocument(context.Background(), d.resource, stage, warehouse, body, key, tenant)
			if err != nil {
				return handleErr(err)
			}
			if stage != api.WMSStageCreate {
				return writeWMSItem(resp, tenant, f.tenant)
			}
			// 201 is a document created; 200 is the server saying it created
			// nothing, because this key had already made one.
			return writeWMSWrite(resp, tenant, f.tenant, resp.StatusCode == http.StatusOK)
		},
	}
	c.Flags().StringVar(&f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	c.Flags().StringVar(&f.warehouse, "warehouse", "", "warehouse code (required)")
	c.Flags().StringVar(&f.fromJSON, "from-json", "", "path to a JSON file holding the request body (use - for stdin)")
	if stage == api.WMSStageCreate {
		c.Flags().StringVar(&f.idemKey, "idempotency-key", "", "make the create safe to retry")
	}
	return c
}

func (d wmsDocument) newUpdate() *cobra.Command {
	f := &wmsFlags{}
	c := &cobra.Command{
		Use:   "update <code>",
		Short: "Change an editable " + d.noun,
		Long: d.fill(`Change {a} that is still editable.

PURPOSE
  Edit the reference, the notes or the lines of a document. This is a merge
  patch: a field the body leaves out is left alone, and null clears
  external_ref or notes. The status is not a field — it moves through the
  lifecycle commands.

{updateNote}

USAGE
  capigo wms {use} update <code> --tenant <code> --warehouse <code>
          --if-match <updated_at> --from-json <path|->

FLAGS
  <code>
      The document's code. Positional, required.

{tenantFlag}

{warehouseFlag}

  --if-match <updated_at>
      The document's current updated_at, copied exactly as the get command
      prints it. Required. If the document has changed since that read the
      call exits 8 with code E0602 and nothing is written: read it again and
      retry with the new value.

  --from-json <path|->
      The fields to change, from a file, or - for stdin. Required.

      Body:

          { "notes": "Dock 3, after 14:00", "external_ref": null }

        capigo wms {use} update {code} --tenant acme \
            --warehouse WH-HN --from-json patch.json \
            --if-match 2026-10-12T08:40:05.752079+00:00

OUTPUT
  The document as it now stands is at .data, in the same shape as
  wms {use} get.
  Its updated_at has moved: use the new one for the next write.

      { "data": { "id": "99b4ebd7-8413-4b1e-af61-4d87592419cc",
                  "code": "{code}", "status": "DRAFT",
                  "updated_at": "2026-10-12T09:02:11.118204+00:00", … },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-12T09:02:11Z" } }

  Exit 8 when --if-match is stale (E0602), or when the document is past the
  point where it can be edited. Exit 5 for a body the API refuses, including
  one that names status. Exit 4 when the warehouse holds no such document.`,
			"updateNote", d.updateNote),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			subject := d.subject("update")
			requireWMSCode(args[0], subject)
			warehouse := requireWarehouse(f.warehouse, subject)
			ifMatch := requireIfMatch(f.ifMatch, subject, "capigo wms "+d.use+" get")
			body, err := readWMSBody(f.fromJSON, subject)
			if err != nil {
				return handleErr(err)
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.UpdateWMSDocument(context.Background(), d.resource, args[0], warehouse, ifMatch, body, tenant)
			if err != nil {
				return handleErr(err)
			}
			return writeWMSWrite(resp, tenant, f.tenant, false)
		},
	}
	c.Flags().StringVar(&f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	c.Flags().StringVar(&f.warehouse, "warehouse", "", "warehouse code (required)")
	c.Flags().StringVar(&f.ifMatch, "if-match", "", "the document's current updated_at (required)")
	c.Flags().StringVar(&f.fromJSON, "from-json", "", "path to a JSON file holding the fields to change (use - for stdin)")
	return c
}

// newAction builds submit and cancel, the two lifecycle actions the API lets a
// key perform. Every posting verb — receive, pick, export, move, approve —
// answers 404 on the wire, so there is no command for them to mislead with.
func (d wmsDocument) newAction(action string) *cobra.Command {
	f := &wmsFlags{}
	withIfMatch := action == "submit" || d.cancelIfMatch
	withReason := action == "cancel" && d.cancelReason

	usage := "capigo wms {use} " + action + " <code> --tenant <code> --warehouse <code>"
	flags := ""
	example := "capigo wms {use} " + action + " {code} --tenant acme \\\n            --warehouse WH-HN"
	if withIfMatch {
		usage += "\n          --if-match <updated_at>"
		example += " \\\n            --if-match 2026-10-12T08:40:05.752079+00:00"
		flags += `

  --if-match <updated_at>
      The document's current updated_at, copied exactly as the get command
      prints it. Required. If the document has changed since that read the
      call exits 8 with code E0602 and nothing happens: read it again and
      retry with the new value.`
	}
	if withReason {
		usage += "\n          --reason <text>"
		example += " \\\n            --reason \"Planned in error, replaced by a new document\""
		flags += `

  --reason <text>
      Why the document is being cancelled. Required, at least 10 characters;
      a shorter one exits 5 before any call.`
	}

	short, purpose := "", ""
	if action == "submit" {
		short = "Submit a draft " + d.noun
		purpose = d.submitNote
	} else {
		short = "Cancel " + d.a()
		purpose = d.cancelNote
	}

	c := &cobra.Command{
		Use:   action + " <code>",
		Short: short,
		Long: d.fill(short+`.

PURPOSE
{purpose}

USAGE
  {usage}

FLAGS
  <code>
      The document's code. Positional, required.

{tenantFlag}

{warehouseFlag}{flags}

        {example}

OUTPUT
  The document as it now stands is at .data, in the same shape as
  wms {use} get.
  Read .data.status: it is the answer to whether the action took.

      { "data": { "id": "99b4ebd7-8413-4b1e-af61-4d87592419cc",
                  "code": "{code}", "status": "{after}", … },
        "meta": { "tenant": "acme", "tenant_source": "flag",
                  "server_time": "2026-10-12T09:02:11Z" } }

  Exit 8 when the document's status does not allow this action.{stale}
  Exit 4 when the warehouse holds no such document.`,
			"purpose", purpose, "usage", usage, "flags", flags, "example", example,
			"after", map[string]string{"submit": api.WMSStatuses(d.resource)[1], "cancel": "CANCELLED"}[action],
			"stale", map[bool]string{true: " Exit 8, with\n  code E0602, when --if-match is stale.", false: ""}[withIfMatch]),
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			subject := d.subject(action)
			requireWMSCode(args[0], subject)
			warehouse := requireWarehouse(f.warehouse, subject)
			ifMatch := ""
			if withIfMatch {
				ifMatch = requireIfMatch(f.ifMatch, subject, "capigo wms "+d.use+" get")
			}
			var body any
			if withReason {
				reason := strings.TrimSpace(f.reason)
				if len([]rune(reason)) < 10 {
					failValidation("%s: --reason is required and must be at least 10 characters", subject)
				}
				body = map[string]string{"reason": reason}
			}

			client, tenant, err := wmsSession(f.tenant, subject)
			if err != nil {
				return handleErr(err)
			}
			resp, err := client.ActOnWMSDocument(context.Background(), d.resource, args[0], action, warehouse, ifMatch, body, tenant)
			if err != nil {
				return handleErr(err)
			}
			return writeWMSWrite(resp, tenant, f.tenant, false)
		},
	}
	c.Flags().StringVar(&f.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	c.Flags().StringVar(&f.warehouse, "warehouse", "", "warehouse code (required)")
	if withIfMatch {
		c.Flags().StringVar(&f.ifMatch, "if-match", "", "the document's current updated_at (required)")
	}
	if withReason {
		c.Flags().StringVar(&f.reason, "reason", "", "why the document is cancelled, at least 10 characters (required)")
	}
	return c
}

// build assembles a family's group. Children are added in the order a caller
// meets them: the reads, the dry runs, then the writes.
func (d wmsDocument) build() wmsDocumentCommands {
	cmds := wmsDocumentCommands{
		group: &cobra.Command{
			Use:   d.use,
			Short: d.short,
			Long: d.fill(d.about + `

USAGE
  capigo wms {use} <command> --tenant <code> --warehouse <code>
          [<args>]`),
		},
		list:     d.newList(),
		get:      d.newGet(),
		validate: d.newPost(api.WMSStageValidate),
		preview:  d.newPost(api.WMSStagePreview),
		create:   d.newPost(api.WMSStageCreate),
		update:   d.newUpdate(),
		cancel:   d.newAction("cancel"),
	}
	cmds.group.AddCommand(cmds.list, cmds.get, cmds.validate, cmds.preview, cmds.create, cmds.update)
	if d.submit {
		cmds.submit = d.newAction("submit")
		cmds.group.AddCommand(cmds.submit)
	}
	cmds.group.AddCommand(cmds.cancel)
	return cmds
}

var (
	wmsInbound            = wmsInboundDocument.build()
	wmsOutbound           = wmsOutboundDocument.build()
	wmsInternalTransfers  = wmsInternalTransferDocument.build()
	wmsWarehouseTransfers = wmsWarehouseTransferDocument.build()
)

func init() {
	wmsWarehousesListCmd.Flags().StringVar(&wmsWarehousesList.tenant, "tenant", "", "tenant to read (required)")
	wmsWarehousesListCmd.Flags().StringVarP(&wmsWarehousesList.query, "query", "q", "", "search on code and name")
	wmsWarehousesListCmd.Flags().StringVar(&wmsWarehousesList.isActive, "is-active", "", "true or false")
	wmsWarehousesListCmd.Flags().StringVar(&wmsWarehousesList.sort, "sort", "", "column:asc or column:desc")
	wmsWarehousesListCmd.Flags().IntVar(&wmsWarehousesList.page, "page", 0, "page to fetch, from 1")
	wmsWarehousesListCmd.Flags().IntVar(&wmsWarehousesList.limit, "limit", 0, "rows per page, 1 to 50")

	wmsWarehousesGetCmd.Flags().StringVar(&wmsWarehousesGet.tenant, "tenant", "", "tenant the warehouse belongs to (required)")

	wmsLocationsListCmd.Flags().StringVar(&wmsLocationsList.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	wmsLocationsListCmd.Flags().StringVar(&wmsLocationsList.warehouse, "warehouse", "", "warehouse code (required)")
	wmsLocationsListCmd.Flags().StringVarP(&wmsLocationsList.query, "query", "q", "", "search on code and name")
	wmsLocationsListCmd.Flags().StringVar(&wmsLocationsList.isActive, "is-active", "", "true or false")
	wmsLocationsListCmd.Flags().StringVar(&wmsLocationsList.sort, "sort", "", "column:asc or column:desc")
	wmsLocationsListCmd.Flags().IntVar(&wmsLocationsList.page, "page", 0, "page to fetch, from 1")
	wmsLocationsListCmd.Flags().IntVar(&wmsLocationsList.limit, "limit", 0, "rows per page, 1 to 50")

	wmsLocationsGetCmd.Flags().StringVar(&wmsLocationsGet.tenant, "tenant", "", "tenant the warehouse belongs to (required)")
	wmsLocationsGetCmd.Flags().StringVar(&wmsLocationsGet.warehouse, "warehouse", "", "warehouse code (required)")

	wmsAvailabilityCheckCmd.Flags().StringVar(&wmsAvailabilityCheck.tenant, "tenant", "", "tenant to check (required)")
	wmsAvailabilityCheckCmd.Flags().StringArrayVar(&wmsAvailabilityCheck.warehouses, "warehouse", nil, "warehouse code to score; repeatable")
	wmsAvailabilityCheckCmd.Flags().StringArrayVar(&wmsAvailabilityCheck.lines, "line", nil, "sku=<sku>,demand=<n>; repeatable")
	wmsAvailabilityCheckCmd.Flags().StringVar(&wmsAvailabilityCheck.fromJSON, "from-json", "", "path to a JSON file holding the request body (use - for stdin)")

	wmsWarehousesCmd.AddCommand(wmsWarehousesListCmd, wmsWarehousesGetCmd)
	wmsLocationsCmd.AddCommand(wmsLocationsListCmd, wmsLocationsGetCmd)
	wmsAvailabilityCmd.AddCommand(wmsAvailabilityCheckCmd)

	wmsCmd.AddCommand(
		wmsWarehousesCmd,
		wmsLocationsCmd,
		wmsAvailabilityCmd,
		wmsInbound.group,
		wmsOutbound.group,
		wmsInternalTransfers.group,
		wmsWarehouseTransfers.group,
	)
	rootCmd.AddCommand(wmsCmd)
}
