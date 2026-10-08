package cmd

import "testing"

func TestMembersInviteSendsCustomMessage(t *testing.T) {
	memberInviteTenant, memberInviteEmail, memberInviteCustomMessage = "acme", "tram@acme.vn", "  Welcome  "
	t.Cleanup(func() { memberInviteTenant, memberInviteEmail, memberInviteCustomMessage = "", "", "" })

	seen, _ := callAgainst(t, `{"data":{"id":"inv-1","status":"pending"}}`,
		func() { _ = membersInviteCmd.RunE(membersInviteCmd, nil) })

	if got := seen.body["custom_message"]; got != "Welcome" {
		t.Errorf("custom_message = %v, want the trimmed text", got)
	}
}

func TestInvitationsListQuery(t *testing.T) {
	invitationListTenant, invitationListStatus, invitationListEmail = "acme", "expired", " Tram@Acme.vn "
	invitationListPage, invitationListLimit = 2, 50
	t.Cleanup(func() {
		invitationListTenant, invitationListStatus, invitationListEmail = "", "", ""
		invitationListPage, invitationListLimit = 0, 0
	})

	seen, printed := callAgainst(t, `{"data":[{"id":"inv-1"}],"meta":{"page":2,"limit":50,"total":51,"has_more":false}}`,
		func() { _ = memberInvitationsListCmd.RunE(memberInvitationsListCmd, nil) })

	if seen.method != "GET" || seen.path != "/members/invitations" {
		t.Errorf("call = %s %s, want GET /members/invitations", seen.method, seen.path)
	}
	if seen.tenant != "acme" {
		t.Errorf("X-Tenant-Code = %q, want acme", seen.tenant)
	}
	if seen.query != "email=Tram%40Acme.vn&limit=50&page=2&status=expired" {
		t.Errorf("query = %q", seen.query)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestInvitationsCancelPostsWithoutBody(t *testing.T) {
	invitationCancelTenant = "acme"
	t.Cleanup(func() { invitationCancelTenant = "" })

	seen, printed := callAgainst(t, `{"data":{"id":"inv-1","status":"cancelled"}}`,
		func() { _ = memberInvitationsCancelCmd.RunE(memberInvitationsCancelCmd, []string{"inv-1"}) })

	if seen.method != "POST" || seen.path != "/members/invitations/inv-1/actions/cancel" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	if seen.hasBody {
		t.Errorf("cancel sent a body %v; the API refuses a field", seen.body)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestJoinRequestsListQuery(t *testing.T) {
	joinRequestListTenant, joinRequestListStatus = "acme", "pending"
	t.Cleanup(func() { joinRequestListTenant, joinRequestListStatus = "", "" })

	seen, _ := callAgainst(t, `{"data":[],"meta":{"page":1,"limit":20,"total":0,"has_more":false}}`,
		func() { _ = joinRequestsListCmd.RunE(joinRequestsListCmd, nil) })

	if seen.method != "GET" || seen.path != "/join-requests" || seen.query != "status=pending" {
		t.Errorf("call = %s %s?%s, want GET /join-requests?status=pending", seen.method, seen.path, seen.query)
	}
}

func TestListQueryOmitsUnsetFilters(t *testing.T) {
	if got := listQuery("x", "", invitationStatuses, "  ", 0, 0); got != "" {
		t.Errorf("listQuery with nothing set = %q, want empty", got)
	}
}
