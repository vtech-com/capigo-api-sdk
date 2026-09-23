package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBoardWriteEndpoints pins the method, path, tenant header, and body that
// each board write method sends. This is the endpoint-level seam: the cmd
// handlers build the payload, these methods own the wire.
func TestBoardWriteEndpoints(t *testing.T) {
	tenant := "acme"
	boardID := "550e8400-e29b-41d4-a716-446655440000"
	listID := "9ab2c744-16fe-4d09-8a52-3ef0b7c61d84"

	tests := []struct {
		name       string
		call       func(c *Client) error
		wantMethod string
		wantPath   string
		wantBody   map[string]any
	}{
		{
			name: "CreateBoard",
			call: func(c *Client) error {
				_, err := c.CreateBoard(context.Background(),
					CreateBoardRequest{TenantCode: tenant, Name: "Sprint 5"}, &tenant)
				return err
			},
			wantMethod: "POST",
			wantPath:   "/mission/boards",
			wantBody:   map[string]any{"tenant_code": "acme", "name": "Sprint 5"},
		},
		{
			name: "UpdateBoard",
			call: func(c *Client) error {
				name := "Sprint 6"
				_, err := c.UpdateBoard(context.Background(), boardID,
					UpdateBoardRequest{TenantCode: tenant, Name: &name}, &tenant)
				return err
			},
			wantMethod: "PATCH",
			wantPath:   "/mission/boards/" + boardID,
			wantBody:   map[string]any{"tenant_code": "acme", "name": "Sprint 6"},
		},
		{
			name: "CreateBoardList",
			call: func(c *Client) error {
				_, err := c.CreateBoardList(context.Background(), boardID,
					CreateBoardListRequest{TenantCode: tenant, Name: "Backlog"}, &tenant)
				return err
			},
			wantMethod: "POST",
			wantPath:   "/mission/boards/" + boardID + "/lists",
			wantBody:   map[string]any{"tenant_code": "acme", "name": "Backlog"},
		},
		{
			name: "UpdateBoardList",
			call: func(c *Client) error {
				_, err := c.UpdateBoardList(context.Background(), boardID, listID,
					UpdateBoardListRequest{TenantCode: tenant}, &tenant)
				return err
			},
			wantMethod: "PATCH",
			wantPath:   "/mission/boards/" + boardID + "/lists/" + listID,
			wantBody:   map[string]any{"tenant_code": "acme"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotTenant string
			var gotBody map[string]any

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotTenant = r.Header.Get("X-Tenant-Code")
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &gotBody)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{}}`))
			}))
			defer srv.Close()

			c := newTestClient(t, srv)
			if err := tt.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}

			if gotMethod != tt.wantMethod {
				t.Errorf("method = %q, want %q", gotMethod, tt.wantMethod)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotTenant != tenant {
				t.Errorf("X-Tenant-Code = %q, want %q", gotTenant, tenant)
			}
			for k, want := range tt.wantBody {
				if gotBody[k] != want {
					t.Errorf("body[%q] = %v, want %v", k, gotBody[k], want)
				}
			}
		})
	}
}

// TestCreateBoardRequest_Shape verifies the nested lists array serializes and
// that tenant_code is a body field.
func TestCreateBoardRequest_Shape(t *testing.T) {
	limit := 5
	req := CreateBoardRequest{
		TenantCode: "acme",
		Name:       "Sprint 5",
		Lists: []CreateBoardListItem{
			{Name: "Backlog", Limit: &limit},
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

	if m["tenant_code"] != "acme" {
		t.Errorf("tenant_code = %v, want acme", m["tenant_code"])
	}
	lists, ok := m["lists"].([]any)
	if !ok || len(lists) != 1 {
		t.Fatalf("lists malformed: %v", m["lists"])
	}
	first, _ := lists[0].(map[string]any)
	if first["name"] != "Backlog" {
		t.Errorf("lists[0].name = %v, want Backlog", first["name"])
	}
	if first["limit"] != float64(5) {
		t.Errorf("lists[0].limit = %v, want 5", first["limit"])
	}
}
