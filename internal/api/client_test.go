package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// userAgentPattern matches: capigo-api-sdk/<version> (<os>; <arch>)
var userAgentPattern = regexp.MustCompile(`^capigo-api-sdk/.+ \(.+; .+\)$`)

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(server.URL, "csk_testkey123")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Use the TLS server's transport.
	c.http = server.Client()
	return c
}

func TestNewClient_RejectsHTTPNonLocalhost(t *testing.T) {
	_, err := NewClient("http://api.example.com/api/v1", "csk_test")
	if err == nil {
		t.Fatal("expected error for http:// non-localhost URL")
	}
}

func TestNewClient_AllowsHTTPLocalhost(t *testing.T) {
	_, err := NewClient("http://localhost:3999/api/v1", "csk_test")
	if err != nil {
		t.Fatalf("unexpected error for localhost: %v", err)
	}
}

func TestNewClient_Allows127(t *testing.T) {
	_, err := NewClient("http://127.0.0.1:3999/api/v1", "csk_test")
	if err != nil {
		t.Fatalf("unexpected error for 127.0.0.1: %v", err)
	}
}

func TestDo_RequestHeaders(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	tenant := "acme"
	c := newTestClient(t, srv)
	_, err := c.Do(context.Background(), "GET", "/tenants", nil, &tenant)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	// Authorization header must contain the key.
	auth := capturedReq.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer csk_") {
		t.Errorf("Authorization = %q, want Bearer csk_...", auth)
	}

	// X-Tenant-Code must be set.
	if got := capturedReq.Header.Get("X-Tenant-Code"); got != "acme" {
		t.Errorf("X-Tenant-Code = %q, want %q", got, "acme")
	}

	// User-Agent must match pattern.
	ua := capturedReq.Header.Get("User-Agent")
	if !userAgentPattern.MatchString(ua) {
		t.Errorf("User-Agent = %q, does not match expected pattern", ua)
	}

	// X-Request-Id must be a valid UUID.
	rid := capturedReq.Header.Get("X-Request-Id")
	if _, err := uuid.Parse(rid); err != nil {
		t.Errorf("X-Request-Id = %q is not a valid UUID: %v", rid, err)
	}
}

func TestDo_NoTenantHeader_WhenTenantNil(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Do(context.Background(), "GET", "/mission/tasks", nil, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	if got := capturedReq.Header.Get("X-Tenant-Code"); got != "" {
		t.Errorf("X-Tenant-Code should be absent, got %q", got)
	}
}

func TestDo_ContentTypeSet_WhenBodyPresent(t *testing.T) {
	var capturedReq *http.Request
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	body := map[string]string{"tenant_code": "acme", "title": "Test"}
	_, err := c.Do(context.Background(), "POST", "/mission/tasks", body, nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	ct := capturedReq.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

type statusTest struct {
	httpStatus   int
	body         string
	wantExitCode int
}

func TestDo_ExitCodeMapping(t *testing.T) {
	tests := []statusTest{
		{200, `{"data":[]}`, 0},
		{400, `{"error":{"code":"BAD","message":"bad"}}`, 5},
		{401, `{"error":{"code":"UNAUTH","message":"unauth"}}`, 2},
		{403, `{"error":{"code":"FORBIDDEN","message":"forbidden"}}`, 3},
		{404, `{"error":{"code":"NOT_FOUND","message":"not found"}}`, 4},
		{429, `{"error":{"code":"RATE_LIMIT","message":"slow down"}}`, 7},
		{500, `{"error":{"code":"SERVER_ERROR","message":"oops"}}`, 1},
	}

	for _, tt := range tests {
		status := tt.httpStatus
		body := tt.body
		t.Run("HTTP_"+http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			_, err := c.Do(context.Background(), "GET", "/test", nil, nil)

			got := ExitCodeFor(err)
			if got != tt.wantExitCode {
				t.Errorf("HTTP %d: ExitCodeFor = %d, want %d (err=%v)", status, got, tt.wantExitCode, err)
			}
		})
	}
}

func TestDo_ParsesAPIError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		body, _ := json.Marshal(map[string]any{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "task not found",
			},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Do(context.Background(), "GET", "/mission/tasks/bad", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !isAPIError(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want NOT_FOUND", apiErr.Code)
	}
	if apiErr.HTTPStatus != 404 {
		t.Errorf("HTTPStatus = %d, want 404", apiErr.HTTPStatus)
	}
}

func isAPIError(err error, target **APIError) bool {
	return errors.As(err, target)
}
