package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// TestWMSEndpoints pins the method, path, query and headers each WMS method
// sends. The path is built from a resource name and a code, and the two things
// that scope a call — the warehouse and the concurrency token — travel beside
// the body, so a mistake in any of them is a call to the wrong document.
func TestWMSEndpoints(t *testing.T) {
	tenant := "acme"
	body := map[string]any{"notes": "x"}

	tests := []struct {
		name        string
		call        func(c *Client) error
		wantMethod  string
		wantPath    string
		wantQuery   url.Values
		wantIfMatch string
		wantIdemKey string
		wantBody    bool
	}{
		{
			name: "list warehouses carries no warehouse",
			call: func(c *Client) error {
				_, err := c.ListWMS(context.Background(), WMSWarehouses, WMSListQuery{IsActive: "true", Sort: "code:asc"}, &tenant)
				return err
			},
			wantMethod: "GET", wantPath: "/wms/warehouses",
			wantQuery: url.Values{"filters[is_active][$eq]": {"true"}, "sort": {"code:asc"}},
		},
		{
			name: "one status is an exact match",
			call: func(c *Client) error {
				_, err := c.ListWMS(context.Background(), WMSInboundReceipts,
					WMSListQuery{Warehouse: "WH-HN", Query: "PO", Statuses: []string{"DRAFT"}, Page: 2, Limit: 50}, &tenant)
				return err
			},
			wantMethod: "GET", wantPath: "/wms/inbound-receipts",
			wantQuery: url.Values{
				"warehouse_code": {"WH-HN"}, "q": {"PO"}, "filters[status][$eq]": {"DRAFT"},
				"page": {"2"}, "limit": {"50"},
			},
		},
		{
			name: "several statuses are a set",
			call: func(c *Client) error {
				_, err := c.ListWMS(context.Background(), WMSOutboundShipments,
					WMSListQuery{Warehouse: "WH-HN", Statuses: []string{"DRAFT", "PLANNED"}}, &tenant)
				return err
			},
			wantMethod: "GET", wantPath: "/wms/outbound-shipments",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}, "filters[status][$in]": {"DRAFT,PLANNED"}},
		},
		{
			name: "get escapes the code",
			call: func(c *Client) error {
				_, err := c.GetWMS(context.Background(), WMSLocations, " P1/A", "WH-HN", "", &tenant)
				return err
			},
			wantMethod: "GET", wantPath: "/wms/locations/ P1/A",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}},
		},
		{
			name: "get shipment with a cost method",
			call: func(c *Client) error {
				_, err := c.GetWMS(context.Background(), WMSOutboundShipments, "OB-1", "WH-HN", "average", &tenant)
				return err
			},
			wantMethod: "GET", wantPath: "/wms/outbound-shipments/OB-1",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}, "cost_method": {"average"}},
		},
		{
			name: "create with a key",
			call: func(c *Client) error {
				_, err := c.PostWMSDocument(context.Background(), WMSInboundReceipts, WMSStageCreate, "WH-HN", body, "k1", &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/inbound-receipts",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}}, wantIdemKey: "k1", wantBody: true,
		},
		{
			name: "preview",
			call: func(c *Client) error {
				_, err := c.PostWMSDocument(context.Background(), WMSWarehouseTransfers, WMSStagePreview, "WH-HN", body, "", &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/warehouse-transfers/preview",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}}, wantBody: true,
		},
		{
			name: "validate",
			call: func(c *Client) error {
				_, err := c.PostWMSDocument(context.Background(), WMSInternalTransfers, WMSStageValidate, "WH-HN", body, "", &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/internal-transfers/validate",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}}, wantBody: true,
		},
		{
			name: "update sends If-Match",
			call: func(c *Client) error {
				_, err := c.UpdateWMSDocument(context.Background(), WMSInboundReceipts, "IB-1", "WH-HN",
					"2026-10-12T08:40:05.752079+00:00", body, &tenant)
				return err
			},
			wantMethod: "PATCH", wantPath: "/wms/inbound-receipts/IB-1",
			wantQuery:   url.Values{"warehouse_code": {"WH-HN"}},
			wantIfMatch: "2026-10-12T08:40:05.752079+00:00", wantBody: true,
		},
		{
			name: "submit sends If-Match and no body",
			call: func(c *Client) error {
				_, err := c.ActOnWMSDocument(context.Background(), WMSOutboundShipments, "OB-1", "submit", "WH-HN", "t1", nil, &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/outbound-shipments/OB-1/actions/submit",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}}, wantIfMatch: "t1",
		},
		{
			name: "an internal transfer's cancel sends no If-Match",
			call: func(c *Client) error {
				_, err := c.ActOnWMSDocument(context.Background(), WMSInternalTransfers, "TR-1", "cancel", "WH-HN", "",
					map[string]string{"reason": "planned in error"}, &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/internal-transfers/TR-1/actions/cancel",
			wantQuery: url.Values{"warehouse_code": {"WH-HN"}}, wantBody: true,
		},
		{
			name: "availability check",
			call: func(c *Client) error {
				_, err := c.CheckWMSAvailability(context.Background(), body, &tenant)
				return err
			},
			wantMethod: "POST", wantPath: "/wms/availability-checks", wantQuery: url.Values{}, wantBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotTenant, gotIfMatch, gotIdemKey string
			var gotQuery url.Values
			var gotBody bool

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.Query()
				gotTenant = r.Header.Get("X-Tenant-Code")
				gotIfMatch = r.Header.Get("If-Match")
				gotIdemKey = r.Header.Get("Idempotency-Key")
				raw, _ := io.ReadAll(r.Body)
				gotBody = len(raw) > 0
				_, _ = w.Write([]byte(`{"data":{}}`))
			}))
			defer srv.Close()

			if err := tt.call(newTestClient(t, srv)); err != nil {
				t.Fatalf("call: %v", err)
			}
			if gotMethod != tt.wantMethod || gotPath != tt.wantPath {
				t.Errorf("call = %s %s, want %s %s", gotMethod, gotPath, tt.wantMethod, tt.wantPath)
			}
			if !reflect.DeepEqual(gotQuery, tt.wantQuery) {
				t.Errorf("query = %v, want %v", gotQuery, tt.wantQuery)
			}
			if gotTenant != tenant {
				t.Errorf("X-Tenant-Code = %q, want %q", gotTenant, tenant)
			}
			if gotIfMatch != tt.wantIfMatch {
				t.Errorf("If-Match = %q, want %q", gotIfMatch, tt.wantIfMatch)
			}
			if gotIdemKey != tt.wantIdemKey {
				t.Errorf("Idempotency-Key = %q, want %q", gotIdemKey, tt.wantIdemKey)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body sent = %v, want %v", gotBody, tt.wantBody)
			}
		})
	}
}

func TestNormalizeWMSStatuses(t *testing.T) {
	got, err := NormalizeWMSStatuses(WMSInboundReceipts, []string{"draft, Verified", "DRAFT", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"DRAFT", "VERIFIED"}; !reflect.DeepEqual(got, want) {
		t.Errorf("statuses = %v, want %v", got, want)
	}

	// MOVED belongs to an internal transfer, not to a receipt: the server would
	// answer "Invalid inbound receipt status filter" and name no valid value.
	if _, err := NormalizeWMSStatuses(WMSInboundReceipts, []string{"MOVED"}); err == nil {
		t.Error("a status of another document was accepted")
	}
}

func TestParseAvailabilityLine(t *testing.T) {
	good := map[string]map[string]any{
		"sku=SKU-A1,demand=5":            {"sku": "SKU-A1", "demand": 5.0},
		"barcode=8930000000011,demand=2": {"barcode": "8930000000011", "demand": 2.0},
		"variant-id=8fc5,demand=0.5":     {"variant_id": "8fc5", "demand": 0.5},
		// The demand is split off from the right, so the SKU keeps its comma.
		"sku=A,B=C,demand=1": {"sku": "A,B=C", "demand": 1.0},
	}
	for line, want := range good {
		got, err := ParseAvailabilityLine(line)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", line, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q = %v, want %v", line, got, want)
		}
	}

	for _, line := range []string{"SKU-A1=5", "sku=SKU-A1", "sku=,demand=1", "name=x,demand=1", "sku=A,demand=0", "sku=A,demand=x"} {
		if _, err := ParseAvailabilityLine(line); err == nil {
			t.Errorf("%q was accepted", line)
		}
	}
}

// A validate that refuses a body answers 200 with no data key. Printing only
// data would print null over the refusal.
func TestSplitAnswer(t *testing.T) {
	cases := []struct {
		name, body, wantData, wantMeta string
	}{
		{"an envelope is unwrapped", `{"data":{"valid":true},"meta":{"page":1}}`, `{"valid":true}`, `{"page":1}`},
		{"a verdict beside data is kept whole",
			`{"request_id":"r1","valid":true,"data":{"line_count":1}}`,
			`{"request_id":"r1","valid":true,"data":{"line_count":1}}`, ``},
		{"a refusal with no data is kept whole",
			`{"valid":false,"errors":[{"code":"VARIANT_NOT_FOUND"}]}`,
			`{"valid":false,"errors":[{"code":"VARIANT_NOT_FOUND"}]}`, ``},
		{"an array is passed through", `[1,2]`, `[1,2]`, ``},
	}
	for _, c := range cases {
		data, meta := SplitAnswer([]byte(c.body))
		if string(data) != c.wantData || string(meta) != c.wantMeta {
			t.Errorf("%s: data=%s meta=%s, want data=%s meta=%s", c.name, data, meta, c.wantData, c.wantMeta)
		}
	}
}
