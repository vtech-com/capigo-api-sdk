package cmd

import (
	"testing"
)

const wmsMemberReply = `{"data":{"id":"m1","user_id":"u1","scopes":["scope.inbound"],"status":"active"}}`

func TestWMSWarehouseMembersList(t *testing.T) {
	setFlags(t, wmsWarehouseMembersListCmd, "tenant", "acme", "warehouse", "WH HN", "page", "2", "limit", "50")

	seen, _ := callAgainst(t, `{"data":[],"meta":{"page":2,"limit":50,"total":0,"has_more":false}}`,
		func() { _ = wmsWarehouseMembersListCmd.RunE(wmsWarehouseMembersListCmd, nil) })

	if seen.method != "GET" || seen.path != "/wms/warehouses/WH HN/members" {
		t.Errorf("call = %s %s, want GET on the warehouse's members", seen.method, seen.path)
	}
	if seen.query != "limit=50&page=2" {
		t.Errorf("query = %q, want limit=50&page=2", seen.query)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
}

func TestWMSWarehouseMembersAddSendsUserAndScopes(t *testing.T) {
	setFlags(t, wmsWarehouseMembersAddCmd, "tenant", "acme", "warehouse", "WH-HN",
		"user-id", " u1 ", "scope", "scope.inbound", "scope", " scope.outbound ")

	seen, printed := callAgainstStatus(t, 201, wmsMemberReply,
		func() { _ = wmsWarehouseMembersAddCmd.RunE(wmsWarehouseMembersAddCmd, nil) })

	if seen.method != "POST" || seen.path != "/wms/warehouses/WH-HN/members" {
		t.Errorf("call = %s %s, want POST /wms/warehouses/WH-HN/members", seen.method, seen.path)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"scopes", "user_id"}) {
		t.Errorf("body keys = %v, want user_id and scopes", got)
	}
	if seen.body["user_id"] != "u1" {
		t.Errorf("user_id = %v, want the trimmed u1", seen.body["user_id"])
	}
	scopes, _ := seen.body["scopes"].([]any)
	if len(scopes) != 2 || scopes[1] != "scope.outbound" {
		t.Errorf("scopes = %v, want both trimmed scopes", seen.body["scopes"])
	}
	if data, _ := printedData(t, printed); data["id"] != "m1" {
		t.Errorf("printed %s, want the membership at .data", printed)
	}
}

// The API requires the scopes array even when it is empty, and an empty one
// stores scope.view; a null would be refused.
func TestWMSWarehouseMembersAddWithoutScopesSendsAnEmptyArray(t *testing.T) {
	setFlags(t, wmsWarehouseMembersAddCmd, "tenant", "acme", "warehouse", "WH-HN", "user-id", "u1")

	seen, _ := callAgainstStatus(t, 201, wmsMemberReply,
		func() { _ = wmsWarehouseMembersAddCmd.RunE(wmsWarehouseMembersAddCmd, nil) })

	if scopes, ok := seen.body["scopes"].([]any); !ok || len(scopes) != 0 {
		t.Errorf("scopes = %v (raw %s), want []", seen.body["scopes"], seen.rawBody)
	}
}

func TestWMSWarehouseMembersScopesCallTheirOwnRoute(t *testing.T) {
	for _, change := range []string{"add", "remove"} {
		t.Run(change, func(t *testing.T) {
			c := wmsWarehouseMembersScopesAddCmd
			if change == "remove" {
				c = wmsWarehouseMembersScopesRemoveCmd
			}
			setFlags(t, c, "tenant", "acme", "warehouse", "WH-HN", "scope", "scope.outbound")

			seen, _ := callAgainst(t, wmsMemberReply, func() { _ = c.RunE(c, []string{" m1 "}) })

			if want := "/wms/warehouses/WH-HN/members/m1/scopes/" + change; seen.method != "POST" || seen.path != want {
				t.Errorf("call = %s %s, want POST %s", seen.method, seen.path, want)
			}
			if got := keysOf(seen.body); !sameKeys(got, []string{"scopes"}) {
				t.Errorf("body keys = %v, want scopes only", got)
			}
		})
	}
}

func TestWMSWarehouseMembersRemove(t *testing.T) {
	setFlags(t, wmsWarehouseMembersRemoveCmd, "tenant", "acme", "warehouse", "WH-HN")

	seen, printed := callAgainst(t, `{"data":{"id":"m1"}}`,
		func() { _ = wmsWarehouseMembersRemoveCmd.RunE(wmsWarehouseMembersRemoveCmd, []string{"m1"}) })

	if seen.method != "DELETE" || seen.path != "/wms/warehouses/WH-HN/members/m1" {
		t.Errorf("call = %s %s, want DELETE /wms/warehouses/WH-HN/members/m1", seen.method, seen.path)
	}
	if seen.hasBody {
		t.Errorf("body = %s, want none", seen.rawBody)
	}
	if data, _ := printedData(t, printed); data["id"] != "m1" {
		t.Errorf("printed %s, want the removed id at .data", printed)
	}
}
