package cmd

import "github.com/vtech-com/capigo-api-sdk/internal/api"

// The four WMS documents. Each value states what is particular to one family;
// wms.go builds the same eight commands from every one of them.
//
// The samples are real answers with the names changed. A page that shows a
// schema name instead teaches a reader nothing about a field it left out.

const wmsVariantSample = `{ "id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                "sku": "SKU-A1", "barcode": "8930000000011",
                "variant_name": "Charger 20W - White",
                "product_id": "964d6637-d751-4e90-bcac-a74de1d16e51",
                "product_name": "Charger 20W" }`

const wmsWarehouseSample = `{ "id": "932bb586-3180-481f-b506-e90a9c3ebbb0",
                         "code": "WH-HN", "name": "Kho Ha Noi" }`

const wmsLineVariantNote = `      Each line names its variant by sku or by product_variant_id.`

const wmsDryRunRefusalNote = `  A body the API cannot read at all — no lines, a missing warehouse — is not
  a verdict: it exits 5 with an error, like any refused request.`

var wmsInboundDocument = wmsDocument{
	use:      "inbound",
	resource: api.WMSInboundReceipts,
	noun:     "inbound receipt",
	plural:   "inbound receipts",
	code:     "IB-WHHN-52429",
	short:    "Plan and read inbound receipts",
	about: `Inbound receipts: stock a warehouse expects to receive.

A receipt is created as a DRAFT, and submit makes it PLANNED — the point from
which the warehouse can receive against it. RECEIVING and VERIFIED are reached
in the Capigo app, as goods are counted; VERIFIED is the terminal, posted
state, where the stock is on the ledger. No command here receives or verifies.

Costs are shown only to a member holding the financial-view scope. Without it
unit_cost and total_value are null and unit_cost_denied / total_value_denied
are true: a null cost there means "not shown", never "free".`,

	search: "Search on code and external reference.",
	sortBy: "created_at or updated_at",
	listRow: `          { "id": "b89c4dae-d3dd-43b4-8dab-abd99825e870",
            "code": "{code}", "status": "DRAFT", "rel_type": null,
            "external_ref": "PO-2291", "notes": null, "line_count": 2,
            "total_qty_expected": 50, "total_qty_received": 0,
            "total_value": 250000, "total_value_denied": false,
            "verified_at": null,
            "created_at": "2026-10-12T08:40:05.752079+00:00",
            "updated_at": "2026-10-12T08:40:05.752079+00:00",
            "warehouse": ` + wmsWarehouseSample + ` }`,
	listNote: `
  total_value is null with total_value_denied true when this key's member may
  not see costs. That null is not a zero.
`,
	getSample: `      { "data": {
          "id": "b89c4dae-d3dd-43b4-8dab-abd99825e870", "code": "{code}",
          "status": "DRAFT", "rel_type": null, "external_ref": "PO-2291",
          "notes": null, "line_count": 1, "total_qty_expected": 40,
          "total_qty_received": 0, "total_value": 200000,
          "total_value_denied": false, "verified_at": null,
          "created_at": "2026-10-12T08:40:05.752079+00:00",
          "updated_at": "2026-10-12T08:40:05.752079+00:00",
          "warehouse": ` + wmsWarehouseSample + `,
          "owner": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb" },
          "cancel_locked": false, "parent": null,
          "lines": [
            { "id": "b82cabe2-38a2-4a8d-84ca-9f1512db340e",
              "product_variant": ` + wmsVariantSample + `,
              "qty_expected": 40, "qty_received": 0, "unit_cost": 5000,
              "unit_cost_denied": false, "variance_notes": null,
              "variance_type": null } ],
          "status_history": [
            { "status": "DRAFT",
              "created_at": "2026-10-12T08:40:05.752079+00:00",
              "created_by_name": "Tram Nguyen" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	getNote: `  parent is null for a free-standing receipt. Otherwise it names the warehouse
  transfer this receipt belongs to — { "type": "warehouse_transfer", "id": …,
  "code": … } — and such a receipt is cancelled only through that transfer.
  cancel_locked true means only owner, the member who created it, may cancel.

  unit_cost and total_value are null with unit_cost_denied / total_value_denied
  true when this key's member may not see costs.`,

	body: `          { "external_ref": "PO-2291", "notes": "Dock 3",
            "lines": [
              { "sku": "SKU-A1", "qty_expected": 40, "unit_cost": 5000 },
              { "product_variant_id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                "qty_expected": 10 } ] }`,
	bodyNote: `      lines is required and must not be empty; qty_expected is required on
      each.
` + wmsLineVariantNote + `
      unit_cost may be left out of a draft, but submit refuses a receipt
      with a line that expects stock and has no cost; 0 is a valid cost.
      external_ref, notes, extra_data, rel_type and rel_id are optional.
      cancel_locked true lets only this key's member cancel the receipt, and
      cannot be changed afterwards.`,
	updateNote: `  While the receipt is DRAFT or PLANNED, lines replaces the whole list. Once
  it is RECEIVING, lines is merged by each line's id — give the id from wms
  inbound get to change a line, leave it out to add one — and a line that
  already has a counted quantity may not be dropped (exit 8, E9639). From
  PLANNED on, every line that expects stock needs a unit_cost (exit 5, E9649)
  and the member needs the financial-view scope (exit 3).`,
	previewOut: `  How the body was understood is at .data: the status the receipt would be
  created in, the totals, and each line with the variant it resolved to.

      { "data": { "status": "DRAFT", "line_count": 1,
                  "total_qty_expected": 40,
                  "lines": [
                    { "product_variant_id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                      "qty_expected": 40, "unit_cost": 5000 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  A body that would be refused exits 5 with the reason — a line naming a
  variant that does not exist, for one. Nothing is at .data then.`,
	validateOut: `  The verdict is at .data.valid. Read it: the command exits 0 for a body that
  is not valid, because the check itself succeeded.

      { "data": { "request_id": "8e5469a4-d2e3-4cb9-92fa-c4f4ba09a177",
                  "valid": true,
                  "data": { "line_count": 1, "total_qty_expected": 40 } },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  When valid is false, .data.errors[] says why and there are no totals:

      { "data": { "request_id": "999a51fd-3ffb-4a12-bd31-b5cc1abf9c16",
                  "valid": false,
                  "errors": [ { "code": "VARIANT_NOT_FOUND",
                                "message": "A line references a product variant that does not exist in this tenant" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

` + wmsDryRunRefusalNote,

	submit: true,
	submitNote: `  Hand a DRAFT receipt to the warehouse: it becomes PLANNED and can be
  received against. The member needs the financial-view scope as well as the
  inbound one (exit 3 otherwise), and every line that expects stock needs a
  unit_cost — a line without one exits 5 with code E9649. Set the cost with
  wms inbound update first; 0 is a valid cost.`,
	cancelReason:  true,
	cancelIfMatch: true,
	cancelNote: `  End a receipt that will not be received. It becomes CANCELLED, which is
  final. A receipt that belongs to a warehouse transfer (parent is not null)
  is cancelled only through that transfer, and one with cancel_locked true
  only by its owner: both exit 8 with code E9680.`,
}

var wmsOutboundDocument = wmsDocument{
	use:      "outbound",
	resource: api.WMSOutboundShipments,
	noun:     "outbound shipment",
	plural:   "outbound shipments",
	code:     "OB-WHHN-37867",
	short:    "Plan and read outbound shipments",
	about: `Outbound shipments: stock a warehouse is asked to send out.

A shipment is created as a DRAFT, and submit makes it PLANNED. AWAITING_EXPORT
and EXPORTED are reached in the Capigo app, as goods are picked, packed and
handed over; EXPORTED is the posted state, and it cannot be cancelled. No
command here picks, packs or exports.

A shipment has a cost only once it has posted, and only a member holding the
financial-view scope is shown it. Every write answers unit_cost and
total_value as null — read the shipment with get after it posts for its cost.`,

	search: "Search on code and external reference.",
	sortBy: "created_at or updated_at",
	listRow: `          { "id": "7d4a0a68-73aa-4599-b36f-cc1421013820",
            "code": "{code}", "status": "PLANNED", "rel_type": null,
            "external_ref": "SO-1042", "notes": null, "line_count": 1,
            "total_qty_ordered": 1000, "total_qty_shipped": 0,
            "exported_at": null,
            "created_at": "2026-09-24T01:19:37.461469+00:00",
            "updated_at": "2026-09-24T01:19:59.393474+00:00",
            "warehouse": ` + wmsWarehouseSample + ` }`,
	listNote: `
  A row carries no cost. total_value is on wms outbound get.
`,
	getSample: `      { "data": {
          "id": "7d4a0a68-73aa-4599-b36f-cc1421013820", "code": "{code}",
          "status": "PLANNED", "rel_type": null, "external_ref": "SO-1042",
          "notes": null, "line_count": 1, "total_qty_ordered": 1000,
          "total_qty_shipped": 0, "exported_at": null,
          "created_at": "2026-09-24T01:19:37.461469+00:00",
          "updated_at": "2026-09-24T01:19:59.393474+00:00",
          "warehouse": ` + wmsWarehouseSample + `,
          "total_value": null, "total_value_denied": false,
          "owner": { "id": "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb" },
          "cancel_locked": false, "parent": null,
          "lines": [
            { "id": "23a38e9f-0bf3-4213-a731-722eba487210",
              "product_variant": ` + wmsVariantSample + `,
              "qty_ordered": 1000, "qty_shipped": 0, "unit_cost": null,
              "unit_cost_denied": false } ],
          "status_history": [
            { "status": "PLANNED",
              "created_at": "2026-09-24T01:19:59.393474+00:00",
              "created_by_name": "Tram Nguyen" },
            { "status": "DRAFT",
              "created_at": "2026-09-24T01:19:37.461469+00:00",
              "created_by_name": "Tram Nguyen" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	getNote: `  unit_cost and total_value are the cost of the goods shipped. They are null
  until the shipment has posted. When this key's member may not see costs
  they are null with unit_cost_denied / total_value_denied true — so check the
  denied flag before reading a null as "no cost yet".

  parent is null for a free-standing shipment. Otherwise it names the
  warehouse transfer this shipment belongs to, and such a shipment is
  cancelled only through that transfer. cancel_locked true means only owner,
  the member who created it, may cancel.`,

	body: `          { "external_ref": "SO-1042", "notes": "Fragile",
            "lines": [
              { "sku": "SKU-A1", "qty_ordered": 12 },
              { "product_variant_id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                "qty_ordered": 3 } ] }`,
	bodyNote: `      lines is required and must not be empty.
` + wmsLineVariantNote + `
      external_ref, notes, extra_data, rel_type and rel_id are optional.
      cancel_locked true lets only this key's member cancel the shipment, and
      cannot be changed afterwards.`,
	updateNote: `  On a DRAFT, lines replaces the whole list. From PLANNED on it cannot be
  replaced (exit 8, E9675): the only lines accepted are the current ones with
  one or more qty_ordered lowered, each lowered line carrying a
  qty_reduction_reason (exit 5, E9674, without one). From AWAITING_EXPORT on
  no line may change.`,
	previewOut: `  How the body was understood is at .data.lines[]: each line with the variant
  it resolved to.

      { "data": { "lines": [
                    { "product_variant_id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                      "qty_ordered": 12 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  A body that would be refused exits with the reason — exit 4 with code E9425
  for a line naming a variant that does not exist. Nothing is at .data then.`,
	validateOut: `  The verdict is at .data.valid:

      { "data": { "valid": true },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  A body that would be refused does not come back as valid false: the command
  exits with the reason instead — exit 4 with code E9425 for a line naming a
  variant that does not exist, exit 5 for a body the API cannot read.`,

	costMethod: true,
	submit:     true,
	submitNote: `  Hand a DRAFT shipment to the warehouse: it becomes PLANNED and can be
  picked against.`,
	cancelReason:  false,
	cancelIfMatch: true,
	cancelNote: `  End a shipment that will not be sent. It may be cancelled while DRAFT,
  PLANNED or AWAITING_EXPORT, and becomes CANCELLED, which is final;
  cancelling from AWAITING_EXPORT opens a put-back plan that returns the
  packed goods to storage. An EXPORTED shipment has left and exits 8 with code
  E9671. No reason is sent: this document records none.`,
}

var wmsInternalTransferDocument = wmsDocument{
	use:      "internal-transfers",
	resource: api.WMSInternalTransfers,
	noun:     "internal transfer",
	plural:   "internal transfers",
	code:     "TR-WHHN-96451",
	short:    "Plan and read moves between locations of one warehouse",
	about: `Internal transfers: stock moved between two locations of the same warehouse.

A transfer is created as a DRAFT, where each line holds only a plan. It
becomes MOVING and then MOVED in the Capigo app, as the stock is carried; no
command here moves stock, and there is no submit — this document has no
approval step. cancel ends a DRAFT or a MOVING transfer.`,

	search: "Search on code.",
	sortBy: "created_at, updated_at or code",
	listRow: `          { "id": "2e2a4cad-396a-4e7c-ad59-1316ac37ecdf",
            "code": "{code}", "status": "MOVED", "line_count": 2,
            "total_qty": 1000,
            "created_at": "2026-10-05T02:21:25.097861+00:00",
            "updated_at": "2026-10-05T02:21:25.097861+00:00",
            "warehouse": ` + wmsWarehouseSample + ` }`,
	listNote: "",
	getSample: `      { "data": {
          "id": "2e2a4cad-396a-4e7c-ad59-1316ac37ecdf", "code": "{code}",
          "status": "MOVED", "line_count": 1, "total_qty": 500,
          "created_at": "2026-10-05T02:21:25.097861+00:00",
          "updated_at": "2026-10-05T02:21:25.097861+00:00",
          "warehouse": ` + wmsWarehouseSample + `,
          "lines": [
            { "id": "bee98bde-9bca-4a9e-a360-8ad495f45ef4",
              "product_variant": ` + wmsVariantSample + `,
              "qty": 500, "planned_qty": 500,
              "from_location": { "id": "c18bfe41-6ad5-48d8-a59b-25945353dcec",
                                 "code": "RECEIVING", "name": "Receiving" },
              "to_location": { "id": "b7238b1b-ba1f-410a-96e7-8e5bac386dfa",
                               "code": "A-1-1", "name": "Bin A-1-1" } } ],
          "status_history": [
            { "status": "DRAFT",
              "created_at": "2026-10-05T02:21:25.097861+00:00",
              "created_by_name": "Tram Nguyen" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	getNote: `  A line's qty is what actually moved, and it is null while the transfer is a
  DRAFT: a draft holds only planned_qty. qty takes that value when the
  transfer starts moving, and planned_qty is frozen from then on.`,

	body: `          { "notes": "Restock the pick face",
            "lines": [
              { "sku": "SKU-A1", "qty": 20,
                "from_location_code": "RECEIVING",
                "to_location_code": "A-1-1" } ] }`,
	bodyNote: `      lines is required and must not be empty.
` + wmsLineVariantNote + `
      from_location_code and to_location_code are location codes of this
      warehouse, as wms locations list prints them.
      external_ref, notes, extra_data, rel_type and rel_id are optional.`,
	updateNote: `  lines, when given, replaces the whole list. A transfer that is no longer
  editable exits 8 with code E9559.`,
	previewOut: `  How the body was understood is at .data: the status the transfer would be
  created in, the totals, and each line with the variant it resolved to.

      { "data": { "status": "DRAFT", "line_count": 1, "total_qty": 20,
                  "lines": [
                    { "product_variant_id": "b6121ce2-9e47-4a76-9f75-41ee6a3bd629",
                      "sku": "SKU-A1", "qty": 20,
                      "from_location_code": "RECEIVING",
                      "to_location_code": "A-1-1" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  A body that would be refused exits with the reason. Nothing is at .data
  then.`,
	validateOut: `  The verdict is at .data.valid. Read it: the command exits 0 for a body that
  is not valid, because the check itself succeeded.

      { "data": { "request_id": "975e2b39-edc5-47de-8d37-65617854c084",
                  "valid": true,
                  "data": { "line_count": 1, "total_qty": 20 } },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  When valid is false, .data.errors[] says why, and lineIndex is the 0-based
  position of the line at fault:

      { "data": { "request_id": "6e63ea0d-79a8-4315-b990-edac8f989745",
                  "valid": false,
                  "errors": [ { "code": "LOCATION_OUTSIDE_WAREHOUSE",
                                "message": "Line references a location that does not exist in the acting warehouse.",
                                "lineIndex": 0 } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

` + wmsDryRunRefusalNote,

	submit:        false,
	cancelReason:  true,
	cancelIfMatch: false,
	cancelNote: `  End a transfer that will not be carried out. A DRAFT or a MOVING transfer
  becomes CANCELLED, which is final; any other status exits 8 with code E9559.
  This cancel takes no concurrency token.`,
}

var wmsWarehouseTransferDocument = wmsDocument{
	use:      "warehouse-transfers",
	resource: api.WMSWarehouseTransfers,
	noun:     "warehouse transfer",
	plural:   "warehouse transfers",
	code:     "WTW-WHHN-24083",
	short:    "Plan and read transfers between two warehouses",
	about: `Warehouse transfers: stock sent from one warehouse to another.

The warehouse given as --warehouse is the one that starts the transfer. On a
PUSH, the default, it is the source and the body names the destination. On a
PULL it is the destination, asking a source warehouse for stock; a PULL never
learns how much the source holds.

A transfer is created as a DRAFT and submit sends it for approval. Approving,
rejecting, shipping and receiving happen in the Capigo app. Once approved, the
transfer generates an outbound shipment at the source and an inbound receipt
at the destination; those two documents belong to the transfer and are
cancelled only through it.

A transfer is listed from both of its warehouses.`,

	search: "Search on code.",
	sortBy: "created_at or code",
	listRow: `          { "id": "6542244f-e452-4b9d-badc-4dbc7541cfb8",
            "code": "{code}", "status": "APPROVED",
            "source_warehouse": ` + wmsWarehouseSample + `,
            "destination_warehouse": { "id": "caf5017a-bb7c-46be-abc0-50103e7c3291",
                                       "code": "WH-HCM",
                                       "name": "Kho Ho Chi Minh" },
            "created_at": "2026-10-12T08:38:44.11316+00:00",
            "exported_at": null,
            "destination_receipt_code": "IB-WHHCM-52429",
            "hours_in_transit": null, "is_abnormal": false,
            "abnormal_resolved": false }`,
	listNote: `
  A row carries no updated_at. Read the transfer with wms warehouse-transfers
  get for the value its writes need.
`,
	getSample: `      { "data": {
          "id": "6542244f-e452-4b9d-badc-4dbc7541cfb8", "code": "{code}",
          "status": "APPROVED", "direction": "PUSH",
          "transfer_ref": "25efe848-d6d6-424f-82a6-a3344e52ff14",
          "external_ref": null, "notes": null,
          "source_warehouse": ` + wmsWarehouseSample + `,
          "destination_warehouse": { "id": "caf5017a-bb7c-46be-abc0-50103e7c3291",
                                     "code": "WH-HCM",
                                     "name": "Kho Ho Chi Minh" },
          "created_at": "2026-10-12T08:38:44.11316+00:00",
          "submitted_at": "2026-10-12T08:38:51.329879+00:00",
          "approved_at": "2026-10-12T08:40:05.752079+00:00",
          "completed_at": null, "rejected_at": null,
          "rejection_reason": null, "cancelled_at": null,
          "cancellation_reason": null,
          "updated_at": "2026-10-12T08:40:05.752079Z", "exported_at": null,
          "hours_in_transit": null, "is_abnormal": null,
          "abnormal_resolved": null,
          "generated_outbound_shipment": {
            "id": "3c41667a-d190-49d3-ab30-e2feb00ab72f",
            "code": "OB-WHHN-37882" },
          "generated_inbound_receipt": {
            "id": "b89c4dae-d3dd-43b4-8dab-abd99825e870",
            "code": "IB-WHHCM-52429" },
          "lines": [
            { "id": "c0fe3db3-a4da-498e-9a29-508e1a3bd141",
              "product_variant": ` + wmsVariantSample + `,
              "qty_requested": 10, "qty_exported": null,
              "qty_received": null, "is_unplanned": false } ],
          "timeline": [
            { "source": "coordination", "status": "APPROVED",
              "created_at": "2026-10-12T08:40:05.752079+00:00",
              "created_by_name": "Tram Nguyen", "notes": null } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }`,
	getNote: `  generated_outbound_shipment and generated_inbound_receipt are null until
  the transfer has generated them. The shipment lives in the source warehouse and
  the receipt in the destination: read each with its own command and that
  warehouse's code.

  On a line, qty_exported is null until the shipment is EXPORTED and
  qty_received is null until the receipt is VERIFIED — null means "not yet",
  not 0. is_unplanned true marks a variant that arrived without being
  requested.

  timeline[] merges the transfer's own history with its shipment's and its
  receipt's; source says which of the three an entry came from.`,

	body: `          { "destination_warehouse_code": "WH-HCM", "notes": "Weekly restock",
            "lines": [ { "sku": "SKU-A1", "qty_requested": 10 } ] }`,
	bodyNote: `      lines is required and must not be empty.
` + wmsLineVariantNote + `
      direction is PUSH or PULL; left out, it is PUSH. A PUSH requires
      destination_warehouse_code. A PULL requires source_warehouse_code
      instead, the warehouse being asked for stock. Either must differ from
      the acting warehouse.
      external_ref, notes, extra_data, rel_type and rel_id are optional.`,
	updateNote: `  lines, when given, replaces the whole list. A transfer that is no longer
  editable exits 8.`,
	previewOut: `  The answer is at .data: valid, the checks behind it, and under preview how
  the transfer would be created.

      { "data": {
          "kind": "preview", "valid": true,
          "checks": [
            { "check": "destination_warehouse", "ok": true,
              "detail": "Destination warehouse \"WH-HCM\" resolves in this tenant" },
            { "check": "lines", "ok": true,
              "detail": "1 line(s) supplied with positive finite quantities" } ],
          "preview": { "direction": "PUSH", "source_warehouse_code": "WH-HN",
                       "destination_warehouse_code": "WH-HCM",
                       "status_after_create": "DRAFT", "line_count": 1,
                       "lines": [ { "index": 0, "product_variant_id": null,
                                    "sku": "SKU-A1", "qty_requested": 10 } ] } },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  The preview does not look a line's SKU up: it echoes the line as given, and
  an unknown SKU still previews as valid. Only create resolves the variant.

  A body the API refuses outright — no destination on a PUSH, the acting
  warehouse named as its own destination — exits 5 with the reason.`,
	validateOut: `  The verdict is at .data.valid, with the checks behind it at .data.checks[]:

      { "data": {
          "kind": "validate", "valid": true,
          "checks": [
            { "check": "acting_warehouse_scope", "ok": true,
              "detail": "Acting warehouse \"WH-HN\" holds scope.warehouse_transfer" },
            { "check": "body_contract", "ok": true,
              "detail": "Request body conforms to the create contract (direction: PUSH)" },
            { "check": "destination_warehouse", "ok": true,
              "detail": "Destination warehouse \"WH-HCM\" resolves in this tenant" },
            { "check": "source_destination_distinct", "ok": true,
              "detail": "Source and destination warehouses differ" },
            { "check": "lines", "ok": true,
              "detail": "1 line(s) supplied with positive finite quantities" } ] },
        "meta": { "tenant": "acme", "tenant_source": "flag" } }

  valid true does not mean every SKU exists: this check covers the
  warehouses, the direction and the quantities, and does not look a line's
  variant up. A line naming an unknown SKU validates, and is refused only by
  create.

  A body the API refuses outright — no destination on a PUSH, the acting
  warehouse named as its own destination — exits 5 with the reason.`,

	submit: true,
	submitNote: `  Send a DRAFT transfer for approval. Whether it is then approved or rejected
  is decided in the Capigo app, not here.`,
	cancelReason:  true,
	cancelIfMatch: true,
	cancelNote: `  End a transfer. It becomes CANCELLED, which is final. A status that no
  longer allows a cancel exits 8 with code E9579.`,
}
