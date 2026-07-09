package cmd

import (
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// TestMissingProductIDs guards the --ids reconciliation. The set difference is
// not reported in meta — a caller holds both sides of it — but the command uses
// it to decide its exit code: asking for ids and getting fewer rows must not
// exit 0.
func TestMissingProductIDs(t *testing.T) {
	got := []api.Product{{ID: "aaa"}, {ID: "BBB"}}

	tests := []struct {
		name          string
		idsFlag       string
		wantMissing   []string
		wantRequested int
	}{
		{"no flag means no reconciliation", "", nil, 0},
		{"all found", "aaa,bbb", nil, 2},
		{"case-insensitive match", "AAA,bbb", nil, 2},
		{"some missing", "aaa,ccc,ddd", []string{"ccc", "ddd"}, 3},
		{"whitespace and empty parts ignored", " aaa , ,ccc", []string{"ccc"}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			missing, requested := missingProductIDs(tc.idsFlag, got)
			if requested != tc.wantRequested {
				t.Errorf("requested = %d, want %d", requested, tc.wantRequested)
			}
			if len(missing) != len(tc.wantMissing) {
				t.Fatalf("missing = %v, want %v", missing, tc.wantMissing)
			}
			for i := range missing {
				if missing[i] != tc.wantMissing[i] {
					t.Errorf("missing[%d] = %q, want %q", i, missing[i], tc.wantMissing[i])
				}
			}
		})
	}
}
