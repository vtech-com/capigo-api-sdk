package cmd

import (
	"encoding/json"

	"github.com/spf13/viper"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
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
func itemMeta(tenant *string, tenantFlag string) output.Meta {
	m := output.Meta{}
	if tenant != nil {
		m.Tenant = *tenant
		m.TenantSource = tenantSource(tenant, tenantFlag)
	}
	return m
}

// listMeta is itemMeta plus the API's pagination. Page, limit and total are
// always emitted for a list, including when the list is empty — a caller that
// asks "how many are there" must not have to distinguish a zero from a
// missing key.
func listMeta(tenant *string, tenantFlag string, m *api.Meta) output.Meta {
	out := itemMeta(tenant, tenantFlag)
	if m == nil {
		// The endpoint sent no pagination. Emitting zeros here would answer
		// "how many are there?" with 0 while data[] holds rows.
		return out
	}
	out.Page = output.Ptr(m.Page)
	out.Limit = output.Ptr(m.Limit)
	out.Total = output.Ptr(m.Total)
	out.HasMore = output.Ptr(m.HasMore)
	return out
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
