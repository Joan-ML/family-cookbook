package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func pancakes() RecipeInput {
	return RecipeInput{
		Name:        "Pancakes",
		Ingredients: []string{"200 g flour", "2 eggs", "300 ml milk"},
		Steps:       []string{"Whisk everything together.", "Fry in a hot pan."},
		Tags:        []string{"breakfast", "vegetarian"},
		Source:      ptr("Grandma"),
	}
}

func createRecipe(t *testing.T, srv *contractServer, in RecipeInput) Recipe {
	t.Helper()
	res := srv.do(t, http.MethodPost, "/api/recipes", mustJSON(t, in))
	wantStatus(t, res, http.StatusCreated)
	return decode[Recipe](t, res)
}

func TestCreateAndGetRecipe(t *testing.T) {
	srv, _ := newTestServer(t)

	res := srv.do(t, http.MethodPost, "/api/recipes", mustJSON(t, pancakes()))

	wantStatus(t, res, http.StatusCreated)
	created := decode[Recipe](t, res)
	if got, want := res.Header.Get("Location"), "/api/recipes/"+created.Id; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	want := Recipe{
		Id:          created.Id,
		Name:        "Pancakes",
		Ingredients: []string{"200 g flour", "2 eggs", "300 ml milk"},
		Steps:       []string{"Whisk everything together.", "Fry in a hot pan."},
		Tags:        []string{"breakfast", "vegetarian"},
		Source:      ptr("Grandma"),
		CreatedAt:   testNow,
		CreatedBy:   testUser,
		UpdatedAt:   testNow,
		UpdatedBy:   testUser,
	}
	assertRecipe(t, created, want)

	res = srv.do(t, http.MethodGet, "/api/recipes/"+created.Id, nil)
	wantStatus(t, res, http.StatusOK)
	assertRecipe(t, decode[Recipe](t, res), want)
}

func TestCreateRecipeNormalizesInput(t *testing.T) {
	srv, _ := newTestServer(t)

	got := createRecipe(t, srv, RecipeInput{
		Name:        "  Soup ",
		Ingredients: []string{" water ", "   ", "salt"},
		Steps:       []string{"Boil."},
		Tags:        []string{"Vegan", " vegan", "QUICK", "  "},
		Source:      ptr("  "),
	})

	if got.Name != "Soup" {
		t.Errorf("name = %q, want %q", got.Name, "Soup")
	}
	if want := []string{"water", "salt"}; !slices.Equal(got.Ingredients, want) {
		t.Errorf("ingredients = %q, want %q", got.Ingredients, want)
	}
	if want := []string{"vegan", "quick"}; !slices.Equal(got.Tags, want) {
		t.Errorf("tags = %q, want %q", got.Tags, want)
	}
	if got.Source != nil {
		t.Errorf("source = %q, want none", *got.Source)
	}
}

func TestCreateRecipeAllowsEmptyLists(t *testing.T) {
	srv, _ := newTestServer(t)

	got := createRecipe(t, srv, RecipeInput{
		Name: "Idea", Ingredients: []string{}, Steps: []string{}, Tags: []string{},
	})

	if got.Ingredients == nil || got.Steps == nil || got.Tags == nil {
		t.Errorf("lists must be empty arrays, not null: %+v", got)
	}
}

func TestCreateRecipeRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"name":`},
		{"missing name", `{"ingredients":[],"steps":[],"tags":[]}`},
		{"missing lists", `{"name":"Soup"}`},
		{"empty name", `{"name":"","ingredients":[],"steps":[],"tags":[]}`},
		{"name too long", `{"name":"` + strings.Repeat("a", 201) + `","ingredients":[],"steps":[],"tags":[]}`},
		{"wrong type", `{"name":"Soup","ingredients":"water","steps":[],"tags":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newTestServer(t)

			res := srv.doInvalid(t, http.MethodPost, "/api/recipes", []byte(tt.body))

			wantStatus(t, res, http.StatusBadRequest)
		})
	}
}

func TestCreateRecipeRejectsBlankName(t *testing.T) {
	srv, _ := newTestServer(t)
	in := pancakes()
	in.Name = "   " // valid per the spec's minLength, but blank once trimmed

	res := srv.do(t, http.MethodPost, "/api/recipes", mustJSON(t, in))

	wantStatus(t, res, http.StatusBadRequest)
}

func TestListRecipesSortedByName(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, name := range []string{"tortilla", "Pancakes", "apple pie"} {
		in := pancakes()
		in.Name = name
		createRecipe(t, srv, in)
	}

	res := srv.do(t, http.MethodGet, "/api/recipes", nil)

	wantStatus(t, res, http.StatusOK)
	var names []string
	for _, item := range decode[RecipeList](t, res).Items {
		names = append(names, item.Name)
	}
	if want := []string{"apple pie", "Pancakes", "tortilla"}; !slices.Equal(names, want) {
		t.Errorf("names = %q, want %q", names, want)
	}
}

func TestListRecipesEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	res := srv.do(t, http.MethodGet, "/api/recipes", nil)

	wantStatus(t, res, http.StatusOK)
	if items := decode[RecipeList](t, res).Items; items == nil || len(items) != 0 {
		t.Errorf("items = %v, want empty array", items)
	}
}

func TestUpdateRecipe(t *testing.T) {
	srv, s := newTestServer(t)
	created := createRecipe(t, srv, pancakes())
	tick(s, time.Hour)

	in := pancakes()
	in.Name = "Crêpes"
	in.Tags = []string{"french"}
	in.Source = nil
	res := srv.do(t, http.MethodPut, "/api/recipes/"+created.Id, mustJSON(t, in))

	wantStatus(t, res, http.StatusOK)
	want := created
	want.Name = "Crêpes"
	want.Tags = []string{"french"}
	want.Source = nil
	want.UpdatedAt = testNow.Add(time.Hour)
	assertRecipe(t, decode[Recipe](t, res), want)

	res = srv.do(t, http.MethodGet, "/api/recipes/"+created.Id, nil)
	assertRecipe(t, decode[Recipe](t, res), want)
}

func TestDeleteRecipe(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createRecipe(t, srv, pancakes())

	res := srv.do(t, http.MethodDelete, "/api/recipes/"+created.Id, nil)

	wantStatus(t, res, http.StatusNoContent)
	res = srv.do(t, http.MethodGet, "/api/recipes/"+created.Id, nil)
	wantStatus(t, res, http.StatusNotFound)
}

func TestMissingRecipeReturns404(t *testing.T) {
	srv, _ := newTestServer(t)
	body := mustJSON(t, pancakes())

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var b []byte
			if method == http.MethodPut {
				b = body
			}

			res := srv.do(t, method, "/api/recipes/missing", b)

			wantStatus(t, res, http.StatusNotFound)
		})
	}
}

func assertRecipe(t *testing.T, got, want Recipe) {
	t.Helper()
	if got.Id != want.Id || got.Name != want.Name ||
		!slices.Equal(got.Ingredients, want.Ingredients) ||
		!slices.Equal(got.Steps, want.Steps) ||
		!slices.Equal(got.Tags, want.Tags) ||
		!equalPtr(got.Source, want.Source) ||
		!got.CreatedAt.Equal(want.CreatedAt) || got.CreatedBy != want.CreatedBy ||
		!got.UpdatedAt.Equal(want.UpdatedAt) || got.UpdatedBy != want.UpdatedBy {
		t.Errorf("recipe mismatch\n got: %s\nwant: %s", mustJSON(t, got), mustJSON(t, want))
	}
}

func equalPtr[T comparable](a, b *T) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
