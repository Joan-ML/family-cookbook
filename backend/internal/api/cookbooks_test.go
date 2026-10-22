package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"family-cookbook/backend/internal/store"
)

func cookbookJSON(name string) []byte { return []byte(`{"name":"` + name + `"}`) }

// join adds username to the cookbook at path (an API path) with role.
func join(t *testing.T, srv *contractServer, path, username string, role store.Role) {
	t.Helper()
	u, err := srv.db.GetUser(t.Context(), username)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimPrefix(path, "/api/cookbooks/")
	if err := srv.db.AddMember(t.Context(), id, u.ID, role, testNow); err != nil {
		t.Fatal(err)
	}
}

func TestCreateGetAndListCookbooks(t *testing.T) {
	srv, _ := newTestServer(t) // already has "Test kitchen"

	res := srv.do(t, http.MethodPost, "/api/cookbooks", cookbookJSON("  Family  "))

	wantStatus(t, res, http.StatusCreated)
	created := decode[Cookbook](t, res)
	if created.Name != "Family" || created.Role != Admin || created.CreatedBy != testUser {
		t.Errorf("created %+v", created)
	}
	if got, want := res.Header.Get("Location"), "/api/cookbooks/"+created.Id; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	got := decode[Cookbook](t, srv.do(t, http.MethodGet, "/api/cookbooks/"+created.Id, nil))
	if got != created {
		t.Errorf("get = %+v, want %+v", got, created)
	}

	createRecipe(t, srv, pancakes()) // in Test kitchen
	list := decode[CookbookList](t, srv.do(t, http.MethodGet, "/api/cookbooks", nil)).Items
	want := []CookbookSummary{
		{Id: created.Id, Name: "Family", Role: Admin, RecipeCount: 0},
		{Id: strings.TrimPrefix(srv.book, "/api/cookbooks/"), Name: "Test kitchen", Role: Admin, RecipeCount: 1},
	}
	if len(list) != 2 || list[0] != want[0] || list[1] != want[1] {
		t.Errorf("list = %+v, want %+v", list, want)
	}
}

func TestCookbookNamesAreValidated(t *testing.T) {
	srv, _ := newTestServer(t)

	wantStatus(t, srv.do(t, http.MethodPost, "/api/cookbooks", cookbookJSON("   ")), http.StatusBadRequest)
	wantStatus(t, srv.doInvalid(t, http.MethodPost, "/api/cookbooks", cookbookJSON(strings.Repeat("a", 101))), http.StatusBadRequest)
	wantStatus(t, srv.do(t, http.MethodPut, srv.book, cookbookJSON("   ")), http.StatusBadRequest)
}

// Non-members get exactly what they'd get for a cookbook that doesn't exist.
func TestCookbooksAreInvisibleToNonMembers(t *testing.T) {
	srv, _ := newTestServer(t)
	recipe := createRecipe(t, srv, pancakes())
	stranger := srv.asUser(t, "stranger")
	missing := "/api/cookbooks/no-such-cookbook"

	if items := decode[CookbookList](t, stranger.do(t, http.MethodGet, "/api/cookbooks", nil)).Items; len(items) != 0 {
		t.Errorf("stranger lists %+v", items)
	}
	body := mustJSON(t, pancakes())
	for _, tt := range []struct {
		method, suffix string
		body           []byte
	}{
		{http.MethodGet, "", nil},
		{http.MethodPut, "", cookbookJSON("Mine now")},
		{http.MethodDelete, "", nil},
		{http.MethodGet, "/recipes", nil},
		{http.MethodPost, "/recipes", body},
		{http.MethodGet, "/recipes/" + recipe.Id, nil},
		{http.MethodPut, "/recipes/" + recipe.Id, body},
		{http.MethodDelete, "/recipes/" + recipe.Id, nil},
	} {
		t.Run(tt.method+" "+tt.suffix, func(t *testing.T) {
			res := stranger.do(t, tt.method, srv.book+tt.suffix, tt.body)
			ref := stranger.do(t, tt.method, missing+tt.suffix, tt.body)

			wantStatus(t, res, http.StatusNotFound)
			wantStatus(t, ref, http.StatusNotFound)
			got, _ := io.ReadAll(res.Body)
			want, _ := io.ReadAll(ref.Body)
			if string(got) != string(want) {
				t.Errorf("body %s differs from a missing cookbook's %s", got, want)
			}
		})
	}

	// Nothing changed for the members.
	if c := decode[Cookbook](t, srv.do(t, http.MethodGet, srv.book, nil)); c.Name != "Test kitchen" {
		t.Errorf("cookbook renamed to %q", c.Name)
	}
	wantStatus(t, srv.do(t, http.MethodGet, srv.book+"/recipes/"+recipe.Id, nil), http.StatusOK)
}

func TestRecipesCantBeReachedThroughAnotherCookbook(t *testing.T) {
	srv, _ := newTestServer(t)
	recipe := createRecipe(t, srv, pancakes())
	other := srv.newCookbook(t, "Other") // same user is a member of both

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		var body []byte
		if method == http.MethodPut {
			body = mustJSON(t, pancakes())
		}
		res := srv.do(t, method, other+"/recipes/"+recipe.Id, body)
		wantStatus(t, res, http.StatusNotFound)
		if msg := decode[Error](t, res).Message; msg != "recipe not found" {
			t.Errorf("%s: message %q", method, msg)
		}
	}
	wantStatus(t, srv.do(t, http.MethodGet, srv.book+"/recipes/"+recipe.Id, nil), http.StatusOK)
}

func TestEditorsEditRecipesButOnlyAdminsManageTheCookbook(t *testing.T) {
	srv, _ := newTestServer(t)
	editor := srv.asUser(t, "editor")
	join(t, srv, srv.book, "editor", store.RoleEditor)

	if c := decode[Cookbook](t, editor.do(t, http.MethodGet, srv.book, nil)); c.Role != Editor {
		t.Errorf("editor sees role %q", c.Role)
	}
	created := decode[Recipe](t, editor.do(t, http.MethodPost, srv.book+"/recipes", mustJSON(t, pancakes())))
	if created.CreatedBy != "editor" {
		t.Errorf("createdBy %q", created.CreatedBy)
	}
	wantStatus(t, editor.do(t, http.MethodDelete, srv.book+"/recipes/"+created.Id, nil), http.StatusNoContent)

	for _, res := range []*http.Response{
		editor.do(t, http.MethodPut, srv.book, cookbookJSON("Renamed")),
		editor.do(t, http.MethodDelete, srv.book, nil),
	} {
		wantStatus(t, res, http.StatusForbidden)
	}

	renamed := decode[Cookbook](t, srv.do(t, http.MethodPut, srv.book, cookbookJSON("Renamed")))
	if renamed.Name != "Renamed" || renamed.Role != Admin {
		t.Errorf("renamed %+v", renamed)
	}
}

func TestDeletingACookbookDeletesItsRecipes(t *testing.T) {
	srv, _ := newTestServer(t)
	recipe := createRecipe(t, srv, pancakes())
	keep := srv.newCookbook(t, "Keep")

	wantStatus(t, srv.do(t, http.MethodDelete, srv.book, nil), http.StatusNoContent)

	wantStatus(t, srv.do(t, http.MethodGet, srv.book, nil), http.StatusNotFound)
	wantStatus(t, srv.do(t, http.MethodGet, srv.book+"/recipes/"+recipe.Id, nil), http.StatusNotFound)
	list := decode[CookbookList](t, srv.do(t, http.MethodGet, "/api/cookbooks", nil)).Items
	if len(list) != 1 || "/api/cookbooks/"+list[0].Id != keep {
		t.Errorf("list = %+v, want only Keep", list)
	}
}
