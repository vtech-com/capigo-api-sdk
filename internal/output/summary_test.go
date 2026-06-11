package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteListSummary(t *testing.T) {
	tests := []struct {
		name    string
		in      ListSummary
		want    string
		notWant string
	}{
		{
			name: "more pages surfaces total and page count",
			in:   ListSummary{Shown: 20, Page: 1, Limit: 20, Total: 43, HasMore: true},
			want: "Total: 43 · showing 20 (page 1/3) · more rows — use --page/--limit (max 100)",
		},
		{
			name: "products advertises --all",
			in:   ListSummary{Shown: 20, Page: 1, Limit: 20, Total: 43, HasMore: true, HintAll: true},
			want: "or --all",
		},
		{
			name: "single page reports all rows shown",
			in:   ListSummary{Shown: 12, Page: 1, Limit: 20, Total: 12, HasMore: false},
			want: "Total: 12 (all rows shown)",
		},
		{
			name: "empty result",
			in:   ListSummary{Shown: 0, Page: 1, Limit: 20, Total: 0, HasMore: false},
			want: "Total: 0 (no matching rows)",
		},
		{
			name: "unreliable meta (total unset) never undercounts visible rows",
			in:   ListSummary{Shown: 7, Page: 1, Limit: 0, Total: 0, HasMore: false},
			want: "Total: 7 (all rows shown)",
		},
		{
			// The exact regression: a 20-row page of a 43-row collection must
			// never read as "20 total" on stdout.
			name:    "page one never claims the page size is the total",
			in:      ListSummary{Shown: 20, Page: 1, Limit: 20, Total: 43, HasMore: true},
			notWant: "all rows shown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteListSummary(&buf, tc.in)
			got := buf.String()
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Errorf("WriteListSummary = %q, want substring %q", got, tc.want)
			}
			if tc.notWant != "" && strings.Contains(got, tc.notWant) {
				t.Errorf("WriteListSummary = %q, must not contain %q", got, tc.notWant)
			}
		})
	}
}
