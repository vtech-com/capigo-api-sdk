package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAll_MultiplePagesUntilHasMoreFalse(t *testing.T) {
	// Return 3 pages of 2 items each; has_more=false on the last page.
	callCount := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")
		hasMore := page != "3"
		body, _ := json.Marshal(map[string]any{
			"data": []map[string]string{
				{"id": fmt.Sprintf("item-%s-1", page)},
				{"id": fmt.Sprintf("item-%s-2", page)},
			},
			"meta": map[string]any{
				"page":     1,
				"limit":    2,
				"total":    6,
				"has_more": hasMore,
			},
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "csk_test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c.http = srv.Client()

	type item struct {
		ID string `json:"id"`
	}
	items, err := FetchAll[item](context.Background(), c, "/test", nil)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}

	if len(items) != 6 {
		t.Errorf("len(items) = %d, want 6", len(items))
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}
