package cmd

import (
	"testing"
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
	m := listMeta(nil, "", []byte(`{"page":1,"limit":20,"total":0,"has_more":false}`))
	if m.Total == nil || *m.Total != 0 {
		t.Errorf("a real total of zero was dropped: %+v", m.Total)
	}
	if m.Page == nil || *m.Page != 1 {
		t.Errorf("page lost: %+v", m.Page)
	}
}

// The API's meta is the API's. GET /mission/boards/{id} sends `list_count`, and
// the CLI has no business deciding a caller may not see it — the rule it applies
// to `data`, applied to `meta`.
func TestListMetaCarriesTheAPIsOwnKeys(t *testing.T) {
	m := listMeta(nil, "", []byte(`{"list_count":5}`))
	if m.Extra["list_count"] != float64(5) {
		t.Errorf("list_count dropped: %+v", m.Extra)
	}
	if m.Page != nil || m.Total != nil {
		t.Errorf("invented pagination out of a meta that had none: %+v", m)
	}
}
