package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// setFlags sets a built command's flags for one test and puts them back after:
// the wms commands hold their flag state inside the command, and a value left
// behind would leak into the next test that runs it.
func setFlags(t *testing.T, c *cobra.Command, pairs ...string) {
	t.Helper()
	for i := 0; i+1 < len(pairs); i += 2 {
		name, value := pairs[i], pairs[i+1]
		flag := c.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("%s has no --%s flag", c.CommandPath(), name)
		}
		if err := c.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for i := 0; i+1 < len(pairs); i += 2 {
			flag := c.Flags().Lookup(pairs[i])
			if replacer, ok := flag.Value.(interface{ Replace([]string) error }); ok {
				_ = replacer.Replace(nil)
			} else {
				_ = flag.Value.Set(flag.DefValue)
			}
			flag.Changed = false
		}
	})
}

func bodyFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write body: %v", err)
	}
	return path
}

func printedData(t *testing.T, printed string) (data map[string]any, meta map[string]any) {
	t.Helper()
	var doc struct {
		Data map[string]any `json:"data"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal([]byte(printed), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, printed)
	}
	return doc.Data, doc.Meta
}

// Every family is the same eight commands built from one description, so one
// family proves the mechanics and the table below proves each family is wired
// to its own resource.
func TestWMSDocumentCommandsCallTheirOwnResource(t *testing.T) {
	for _, doc := range []struct {
		resource string
		cmds     wmsDocumentCommands
	}{
		{"inbound-receipts", wmsInbound},
		{"outbound-shipments", wmsOutbound},
		{"internal-transfers", wmsInternalTransfers},
		{"warehouse-transfers", wmsWarehouseTransfers},
	} {
		setFlags(t, doc.cmds.get, "tenant", "acme", "warehouse", "WH-HN")
		seen, _ := callAgainst(t, `{"data":{"code":"X-1"}}`, func() { _ = doc.cmds.get.RunE(doc.cmds.get, []string{"X-1"}) })

		want := "/wms/" + doc.resource + "/X-1"
		if seen.method != "GET" || seen.path != want || seen.query != "warehouse_code=WH-HN" || seen.tenant != "acme" {
			t.Errorf("%s get: call = %s %s?%s tenant=%q, want GET %s?warehouse_code=WH-HN",
				doc.resource, seen.method, seen.path, seen.query, seen.tenant, want)
		}
	}
}

func TestWMSListSendsItsFilters(t *testing.T) {
	c := wmsInbound.list
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "status", "draft,planned", "query", "PO", "limit", "50")

	seen, printed := callAgainst(t, `{"data":[{"code":"IB-1"}],"meta":{"page":1,"limit":50,"total":1,"has_more":false}}`,
		func() { _ = c.RunE(c, nil) })

	for _, want := range []string{
		"warehouse_code=WH-HN", "q=PO", "limit=50",
		"filters%5Bstatus%5D%5B%24in%5D=DRAFT%2CPLANNED",
	} {
		if !containsParam(seen.query, want) {
			t.Errorf("query = %q, want it to carry %s", seen.query, want)
		}
	}
	if !strings.Contains(printed, `"total": 1`) || !strings.Contains(printed, `"tenant": "acme"`) {
		t.Errorf("the page total and the tenant must be on stdout:\n%s", printed)
	}
}

func TestWMSCreateSendsTheFileAndTheKey(t *testing.T) {
	c := wmsInbound.create
	body := `{"lines":[{"sku":"SKU-A1","qty_expected":5}]}`
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "from-json", bodyFile(t, body), "idempotency-key", "k1")

	seen, printed := callAgainstStatus(t, http.StatusCreated, `{"data":{"code":"IB-1","status":"DRAFT"}}`,
		func() { _ = c.RunE(c, nil) })

	if seen.method != "POST" || seen.path != "/wms/inbound-receipts" || seen.query != "warehouse_code=WH-HN" {
		t.Errorf("call = %s %s?%s", seen.method, seen.path, seen.query)
	}
	if seen.rawBody != body {
		t.Errorf("body = %s, want the file sent byte for byte", seen.rawBody)
	}
	if seen.idemKey != "k1" {
		t.Errorf("Idempotency-Key = %q, want k1", seen.idemKey)
	}
	data, meta := printedData(t, printed)
	if data["code"] != "IB-1" {
		t.Errorf("data = %v, want the created document", data)
	}
	if _, replayed := meta["replayed"]; replayed {
		t.Error("a 201 is a document created, not a replay")
	}
}

// 200 on a create is the server saying it created nothing. Only the status
// code says so, and the caller never sees the status code.
func TestWMSCreateSaysWhenItWasAReplay(t *testing.T) {
	c := wmsOutbound.create
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "from-json", bodyFile(t, `{"lines":[]}`), "idempotency-key", "k1")

	_, printed := callAgainstStatus(t, http.StatusOK, `{"data":{"code":"OB-1"}}`, func() { _ = c.RunE(c, nil) })

	if _, meta := printedData(t, printed); meta["replayed"] != true {
		t.Errorf("meta.replayed must be true for a replayed create:\n%s", printed)
	}
}

// A validate that refuses a body answers 200 with the verdict beside — not
// inside — data. Printing the envelope's data alone prints null, which reads as
// "nothing wrong".
func TestWMSValidatePrintsARefusal(t *testing.T) {
	c := wmsInbound.validate
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "from-json", bodyFile(t, `{"lines":[{"sku":"NOPE","qty_expected":1}]}`))

	seen, printed := callAgainst(t,
		`{"request_id":"r1","valid":false,"errors":[{"code":"VARIANT_NOT_FOUND","message":"no such variant"}]}`,
		func() { _ = c.RunE(c, nil) })

	if seen.path != "/wms/inbound-receipts/validate" {
		t.Errorf("path = %s", seen.path)
	}
	data, _ := printedData(t, printed)
	if data["valid"] != false {
		t.Fatalf(".data.valid must be false on stdout:\n%s", printed)
	}
	if errs, _ := data["errors"].([]any); len(errs) != 1 {
		t.Errorf(".data.errors must carry the reason:\n%s", printed)
	}
}

func TestWMSUpdateSendsIfMatch(t *testing.T) {
	c := wmsInbound.update
	const token = "2026-10-12T08:40:05.752079+00:00"
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "if-match", token, "from-json", bodyFile(t, `{"notes":null}`))

	seen, _ := callAgainst(t, `{"data":{"code":"IB-1"}}`, func() { _ = c.RunE(c, []string{"IB-1"}) })

	if seen.method != "PATCH" || seen.path != "/wms/inbound-receipts/IB-1" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	if seen.ifMatch != token {
		t.Errorf("If-Match = %q, want the token untouched", seen.ifMatch)
	}
	if seen.rawBody != `{"notes":null}` {
		t.Errorf("body = %s; a null must reach the API as a null, it is how a field is cleared", seen.rawBody)
	}
}

func TestWMSLifecycleActions(t *testing.T) {
	t.Run("submit sends the token and no body", func(t *testing.T) {
		c := wmsWarehouseTransfers.submit
		setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "if-match", "t1")
		seen, _ := callAgainst(t, `{"data":{"status":"PENDING_APPROVAL"}}`, func() { _ = c.RunE(c, []string{"WTW-1"}) })

		if seen.path != "/wms/warehouse-transfers/WTW-1/actions/submit" || seen.ifMatch != "t1" || seen.hasBody {
			t.Errorf("call = %s If-Match=%q hasBody=%v", seen.path, seen.ifMatch, seen.hasBody)
		}
	})

	t.Run("cancel sends its reason", func(t *testing.T) {
		c := wmsInbound.cancel
		setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "if-match", "t1", "reason", "  supplier postponed the delivery ")
		seen, _ := callAgainst(t, `{"data":{"status":"CANCELLED"}}`, func() { _ = c.RunE(c, []string{"IB-1"}) })

		if seen.path != "/wms/inbound-receipts/IB-1/actions/cancel" || seen.ifMatch != "t1" {
			t.Errorf("call = %s If-Match=%q", seen.path, seen.ifMatch)
		}
		if seen.body["reason"] != "supplier postponed the delivery" {
			t.Errorf("body = %v, want the trimmed reason", seen.body)
		}
	})
}

// The API differs per document in exactly these three ways, and each is a flag
// that must exist where the API reads it and must not where it does not: a
// flag the API ignores is a promise the command cannot keep.
func TestWMSActionsMatchEachDocument(t *testing.T) {
	if wmsInternalTransfers.submit != nil {
		t.Error("an internal transfer has no submit action; the API answers 404")
	}
	if wmsInternalTransfers.cancel.Flags().Lookup("if-match") != nil {
		t.Error("an internal transfer's cancel takes no If-Match")
	}
	if wmsOutbound.cancel.Flags().Lookup("reason") != nil {
		t.Error("an outbound shipment's cancel sends no body, so it takes no --reason")
	}
	for name, c := range map[string]*cobra.Command{
		"inbound": wmsInbound.cancel, "warehouse-transfers": wmsWarehouseTransfers.cancel,
	} {
		if c.Flags().Lookup("reason") == nil || c.Flags().Lookup("if-match") == nil {
			t.Errorf("wms %s cancel needs both --reason and --if-match", name)
		}
	}
	if wmsOutbound.get.Flags().Lookup("cost-method") == nil {
		t.Error("wms outbound get must expose --cost-method")
	}
	if wmsInbound.get.Flags().Lookup("cost-method") != nil {
		t.Error("only an outbound shipment reads cost_method")
	}
}

func TestWMSAvailabilityCheckBuildsItsBody(t *testing.T) {
	c := wmsAvailabilityCheckCmd
	setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "warehouse", "WH-HCM",
		"line", "sku=SKU-A1,demand=5", "line", "barcode=893,demand=2")

	seen, printed := callAgainst(t, `{"data":[{"line_no":1,"found":true,"warehouses":[]}]}`, func() { _ = c.RunE(c, nil) })

	if seen.method != "POST" || seen.path != "/wms/availability-checks" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	want := `{"lines":[{"demand":5,"sku":"SKU-A1"},{"barcode":"893","demand":2}],"warehouses":["WH-HN","WH-HCM"]}`
	if seen.rawBody != want {
		t.Errorf("body = %s\nwant   %s", seen.rawBody, want)
	}
	if !strings.Contains(printed, `"line_no": 1`) {
		t.Errorf("the rows must be on stdout:\n%s", printed)
	}
}
