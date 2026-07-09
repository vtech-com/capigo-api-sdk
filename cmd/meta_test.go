package cmd

import (
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// GET /tenants sends no meta. Decoded into a value type, its absence became a
// meta of zeros, and `capigo tenants list` printed "total": 0 beside a row of
// data. The CLI's own rule — read meta.total, do not count data[] — turned that
// zero into the wrong answer, stated confidently, on the stream a caller parses.
func TestListMetaOmitsPaginationWhenTheAPISendsNone(t *testing.T) {
	m := listMeta(nil, "", nil)
	if m.Page != nil || m.Limit != nil || m.Total != nil || m.HasMore != nil {
		t.Errorf("an absent meta must not become a meta of zeros: %+v", m)
	}
}

// And when the API does send one, every field survives — including a real zero,
// which is the answer to "how many are there?" on an empty tenant.
func TestListMetaKeepsARealZero(t *testing.T) {
	m := listMeta(nil, "", &api.Meta{Page: 1, Limit: 20, Total: 0, HasMore: false})
	if m.Total == nil || *m.Total != 0 {
		t.Errorf("a real total of zero was dropped: %+v", m.Total)
	}
	if m.Page == nil || *m.Page != 1 {
		t.Errorf("page lost: %+v", m.Page)
	}
}
