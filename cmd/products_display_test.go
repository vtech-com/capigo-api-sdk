package cmd

import (
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// TestToOutputProduct_SoftDeleteAndAliases guards the two table-visibility
// fixes: a soft-deleted product must never render as plainly live, and the
// aliases the dedup workflow depends on must reach the display model.
func TestToOutputProduct_SoftDeleteAndAliases(t *testing.T) {
	p := api.Product{
		ID:        "p1",
		Name:      "Pin iPhone 13",
		Status:    "ACTIVE",
		Aliases:   []string{"AP-BA-13", "PIN-IP13"},
		IsDeleted: true,
	}

	got := toOutputProduct(p)

	if got.Status != "ACTIVE (DELETED)" {
		t.Errorf("Status = %q, want %q — a soft-deleted product must carry the tombstone", got.Status, "ACTIVE (DELETED)")
	}
	if got.Aliases != "AP-BA-13, PIN-IP13" {
		t.Errorf("Aliases = %q, want joined alias list", got.Aliases)
	}

	p.IsDeleted = false
	if got := toOutputProduct(p); got.Status != "ACTIVE" {
		t.Errorf("Status = %q, want %q for a live product", got.Status, "ACTIVE")
	}
}
