package api

import (
	"net/http"
	"strings"
	"testing"
)

func memberJSON(username string, role Role) []byte {
	return []byte(`{"username":"` + username + `","role":"` + string(role) + `"}`)
}

func roleJSON(role Role) []byte { return []byte(`{"role":"` + string(role) + `"}`) }

func memberNames(t *testing.T, srv *contractServer, book string) []string {
	t.Helper()
	res := srv.do(t, http.MethodGet, book+"/members", nil)
	wantStatus(t, res, http.StatusOK)
	var names []string
	for _, m := range decode[MemberList](t, res).Items {
		names = append(names, m.Username+":"+string(m.Role))
	}
	return names
}

func TestAddingAMemberSharesTheCookbook(t *testing.T) {
	srv, _ := newTestServer(t)
	anna := srv.asUser(t, "anna")

	res := srv.do(t, http.MethodPost, srv.book+"/members", memberJSON("Anna", Editor))

	wantStatus(t, res, http.StatusCreated)
	if m := decode[Member](t, res); m.Username != "anna" || m.Role != Editor || !m.JoinedAt.Equal(testNow) {
		t.Errorf("added %+v", m)
	}
	// Anna now sees the cookbook, its members and its recipes.
	if items := decode[CookbookList](t, anna.do(t, http.MethodGet, "/api/cookbooks", nil)).Items; len(items) != 1 || items[0].Role != Editor {
		t.Errorf("anna's cookbooks: %+v", items)
	}
	if got := strings.Join(memberNames(t, anna, srv.book), " "); got != "anna:editor dev:admin" {
		t.Errorf("members = %s", got)
	}
	wantStatus(t, anna.do(t, http.MethodPost, srv.book+"/recipes", mustJSON(t, pancakes())), http.StatusCreated)
}

func TestAddingMembersIsValidated(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.asUser(t, "anna")
	wantStatus(t, srv.do(t, http.MethodPost, srv.book+"/members", memberJSON("anna", Editor)), http.StatusCreated)

	res := srv.do(t, http.MethodPost, srv.book+"/members", memberJSON("anna", Admin))
	wantStatus(t, res, http.StatusConflict)
	if msg := decode[Error](t, res).Message; msg != "anna is already a member" {
		t.Errorf("message %q", msg)
	}

	for _, name := range []string{"nobody", "not a valid name!"} {
		res := srv.do(t, http.MethodPost, srv.book+"/members", memberJSON(name, Editor))
		wantStatus(t, res, http.StatusBadRequest)
		if msg := decode[Error](t, res).Message; !strings.HasPrefix(msg, "no account named") {
			t.Errorf("%s: message %q", name, msg)
		}
	}
	wantStatus(t, srv.doInvalid(t, http.MethodPost, srv.book+"/members", memberJSON("anna", "owner")), http.StatusBadRequest)
}

func TestOnlyAdminsManageMembers(t *testing.T) {
	srv, _ := newTestServer(t)
	anna := srv.asUser(t, "anna")
	srv.asUser(t, "bob")
	join(t, srv, srv.book, "anna", "editor")
	join(t, srv, srv.book, "bob", "editor")
	stranger := srv.asUser(t, "stranger")

	for _, res := range []*http.Response{
		anna.do(t, http.MethodPost, srv.book+"/members", memberJSON("stranger", Editor)),
		anna.do(t, http.MethodPut, srv.book+"/members/bob", roleJSON(Admin)),
		anna.do(t, http.MethodPut, srv.book+"/members/anna", roleJSON(Admin)), // no self-promotion
		anna.do(t, http.MethodDelete, srv.book+"/members/bob", nil),
		anna.do(t, http.MethodDelete, srv.book+"/members/nobody", nil),
	} {
		wantStatus(t, res, http.StatusForbidden)
	}
	// Non-members can't even see who's in it.
	wantStatus(t, stranger.do(t, http.MethodGet, srv.book+"/members", nil), http.StatusNotFound)
	wantStatus(t, stranger.do(t, http.MethodDelete, srv.book+"/members/anna", nil), http.StatusNotFound)

	if got := strings.Join(memberNames(t, srv, srv.book), " "); got != "anna:editor bob:editor dev:admin" {
		t.Errorf("members changed: %s", got)
	}
}

func TestCookbooksAlwaysKeepAnAdmin(t *testing.T) {
	srv, _ := newTestServer(t)
	anna := srv.asUser(t, "anna")
	join(t, srv, srv.book, "anna", "editor")

	// The only admin can't step down or leave...
	res := srv.do(t, http.MethodPut, srv.book+"/members/dev", roleJSON(Editor))
	wantStatus(t, res, http.StatusConflict)
	if msg := decode[Error](t, res).Message; !strings.Contains(msg, "make someone else admin first") {
		t.Errorf("message %q", msg)
	}
	res = srv.do(t, http.MethodDelete, srv.book+"/members/dev", nil)
	wantStatus(t, res, http.StatusConflict)
	if msg := decode[Error](t, res).Message; !strings.Contains(msg, "before leaving, or delete the cookbook") {
		t.Errorf("message %q", msg)
	}

	// ...until there's another admin.
	promoted := srv.do(t, http.MethodPut, srv.book+"/members/anna", roleJSON(Admin))
	wantStatus(t, promoted, http.StatusOK)
	if m := decode[Member](t, promoted); m.Role != Admin {
		t.Errorf("anna is %q", m.Role)
	}
	wantStatus(t, srv.do(t, http.MethodDelete, srv.book+"/members/dev", nil), http.StatusNoContent)

	wantStatus(t, srv.do(t, http.MethodGet, srv.book, nil), http.StatusNotFound)
	if got := strings.Join(memberNames(t, anna, srv.book), " "); got != "anna:admin" {
		t.Errorf("members = %s", got)
	}
}

func TestMembersCanLeaveAndBeRemoved(t *testing.T) {
	srv, _ := newTestServer(t)
	anna := srv.asUser(t, "anna")
	bob := srv.asUser(t, "bob")
	join(t, srv, srv.book, "anna", "editor")
	join(t, srv, srv.book, "bob", "editor")

	wantStatus(t, anna.do(t, http.MethodDelete, srv.book+"/members/anna", nil), http.StatusNoContent) // leaving
	wantStatus(t, srv.do(t, http.MethodDelete, srv.book+"/members/bob", nil), http.StatusNoContent)   // removed

	for _, gone := range []*contractServer{anna, bob} {
		wantStatus(t, gone.do(t, http.MethodGet, srv.book, nil), http.StatusNotFound)
	}
	wantStatus(t, srv.do(t, http.MethodDelete, srv.book+"/members/bob", nil), http.StatusNotFound)
	wantStatus(t, srv.do(t, http.MethodPut, srv.book+"/members/bob", roleJSON(Admin)), http.StatusNotFound)
	if got := strings.Join(memberNames(t, srv, srv.book), " "); got != "dev:admin" {
		t.Errorf("members = %s", got)
	}
}
