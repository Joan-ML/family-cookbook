package sqlite

import (
	"context"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"family-cookbook/backend/internal/store"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func addRecipe(t *testing.T, s *Store, cookbookID, id, name string) {
	t.Helper()
	now := time.Now()
	err := s.CreateRecipe(context.Background(), store.Recipe{
		ID: id, CookbookID: cookbookID, RecipeContent: store.RecipeContent{Name: name},
		CreatedAt: now, CreatedBy: "owner", UpdatedAt: now, UpdatedBy: "owner",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCookbooksAreOnlyVisibleToMembers(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	owner := addUser(t, s, "owner")
	other := addUser(t, s, "other")
	addCookbook(t, s, "c1", "Family", owner)
	addRecipe(t, s, "c1", "r1", "Soup")

	c, err := s.GetCookbook(ctx, "c1", owner)
	if err != nil || c.Name != "Family" || c.Role != store.RoleAdmin {
		t.Fatalf("owner sees %+v, %v", c, err)
	}
	if _, err := s.GetCookbook(ctx, "c1", other); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("non-member: err = %v, want ErrNotFound", err)
	}
	if list, _ := s.ListCookbooks(ctx, other); len(list) != 0 {
		t.Errorf("non-member lists %+v", list)
	}

	if err := s.AddMember(ctx, "c1", other, store.RoleEditor, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListCookbooks(ctx, other)
	if err != nil || len(list) != 1 || list[0].Role != store.RoleEditor || list[0].RecipeCount != 1 {
		t.Errorf("member lists %+v, %v", list, err)
	}
	if err := s.AddMember(ctx, "c1", other, store.RoleEditor, time.Now()); !errors.Is(err, store.ErrExists) {
		t.Errorf("adding twice: err = %v, want ErrExists", err)
	}
}

func TestRecipesAreScopedToTheirCookbook(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	owner := addUser(t, s, "owner")
	addCookbook(t, s, "c1", "Family", owner)
	addCookbook(t, s, "c2", "Friends", owner)
	addRecipe(t, s, "c1", "r1", "Soup")

	if _, err := s.GetRecipe(ctx, "c2", "r1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get through the wrong cookbook: err = %v", err)
	}
	if _, err := s.UpdateRecipe(ctx, "c2", "r1", store.RecipeContent{Name: "X"}, "owner", time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("update through the wrong cookbook: err = %v", err)
	}
	if err := s.DeleteRecipe(ctx, "c2", "r1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("delete through the wrong cookbook: err = %v", err)
	}
	if list, _ := s.ListRecipes(ctx, "c2"); len(list) != 0 {
		t.Errorf("c2 lists %+v", list)
	}
	if r, err := s.GetRecipe(ctx, "c1", "r1"); err != nil || r.Name != "Soup" {
		t.Errorf("the recipe changed: %+v, %v", r, err)
	}
}

func TestDeleteCookbookDeletesItsRecipes(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	owner := addUser(t, s, "owner")
	addCookbook(t, s, "c1", "Family", owner)
	addRecipe(t, s, "c1", "r1", "Soup")

	if err := s.DeleteCookbook(ctx, "c1"); err != nil {
		t.Fatal(err)
	}

	for table, want := range map[string]int{"recipes": 0, "recipe_tags": 0, "memberships": 0} {
		var n int
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s has %d rows left", table, n)
		}
	}
	if err := s.DeleteCookbook(ctx, "c1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleting again: err = %v", err)
	}
}

func TestDeleteUserNeverLeavesACookbookWithoutAdmin(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	gone := addUser(t, s, "gone")
	early := addUser(t, s, "early")
	late := addUser(t, s, "late")
	admin := addUser(t, s, "admin")
	t0 := time.Now()

	// Only member: deleted with its recipes.
	addCookbook(t, s, "solo", "Solo", gone)
	addRecipe(t, s, "solo", "r1", "Soup")
	// Last admin, others remain: the longest-standing member is promoted.
	addCookbook(t, s, "shared", "Shared", gone)
	must(t, s.AddMember(ctx, "shared", late, store.RoleEditor, t0.Add(2*time.Hour)))
	must(t, s.AddMember(ctx, "shared", early, store.RoleEditor, t0.Add(time.Hour)))
	// Another admin remains: nothing changes but the membership.
	addCookbook(t, s, "co", "Co-run", admin)
	must(t, s.AddMember(ctx, "co", gone, store.RoleAdmin, t0))
	must(t, s.AddMember(ctx, "co", late, store.RoleEditor, t0))

	removal, err := s.DeleteUser(ctx, "gone")
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(removal.DeletedCookbooks, []string{"Solo"}) {
		t.Errorf("deleted %q, want only Solo", removal.DeletedCookbooks)
	}
	if want := map[string]string{"Shared": "early"}; !maps.Equal(removal.Promoted, want) {
		t.Errorf("promoted %v, want %v", removal.Promoted, want)
	}
	if c, err := s.GetCookbook(ctx, "shared", early); err != nil || c.Role != store.RoleAdmin {
		t.Errorf("early in Shared: %+v, %v; want admin", c, err)
	}
	if c, _ := s.GetCookbook(ctx, "shared", late); c.Role != store.RoleEditor {
		t.Errorf("late in Shared is %q, want still editor", c.Role)
	}
	if c, _ := s.GetCookbook(ctx, "co", late); c.Role != store.RoleEditor {
		t.Errorf("late in Co-run is %q: nobody should be promoted while an admin remains", c.Role)
	}
	if _, err := s.GetCookbook(ctx, "solo", gone); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Solo still exists: %v", err)
	}
	var recipes int
	must(t, s.db.QueryRowContext(ctx, `SELECT count(*) FROM recipes`).Scan(&recipes))
	if recipes != 0 {
		t.Errorf("%d recipes of the deleted cookbook remain", recipes)
	}
	if _, err := s.DeleteUser(ctx, "gone"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleting again: err = %v", err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
