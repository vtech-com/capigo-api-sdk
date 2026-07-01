package cmd

import (
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// TestToOutputProduct_SoftDeleteAndAliases guards the table-visibility
// fixes: a soft-deleted product must never render as plainly live, and the
// aliases and tags the dedup/collision workflow depends on (both matched by
// --query) must reach the display model.
func TestToOutputProduct_SoftDeleteAndAliases(t *testing.T) {
	p := api.Product{
		ID:        "p1",
		Name:      "Pin iPhone 13",
		Status:    "ACTIVE",
		Aliases:   []string{"AP-BA-13", "PIN-IP13"},
		Tags:      []string{"organic", "premium"},
		IsDeleted: true,
	}

	got := toOutputProduct(p)

	if got.Status != "ACTIVE (DELETED)" {
		t.Errorf("Status = %q, want %q — a soft-deleted product must carry the tombstone", got.Status, "ACTIVE (DELETED)")
	}
	if got.Aliases != "AP-BA-13, PIN-IP13" {
		t.Errorf("Aliases = %q, want joined alias list", got.Aliases)
	}
	if got.Tags != "organic, premium" {
		t.Errorf("Tags = %q, want joined tag list", got.Tags)
	}

	p.IsDeleted = false
	if got := toOutputProduct(p); got.Status != "ACTIVE" {
		t.Errorf("Status = %q, want %q for a live product", got.Status, "ACTIVE")
	}
}

// TestMissingProductIDs guards the --ids reconciliation: a clean exit 0 with
// fewer rows than requested must name the IDs that did not come back.
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
