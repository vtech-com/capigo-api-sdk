package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// The WMS resources, named as the API's paths name them. A warehouse is
// tenant-scoped; everything else lives inside one warehouse and is read and
// written with that warehouse's code beside the tenant.
const (
	WMSWarehouses         = "warehouses"
	WMSLocations          = "locations"
	WMSInboundReceipts    = "inbound-receipts"
	WMSOutboundShipments  = "outbound-shipments"
	WMSInternalTransfers  = "internal-transfers"
	WMSWarehouseTransfers = "warehouse-transfers"
)

// MaxWMSLimit is the largest page a WMS list serves. The PCMS lists serve 100;
// these refuse anything over 50 with INVALID_LIMIT.
const MaxWMSLimit = 50

// wmsStatuses are the statuses each document's list accepts as a filter, in
// the order the document moves through them. The server refuses any other
// value with INVALID_QUERY_PARAMS and a message that does not list the valid
// ones, so the CLI checks first and can say what they are.
var wmsStatuses = map[string][]string{
	WMSInboundReceipts:    {"DRAFT", "PLANNED", "RECEIVING", "VERIFIED", "CANCELLED"},
	WMSOutboundShipments:  {"DRAFT", "PLANNED", "AWAITING_EXPORT", "EXPORTED", "CANCELLED"},
	WMSInternalTransfers:  {"DRAFT", "MOVING", "MOVED", "CANCELLED"},
	WMSWarehouseTransfers: {"DRAFT", "PENDING_APPROVAL", "APPROVED", "IN_TRANSIT", "COMPLETED", "REJECTED", "CANCELLED"},
}

// WMSStatuses returns the statuses a document's list filters by, or nil for a
// resource that has no status.
func WMSStatuses(resource string) []string {
	return wmsStatuses[resource]
}

// NormalizeWMSStatuses turns what a caller typed after --status into the
// values the API matches: each entry may hold several statuses separated by
// commas, case is forgiven, and a repeat is dropped. A status the document
// does not have is an error naming the ones it does.
func NormalizeWMSStatuses(resource string, given []string) ([]string, error) {
	valid := wmsStatuses[resource]
	seen := map[string]bool{}
	var out []string
	for _, entry := range given {
		for _, raw := range strings.Split(entry, ",") {
			status := strings.ToUpper(strings.TrimSpace(raw))
			if status == "" || seen[status] {
				continue
			}
			known := false
			for _, v := range valid {
				if v == status {
					known = true
					break
				}
			}
			if !known {
				return nil, fmt.Errorf("unknown status %q; one of %s", strings.TrimSpace(raw), strings.Join(valid, ", "))
			}
			seen[status] = true
			out = append(out, status)
		}
	}
	return out, nil
}

// WMSListQuery is everything a WMS list can be narrowed by. A zero field sends
// no parameter.
type WMSListQuery struct {
	// Warehouse is the acting warehouse's code. Empty only for the warehouse
	// list itself, which is tenant-scoped.
	Warehouse string
	Query     string
	// Statuses filters a document list: one value is sent as an exact match,
	// several as a set.
	Statuses []string
	// IsActive is "true" or "false", for the warehouse and location lists.
	IsActive string
	Sort     string
	Page     int
	Limit    int
}

func (q WMSListQuery) values() url.Values {
	params := url.Values{}
	set := func(name, value string) {
		if value != "" {
			params.Set(name, value)
		}
	}
	set("warehouse_code", q.Warehouse)
	set("q", q.Query)
	set("filters[is_active][$eq]", q.IsActive)
	set("sort", q.Sort)
	switch len(q.Statuses) {
	case 0:
	case 1:
		params.Set("filters[status][$eq]", q.Statuses[0])
	default:
		params.Set("filters[status][$in]", strings.Join(q.Statuses, ","))
	}
	if q.Page > 0 {
		params.Set("page", strconv.Itoa(q.Page))
	}
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	return params
}

func wmsPath(resource string, segments []string, params url.Values) string {
	path := "/wms/" + resource
	for _, s := range segments {
		path += "/" + url.PathEscape(s)
	}
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return path
}

func warehouseParam(warehouse string) url.Values {
	params := url.Values{}
	if warehouse != "" {
		params.Set("warehouse_code", warehouse)
	}
	return params
}

// ListWMS fetches a page of one WMS resource.
func (c *Client) ListWMS(ctx context.Context, resource string, q WMSListQuery, tenant *string) (*Response, error) {
	return c.Do(ctx, "GET", wmsPath(resource, nil, q.values()), nil, tenant)
}

// GetWMS fetches one WMS record by its code. warehouse is empty for a
// warehouse itself. costMethod is only read by an outbound shipment.
func (c *Client) GetWMS(ctx context.Context, resource, code, warehouse, costMethod string, tenant *string) (*Response, error) {
	params := warehouseParam(warehouse)
	if costMethod != "" {
		params.Set("cost_method", costMethod)
	}
	return c.Do(ctx, "GET", wmsPath(resource, []string{code}, params), nil, tenant)
}

// The three things a create body can be sent to. Preview and validate run the
// create's own authorization and validation and write nothing.
const (
	WMSStageCreate   = ""
	WMSStagePreview  = "preview"
	WMSStageValidate = "validate"
)

// PostWMSDocument sends a create body to a document's create, preview or
// validate endpoint. idempotencyKey is only meaningful on a create.
func (c *Client) PostWMSDocument(ctx context.Context, resource, stage, warehouse string, body any, idempotencyKey string, tenant *string) (*Response, error) {
	var segments []string
	if stage != WMSStageCreate {
		segments = []string{stage}
	}
	var headers map[string]string
	if idempotencyKey != "" {
		headers = map[string]string{"Idempotency-Key": idempotencyKey}
	}
	return c.DoWithHeaders(ctx, "POST", wmsPath(resource, segments, warehouseParam(warehouse)), body, tenant, headers)
}

// UpdateWMSDocument merge-patches an editable document. ifMatch is the
// document's current updated_at, which the API requires as its concurrency
// token.
func (c *Client) UpdateWMSDocument(ctx context.Context, resource, code, warehouse, ifMatch string, body any, tenant *string) (*Response, error) {
	return c.DoWithHeaders(ctx, "PATCH", wmsPath(resource, []string{code}, warehouseParam(warehouse)), body, tenant,
		map[string]string{"If-Match": ifMatch})
}

// ActOnWMSDocument applies a lifecycle action — submit or cancel — to a
// document. ifMatch is empty for an internal transfer, whose cancel takes no
// concurrency token.
func (c *Client) ActOnWMSDocument(ctx context.Context, resource, code, action, warehouse, ifMatch string, body any, tenant *string) (*Response, error) {
	var headers map[string]string
	if ifMatch != "" {
		headers = map[string]string{"If-Match": ifMatch}
	}
	return c.DoWithHeaders(ctx, "POST", wmsPath(resource, []string{code, "actions", action}, warehouseParam(warehouse)), body, tenant, headers)
}

// CheckWMSAvailability asks how well each named warehouse covers each line.
// A read, though it travels as a POST.
func (c *Client) CheckWMSAvailability(ctx context.Context, body any, tenant *string) (*Response, error) {
	return c.Do(ctx, "POST", "/wms/availability-checks", body, tenant)
}

// availabilityLineKinds maps what a caller writes before the "=" of a --line
// to the body field that names a variant.
var availabilityLineKinds = map[string]string{
	"sku":        "sku",
	"barcode":    "barcode",
	"variant-id": "variant_id",
	"variant_id": "variant_id",
}

// ParseAvailabilityLine reads one line of an availability check, written
// `<sku|barcode|variant-id>=<value>,demand=<n>`.
//
// The demand is split off from the right, so a SKU that itself holds a comma
// or an equals sign still arrives whole.
func ParseAvailabilityLine(line string) (map[string]any, error) {
	const marker = ",demand="
	at := strings.LastIndex(line, marker)
	if at < 0 {
		return nil, fmt.Errorf("line %q has no demand; write it as sku=<sku>,demand=<n>", line)
	}
	kind, value, found := strings.Cut(line[:at], "=")
	field, known := availabilityLineKinds[strings.TrimSpace(kind)]
	value = strings.TrimSpace(value)
	if !found || !known || value == "" {
		return nil, fmt.Errorf("line %q must name one variant as sku=<sku>, barcode=<code> or variant-id=<uuid>", line)
	}
	demand, err := strconv.ParseFloat(strings.TrimSpace(line[at+len(marker):]), 64)
	if err != nil || demand <= 0 {
		return nil, fmt.Errorf("line %q needs a demand greater than 0", line)
	}
	return map[string]any{field: value, "demand": demand}, nil
}

// SplitAnswer returns what a command should print as data, and the API's meta.
//
// Almost every endpoint answers { "data": …, "meta": … } and data is the whole
// answer. The WMS dry runs do not: a validate answers { "valid": false,
// "errors": […] } with no data key at all, and a warehouse-transfer preview
// puts its verdict in valid and checks beside it. Printing only data there
// would print null over a refusal. So a body that carries anything beside data
// and meta is printed whole, and one that carries only those is unwrapped as
// usual.
func SplitAnswer(body []byte) (data, meta json.RawMessage) {
	body = bytes.TrimSpace(body)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return body, nil
	}
	for key := range fields {
		if key != "data" && key != "meta" {
			return body, fields["meta"]
		}
	}
	return fields["data"], fields["meta"]
}
