package cmd

import (
	"encoding/json"

	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/output"
)

// The tenant a command resolved to is the one fact about a call that the API's
// own response never carries. It used to be printed as a line of prose, in one
// output mode out of three:
//
//	Tenant: acme (from CAPIGO_TENANT)
//
// With a single JSON shape there is nowhere for a line of prose to live, and
// nowhere it should: a caller that has just written to the wrong tenant needs
// to be able to read that fact, not notice it. It goes in meta.

// tenantSource names where a resolved tenant came from, so a silently
// defaulted tenant is visible rather than assumed. Empty when --tenant was
// given explicitly: there is nothing to explain.
func tenantSource(tenant *string, tenantFlag string) string {
	if tenant == nil {
		return ""
	}
	if tenantFlag != "" {
		return "flag"
	}
	if viper.GetString("tenant") != "" {
		return "env"
	}
	return "config"
}

// itemMeta is the meta of a single-item response: a get, or a successful write.
// raw is the API's own meta, which may be absent, and whose keys the CLI has no
// business filtering.
func itemMeta(tenant *string, tenantFlag string, raw json.RawMessage) output.Meta {
	m := mergeAPIMeta(raw)
	if tenant != nil {
		m.Tenant = *tenant
		m.TenantSource = tenantSource(tenant, tenantFlag)
	}
	return m
}

// listMeta is itemMeta by another name. Pagination is not something the CLI adds
// — it is four of the API's own meta keys, and they arrive the same way
// `list_count` does on a board.
func listMeta(tenant *string, tenantFlag string, raw json.RawMessage) output.Meta {
	return itemMeta(tenant, tenantFlag, raw)
}

// knownMetaKeys are the ones output.Meta names explicitly, so they render in a
// fixed order rather than wherever a map iteration puts them.
var knownMetaKeys = map[string]bool{
	"page": true, "limit": true, "total": true, "has_more": true,
}

// mergeAPIMeta lifts the API's meta into ours: the four pagination keys into
// their fields, everything else into Extra, untouched.
//
// An absent meta stays absent. GET /tenants sends none, and a meta of zeros is
// not the same answer as no meta — `capigo tenants list` once printed
// "total": 0 beside a row of data because a value type could not tell them apart.
func mergeAPIMeta(raw json.RawMessage) output.Meta {
	var m output.Meta
	if len(raw) == 0 || string(raw) == "null" {
		return m
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return m
	}
	for k, v := range fields {
		if !knownMetaKeys[k] {
			var value any
			if json.Unmarshal(v, &value) == nil {
				if m.Extra == nil {
					m.Extra = map[string]any{}
				}
				m.Extra[k] = value
			}
			continue
		}
		switch k {
		case "page":
			m.Page = intPtr(v)
		case "limit":
			m.Limit = intPtr(v)
		case "total":
			m.Total = intPtr(v)
		case "has_more":
			var b bool
			if json.Unmarshal(v, &b) == nil {
				m.HasMore = output.Ptr(b)
			}
		}
	}
	return m
}

func intPtr(v json.RawMessage) *int {
	var n int
	if json.Unmarshal(v, &n) != nil {
		return nil
	}
	return output.Ptr(n)
}

// rawList is the API's `data` array, passed through untouched. An absent array
// becomes an empty one: a caller must not have to tell "no rows" apart from "no
// such key".
func rawList(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage("[]")
	}
	return data
}

// rawItem is the API's `data` object, passed through untouched.
func rawItem(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage("null")
	}
	return data
}

// idsOf reads the `id` of every row, for logic that must reason about which
// records came back. It never decides what the caller sees: the rows are still
// printed verbatim.
func idsOf(rows json.RawMessage) []string {
	var out []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rows, &out); err != nil {
		return nil
	}
	ids := make([]string, 0, len(out))
	for _, r := range out {
		ids = append(ids, r.ID)
	}
	return ids
}

// tenantCodesOf reads the `tenant_code` of every row, for the same reason.
func tenantCodesOf(rows json.RawMessage) []string {
	var out []struct {
		TenantCode string `json:"tenant_code"`
	}
	if err := json.Unmarshal(rows, &out); err != nil {
		return nil
	}
	codes := make([]string, 0, len(out))
	for _, r := range out {
		codes = append(codes, r.TenantCode)
	}
	return codes
}
