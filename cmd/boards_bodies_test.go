package cmd

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

// The body tags are what the API's strict schemas match on, and neither guard
// can see them: the body-coverage guard checks that a flag exists per spec field
// — and skips any command registering --from-json, which `boards lists update`
// does — never the JSON the client actually emits. A renamed tag is a 400 at
// runtime with every test green, so each board write's key set is pinned here.
func TestBoardWriteBodiesCarryTheSpecKeys(t *testing.T) {
	after := "4d9a1c07-2b6e-4f83-a5d1-8c07e2f419bb"
	listName := "Doing"

	cases := []struct {
		name string
		body any
		want []string
	}{
		{
			name: "boards members add",
			body: api.AddBoardMembersRequest{
				TenantCode: "acme",
				UserIDs:    []string{after},
			},
			want: []string{"tenant_code", "user_ids"},
		},
		{
			name: "boards members update",
			body: api.UpdateBoardMemberRequest{TenantCode: "acme", Role: "owner"},
			want: []string{"tenant_code", "role"},
		},
		{
			name: "boards members remove",
			body: api.RemoveBoardMemberRequest{TenantCode: "acme"},
			want: []string{"tenant_code"},
		},
		{
			name: "boards lists update, reorder",
			body: api.UpdateBoardListRequest{TenantCode: "acme", AfterListID: &after},
			want: []string{"tenant_code", "after_list_id"},
		},
		{
			name: "boards lists update, fields",
			body: api.UpdateBoardListRequest{TenantCode: "acme", Name: &listName},
			want: []string{"tenant_code", "name"},
		},
		{
			name: "boards lists delete",
			body: api.DeleteBoardListRequest{TenantCode: "acme"},
			want: []string{"tenant_code"},
		},
		{
			name: "boards delete",
			body: api.DeleteBoardRequest{TenantCode: "acme"},
			want: []string{"tenant_code"},
		},
	}

	for _, tc := range cases {
		raw, err := json.Marshal(tc.body)
		if err != nil {
			t.Fatalf("%s: marshal: %v", tc.name, err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s: unmarshal: %v", tc.name, err)
		}

		keys := make([]string, 0, len(got))
		for key := range got {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		want := append([]string(nil), tc.want...)
		sort.Strings(want)

		if !reflect.DeepEqual(keys, want) {
			t.Errorf("%s: body keys = %v, want %v — the API's strict schema matches these names exactly, and a mismatch is a 400 at runtime",
				tc.name, keys, want)
		}
	}
}
