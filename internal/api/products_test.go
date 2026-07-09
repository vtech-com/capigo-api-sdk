package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// productFixture returns a minimal valid PublicProductResponse JSON payload.
func productFixture(id, name, status string) []byte {
	b, _ := json.Marshal(map[string]any{
		"data": map[string]any{
			"id":         id,
			"name":       name,
			"slug":       "test-slug",
			"status":     status,
			"currency":   "VND",
			"aliases":    []string{},
			"is_deleted": false,
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
			"options":    []any{},
			"variants":   []any{},
		},
	})
	return b
}

// productListFixture returns a paginated product list JSON payload.
func productListFixture(products []map[string]any, hasMore bool, total int) []byte {
	b, _ := json.Marshal(map[string]any{
		"data": products,
		"meta": map[string]any{
			"page":     1,
			"limit":    20,
			"total":    total,
			"has_more": hasMore,
		},
	})
	return b
}

// TestProductsList_TenantHeaderSent verifies GET /pcms/products sends X-Tenant-Code.
func TestProductsList_TenantHeaderSent(t *testing.T) {
	var capturedTenant string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = r.Header.Get("X-Tenant-Code")
		if r.URL.Path != "/pcms/products" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server-Time", "2026-05-01T12:00:00Z")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productListFixture(nil, false, 0))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	resp, err := c.Do(context.Background(), "GET", "/pcms/products", nil, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if capturedTenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want %q", capturedTenant, "acme")
	}

	// Verify X-Server-Time is captured in Response.
	if resp.ServerTime != "2026-05-01T12:00:00Z" {
		t.Errorf("ServerTime = %q, want 2026-05-01T12:00:00Z", resp.ServerTime)
	}
}

// TestProductsList_ServerTimeEmpty verifies ServerTime is empty when header absent.
func TestProductsList_ServerTimeEmpty(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// intentionally do NOT set X-Server-Time
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productListFixture(nil, false, 0))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	resp, err := c.Do(context.Background(), "GET", "/pcms/products", nil, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if resp.ServerTime != "" {
		t.Errorf("ServerTime = %q, want empty string", resp.ServerTime)
	}
}

// TestProductsList_UpdatedSinceQuery verifies the updated_since query param is forwarded.
func TestProductsList_UpdatedSinceQuery(t *testing.T) {
	var capturedQuery string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productListFixture(nil, false, 0))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	_, err := c.Do(context.Background(), "GET",
		"/pcms/products?updated_since=2026-01-01T00%3A00%3A00Z",
		nil, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if !strings.Contains(capturedQuery, "updated_since=") {
		t.Errorf("expected updated_since= in query string, got %q", capturedQuery)
	}
}

// TestProductsCreate_201 verifies POST /pcms/products returns a product on 201.
func TestProductsCreate_201(t *testing.T) {
	productID := "550e8400-e29b-41d4-a716-446655440001"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/pcms/products" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body CreateProductRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body.Name == "" {
			t.Error("expected non-empty name in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(productFixture(productID, body.Name, "DRAFT"))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	req := CreateProductRequest{Name: "Test Product"}
	resp, err := c.Do(context.Background(), "POST", "/pcms/products", req, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want 201", resp.StatusCode)
	}

	// The client does not model a product. It hands the API's bytes on, and the
	// caller sees every field the API sent — including ones this SDK has never
	// heard of.
	var envelope RawEnvelope
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, want := range []string{productID, `"DRAFT"`} {
		if !strings.Contains(string(envelope.Data), want) {
			t.Errorf("product body %s does not carry %s", envelope.Data, want)
		}
	}
}

// TestProductsCreate_RequiresTenant verifies 403 maps to exit code 3.
func TestProductsCreate_TenantDenied(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"E9103","message":"tenant denied"}}`))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	_, err := c.Do(context.Background(), "POST", "/pcms/products",
		CreateProductRequest{Name: "X"}, &tenant)
	if err == nil {
		t.Fatal("expected error")
	}
	if code := ExitCodeFor(err); code != 3 {
		t.Errorf("ExitCodeFor = %d, want 3 (permission error)", code)
	}
}

// TestProductsUpdate_200 verifies PUT /pcms/products/{id} returns updated product.
func TestProductsUpdate_200(t *testing.T) {
	productID := "550e8400-e29b-41d4-a716-446655440002"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		expectedPath := "/pcms/products/" + productID
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productFixture(productID, "Updated Name", "ACTIVE"))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	newName := "Updated Name"
	req := UpdateProductRequest{Name: &newName}
	resp, err := c.Do(context.Background(), "PUT", "/pcms/products/"+productID, req, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

// TestProductsUpdate_NotFound verifies 404 maps to exit code 4.
func TestProductsUpdate_NotFound(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"E9417","message":"product not found"}}`))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	newName := "New Name"
	_, err := c.Do(context.Background(), "PUT",
		"/pcms/products/nonexistent-id",
		UpdateProductRequest{Name: &newName}, &tenant)
	if err == nil {
		t.Fatal("expected error")
	}
	if code := ExitCodeFor(err); code != 4 {
		t.Errorf("ExitCodeFor = %d, want 4 (not found)", code)
	}
}

// TestProductsVariants_200 verifies PUT /pcms/products/{id}/variants returns product.
func TestProductsVariants_200(t *testing.T) {
	productID := "550e8400-e29b-41d4-a716-446655440003"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}
		expectedPath := "/pcms/products/" + productID + "/variants"
		if r.URL.Path != expectedPath {
			t.Errorf("path = %q, want %q", r.URL.Path, expectedPath)
		}

		// Decode the array body.
		var items []UpsertVariantItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if len(items) == 0 {
			t.Error("expected at least one variant item")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productFixture(productID, "My Product", "ACTIVE"))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)

	variantName := "Blue / L"
	items := []UpsertVariantItem{
		{Name: &variantName},
	}
	resp, err := c.Do(context.Background(), "PUT",
		"/pcms/products/"+productID+"/variants", items, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

// TestProductsVariants_UpsertSemantics verifies variant_id is optional (CREATE vs UPDATE).
func TestProductsVariants_UpsertSemantics(t *testing.T) {
	productID := "550e8400-e29b-41d4-a716-446655440004"
	existingID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []UpsertVariantItem
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			t.Errorf("decode body: %v", err)
		}

		// First item: UPDATE (has variant_id, no name)
		if items[0].VariantID == nil || *items[0].VariantID != existingID {
			t.Errorf("items[0].VariantID = %v, want %q", items[0].VariantID, existingID)
		}

		// Second item: CREATE (no variant_id, has name)
		if items[1].VariantID != nil {
			t.Errorf("items[1].VariantID should be nil for CREATE, got %v", items[1].VariantID)
		}
		if items[1].Name == nil || *items[1].Name == "" {
			t.Error("items[1].Name should be non-empty for CREATE")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productFixture(productID, "My Product", "ACTIVE"))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)

	variantName := "Red / M"
	price := 99000.0
	items := []UpsertVariantItem{
		{VariantID: &existingID, Price: &price}, // UPDATE
		{Name: &variantName},                    // CREATE
	}
	_, err := c.Do(context.Background(), "PUT",
		"/pcms/products/"+productID+"/variants", items, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
}

// TestUpsertVariantItem_ManufacturerLegacyExtraData verifies the write struct
// serializes manufacturer_code, legacy_code, and extra_data when set.
func TestUpsertVariantItem_ManufacturerLegacyExtraData(t *testing.T) {
	mfrCode := "MFR-123"
	legacy := "ERP-9"
	item := UpsertVariantItem{
		Name:             strPtr("Blue / L"),
		ManufacturerCode: &mfrCode,
		LegacyCode:       &legacy,
		ExtraData:        map[string]any{"warranty_months": float64(12)},
	}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if v, ok := m["manufacturer_code"]; !ok || v != mfrCode {
		t.Errorf("manufacturer_code = %v, want %q", v, mfrCode)
	}
	if v, ok := m["legacy_code"]; !ok || v != legacy {
		t.Errorf("legacy_code = %v, want %q", v, legacy)
	}
	extra, ok := m["extra_data"].(map[string]any)
	if !ok || extra["warranty_months"] != float64(12) {
		t.Errorf("extra_data = %v, want map with warranty_months=12", m["extra_data"])
	}
}

// TestProductsVariants_FromJSONRawPassthroughPreservesUnknownFields is a
// regression test for a silent data-loss bug: cmd/products.go used to decode
// --from-json into []UpsertVariantItem and re-marshal that struct as the
// request body, which dropped any field the struct didn't declare (e.g.
// manufacturer_code/legacy_code/extra_data before those fields were added,
// or any future field the API adds ahead of the SDK). The fix sends the raw
// bytes untouched — this test pins that behavior at the transport layer by
// sending json.RawMessage the same way cmd/products.go now does.
func TestProductsVariants_FromJSONRawPassthroughPreservesUnknownFields(t *testing.T) {
	productID := "550e8400-e29b-41d4-a716-446655440005"
	rawInput := json.RawMessage(`[{"name":"Blue / L","manufacturer_code":"MFR-1","legacy_code":"ERP-1","extra_data":{"warranty_months":12},"some_future_field":"xyz"}]`)

	var receivedBody map[string]any
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		receivedBody = items[0]

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(productFixture(productID, "My Product", "ACTIVE"))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)

	_, err := c.Do(context.Background(), "PUT",
		"/pcms/products/"+productID+"/variants", rawInput, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	for field, want := range map[string]any{
		"manufacturer_code": "MFR-1",
		"legacy_code":       "ERP-1",
		"some_future_field": "xyz",
	} {
		if got := receivedBody[field]; got != want {
			t.Errorf("received body[%q] = %v, want %v (field was dropped)", field, got, want)
		}
	}
	extra, ok := receivedBody["extra_data"].(map[string]any)
	if !ok || extra["warranty_months"] != float64(12) {
		t.Errorf("received body[extra_data] = %v, want map with warranty_months=12", receivedBody["extra_data"])
	}
}

// TestUpsertVariantItem_OptionalVariantID verifies that UpsertVariantItem
// with a nil VariantID omits the field from JSON (CREATE semantics).
func TestUpsertVariantItem_OptionalVariantID(t *testing.T) {
	name := "Default"
	item := UpsertVariantItem{Name: &name}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := m["variant_id"]; ok {
		t.Error("variant_id should be omitted when nil, but was present in JSON")
	}
	if v, ok := m["name"]; !ok || v != "Default" {
		t.Errorf("name = %v, want \"Default\"", v)
	}
}

// TestCreateProductRequest_OptionsAndVariants verifies options+variants JSON round-trip.
func TestCreateProductRequest_OptionsAndVariants(t *testing.T) {
	opt1val := "Red"
	req := CreateProductRequest{
		Name: "T-Shirt",
		Options: []CreateProductOptionItem{
			{Name: "Color", Values: []string{"Red", "Blue"}},
		},
		Variants: []CreateProductVariantItem{
			{Name: "Red", Option1: &opt1val},
		},
	}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := m["options"]; !ok {
		t.Error("options missing from JSON")
	}
	if _, ok := m["variants"]; !ok {
		t.Error("variants missing from JSON")
	}
}

// TestUpdateProductRequest_OmitsUnsetFields ensures only provided fields appear in JSON.
func TestUpdateProductRequest_OmitsUnsetFields(t *testing.T) {
	newName := "Updated"
	req := UpdateProductRequest{Name: &newName}

	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Only "name" should be present; other optional fields should be omitted.
	if len(m) != 1 {
		t.Errorf("expected 1 field in JSON, got %d: %v", len(m), m)
	}
	if m["name"] != "Updated" {
		t.Errorf("name = %v, want Updated", m["name"])
	}
}

// TestProductRequests_TagsSerialization ensures tags reach the wire when set
// (create + update) and are omitted when unset, matching the aliases behavior.
func TestProductRequests_TagsSerialization(t *testing.T) {
	create, err := json.Marshal(CreateProductRequest{Name: "P", Tags: []string{"organic", "premium"}})
	if err != nil {
		t.Fatalf("marshal create: %v", err)
	}
	if !strings.Contains(string(create), `"tags":["organic","premium"]`) {
		t.Errorf("create body missing tags: %s", create)
	}

	update, err := json.Marshal(UpdateProductRequest{Tags: []string{"sale"}})
	if err != nil {
		t.Fatalf("marshal update: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(update, &m); err != nil {
		t.Fatalf("unmarshal update: %v", err)
	}
	if len(m) != 1 || m["tags"] == nil {
		t.Errorf("update body should carry only tags, got %v", m)
	}

	// Unset tags must not appear (omitempty).
	bare, _ := json.Marshal(CreateProductRequest{Name: "P"})
	if strings.Contains(string(bare), "tags") {
		t.Errorf("unset tags must be omitted: %s", bare)
	}
}
