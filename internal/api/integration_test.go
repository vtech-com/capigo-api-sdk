package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// uaPattern matches: capigo-api-sdk/<version> (<os>; <arch>)
var uaPattern = regexp.MustCompile(`^capigo-api-sdk/.+ \(.+; .+\)$`)

// newIntegrationClient creates an api.Client pointed at the given TLS server.
// It swaps the internal HTTP transport so the self-signed cert is trusted.
func newIntegrationClient(t *testing.T, srv *httptest.Server, apiKey string) *Client {
	t.Helper()
	c, err := NewClient(srv.URL, apiKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.http = srv.Client()
	return c
}

// assertCommonHeaders checks headers that every request must carry.
func assertCommonHeaders(t *testing.T, r *http.Request, wantTenant *string) {
	t.Helper()

	// Authorization: Bearer <key>
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		t.Errorf("Authorization = %q; want Bearer ...", auth)
	}

	// User-Agent must match pattern.
	ua := r.Header.Get("User-Agent")
	if !uaPattern.MatchString(ua) {
		t.Errorf("User-Agent = %q; does not match %s", ua, uaPattern)
	}

	// X-Request-Id must be a valid UUID.
	rid := r.Header.Get("X-Request-Id")
	if _, err := uuid.Parse(rid); err != nil {
		t.Errorf("X-Request-Id = %q; not a valid UUID: %v", rid, err)
	}

	// X-Tenant-Code: present only when wantTenant is non-nil.
	gotTenant := r.Header.Get("X-Tenant-Code")
	if wantTenant == nil {
		if gotTenant != "" {
			t.Errorf("X-Tenant-Code should be absent, got %q", gotTenant)
		}
	} else {
		if gotTenant != *wantTenant {
			t.Errorf("X-Tenant-Code = %q; want %q", gotTenant, *wantTenant)
		}
	}
}

// --- GET /me ---

func TestIntegration_GetMe(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":           "usr_001",
				"display_name": "Alice",
				"email":        "alice@example.com",
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newIntegrationClient(t, srv, "csk_testkey")
	resp, err := c.Do(context.Background(), "GET", "/me", nil, nil)
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}

	// /me sends no X-Tenant-Code header.
	assertCommonHeaders(t, capturedReq, nil)
}

// --- GET /tenants ---

func TestIntegration_GetTenants_NoTenantHeader(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"tenant_code": "acme", "name": "ACME Corp", "role": "admin", "joined_at": "2024-01-01T00:00:00Z"},
			},
			"meta": map[string]any{"page": 1, "limit": 50, "total": 1, "has_more": false},
		})
	}))
	t.Cleanup(srv.Close)

	c := newIntegrationClient(t, srv, "csk_testkey")
	resp, err := c.Do(context.Background(), "GET", "/tenants", nil, nil)
	if err != nil {
		t.Fatalf("GET /tenants: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}

	// tenants list must NOT send X-Tenant-Code.
	assertCommonHeaders(t, capturedReq, nil)

	// Response body must decode into Envelope[[]Tenant].
	var envelope Envelope[[]Tenant]
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].TenantCode != "acme" {
		t.Errorf("unexpected tenants data: %v", envelope.Data)
	}
}

// --- GET /mission/boards ---

func TestIntegration_GetBoards_SendsTenantHeader(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "brd_001", "name": "Sprint Board", "is_public": true, "created_at": "2024-01-01T00:00:00Z"},
			},
			"meta": map[string]any{"page": 1, "limit": 50, "total": 1, "has_more": false},
		})
	}))
	t.Cleanup(srv.Close)

	tenant := "acme"
	c := newIntegrationClient(t, srv, "csk_testkey")
	resp, err := c.Do(context.Background(), "GET", "/mission/boards", nil, &tenant)
	if err != nil {
		t.Fatalf("GET /mission/boards: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}

	// boards list with resolved tenant must include X-Tenant-Code.
	assertCommonHeaders(t, capturedReq, strPtr("acme"))

	var envelope Envelope[[]Board]
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		t.Fatalf("decode boards: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].ID != "brd_001" {
		t.Errorf("unexpected boards data: %v", envelope.Data)
	}
}

// --- GET /mission/tasks ---

func TestIntegration_GetTasks_WithTenantHeader(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "tsk_001", "code": "T-1", "title": "Fix bug", "status": "open",
					"has_subtasks": false, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z",
				},
			},
			"meta": map[string]any{"page": 1, "limit": 50, "total": 1, "has_more": false},
		})
	}))
	t.Cleanup(srv.Close)

	tenant := "acme"
	c := newIntegrationClient(t, srv, "csk_testkey")
	resp, err := c.Do(context.Background(), "GET", "/mission/tasks", nil, &tenant)
	if err != nil {
		t.Fatalf("GET /mission/tasks: %v", err)
	}

	assertCommonHeaders(t, capturedReq, strPtr("acme"))

	var envelope Envelope[[]Task]
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if len(envelope.Data) == 0 || envelope.Data[0].ID != "tsk_001" {
		t.Errorf("unexpected tasks data: %v", envelope.Data)
	}
}

// --- GET /mission/tasks/{id} ---

func TestIntegration_GetTask_ByID(t *testing.T) {
	const taskID = "tsk_001"
	var capturedPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id": taskID, "code": "T-1", "title": "Fix bug", "status": "open",
				"has_subtasks": false, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z",
			},
		})
	}))
	t.Cleanup(srv.Close)

	tenant := "acme"
	c := newIntegrationClient(t, srv, "csk_testkey")
	resp, err := c.Do(context.Background(), "GET", "/mission/tasks/"+taskID, nil, &tenant)
	if err != nil {
		t.Fatalf("GET /mission/tasks/{id}: %v", err)
	}

	if !strings.HasSuffix(capturedPath, "/mission/tasks/"+taskID) {
		t.Errorf("path = %q; want suffix /mission/tasks/%s", capturedPath, taskID)
	}

	var envelope struct {
		Data Task `json:"data"`
	}
	if err := json.Unmarshal(resp.Body, &envelope); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if envelope.Data.ID != taskID {
		t.Errorf("task ID = %q; want %q", envelope.Data.ID, taskID)
	}
}

// --- POST /mission/tasks ---

func TestIntegration_CreateTask_TenantInBodyNotHeader(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody CreateTaskRequest

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r

		// Fail the test immediately if X-Tenant-Code was set — it must not be.
		if r.Header.Get("X-Tenant-Code") != "" {
			http.Error(w, "X-Tenant-Code must not be set for POST /mission/tasks", http.StatusBadRequest)
			return
		}

		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id": "tsk_002", "code": "T-2", "title": capturedBody.Title, "status": "open",
				"tenant_code":  capturedBody.TenantCode,
				"has_subtasks": false, "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z",
			},
		})
	}))
	t.Cleanup(srv.Close)

	c := newIntegrationClient(t, srv, "csk_testkey")
	body := CreateTaskRequest{
		TenantCode: "acme",
		Title:      "New integration task",
	}
	// Note: tenant passed as nil to Do — tenant_code is in the body, not the header.
	resp, err := c.Do(context.Background(), "POST", "/mission/tasks", body, nil)
	if err != nil {
		t.Fatalf("POST /mission/tasks: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d; want 201", resp.StatusCode)
	}

	// Must have no X-Tenant-Code header.
	assertCommonHeaders(t, capturedReq, nil)

	// tenant_code must appear in the request body.
	if capturedBody.TenantCode != "acme" {
		t.Errorf("body.tenant_code = %q; want acme", capturedBody.TenantCode)
	}
	if capturedBody.Title != "New integration task" {
		t.Errorf("body.title = %q; want 'New integration task'", capturedBody.Title)
	}
}

// --- Error response mapping ---

func TestIntegration_ErrorResponseMapping(t *testing.T) {
	tests := []struct {
		name         string
		httpStatus   int
		apiCode      string
		wantExitCode int
	}{
		{"400 validation", 400, "VALIDATION_ERROR", 5},
		{"401 unauthorized", 401, "UNAUTHORIZED", 2},
		{"403 forbidden", 403, "FORBIDDEN", 3},
		{"404 not found", 404, "NOT_FOUND", 4},
		{"429 rate limit", 429, "RATE_LIMIT_EXCEEDED", 7},
		{"500 server error", 500, "INTERNAL_ERROR", 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			status := tt.httpStatus
			code := tt.apiCode

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":    code,
						"message": "test error message",
					},
				})
			}))
			t.Cleanup(srv.Close)

			c := newIntegrationClient(t, srv, "csk_testkey")
			_, err := c.Do(context.Background(), "GET", "/mission/tasks", nil, nil)
			if err == nil {
				t.Fatalf("expected error for HTTP %d", status)
			}

			gotExitCode := ExitCodeFor(err)
			if gotExitCode != tt.wantExitCode {
				t.Errorf("ExitCodeFor (HTTP %d) = %d; want %d (err=%v)",
					status, gotExitCode, tt.wantExitCode, err)
			}

			var apiErr *APIError
			if !isAPIError(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T", err)
			}
			if apiErr.Code != code {
				t.Errorf("APIError.Code = %q; want %q", apiErr.Code, code)
			}
			if apiErr.HTTPStatus != status {
				t.Errorf("APIError.HTTPStatus = %d; want %d", apiErr.HTTPStatus, status)
			}
		})
	}
}

// --- tasks create rejects global mode (exit code 5) ---
// This is a unit-level check on the validation logic in cmd/tasks.go.
// We verify the exit code mapping used for the VALIDATION_ERROR APIError
// that tasks create emits when no tenant is resolved.
func TestIntegration_TasksCreate_GlobalModeExitCode(t *testing.T) {
	// The cmd layer creates this error when isGlobal == true.
	err := &APIError{
		Code:       "VALIDATION_ERROR",
		Message:    "tasks create requires a tenant; pass --tenant <code> or set default",
		HTTPStatus: 400,
	}
	got := ExitCodeFor(err)
	if got != 5 {
		t.Errorf("ExitCodeFor(VALIDATION_ERROR/400) = %d; want 5", got)
	}
}

// --- Header isolation: X-Request-Id is unique per request ---

func TestIntegration_UniqueRequestIDs(t *testing.T) {
	var requestIDs []string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIDs = append(requestIDs, r.Header.Get("X-Request-Id"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newIntegrationClient(t, srv, "csk_testkey")
	const n = 5
	for i := 0; i < n; i++ {
		_, _ = c.Do(context.Background(), "GET", "/tenants", nil, nil)
	}

	if len(requestIDs) != n {
		t.Fatalf("expected %d requests, got %d", n, len(requestIDs))
	}
	seen := make(map[string]bool, n)
	for _, rid := range requestIDs {
		if seen[rid] {
			t.Errorf("duplicate X-Request-Id: %q", rid)
		}
		seen[rid] = true
	}
}
