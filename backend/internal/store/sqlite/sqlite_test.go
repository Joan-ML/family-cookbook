package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"family-cookbook/backend/internal/store"
)

func TestReopenKeepsDataAndSkipsAppliedMigrations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	r := store.Recipe{
		ID:            "r1",
		RecipeContent: store.RecipeContent{Name: "Soup", Tags: []string{"vegan"}},
		CreatedAt:     now, CreatedBy: "dev", UpdatedAt: now, UpdatedBy: "dev",
	}
	if err := s.CreateRecipe(ctx, r); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer s.Close()
	if _, err := s.GetRecipe(ctx, "r1"); err != nil {
		t.Fatalf("recipe lost after reopening: %v", err)
	}
}

func TestDeleteRecipeRemovesItsTags(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	r := store.Recipe{
		ID:            "r1",
		RecipeContent: store.RecipeContent{Name: "Soup", Tags: []string{"vegan", "quick"}},
		CreatedAt:     now, CreatedBy: "dev", UpdatedAt: now, UpdatedBy: "dev",
	}
	if err := s.CreateRecipe(ctx, r); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteRecipe(ctx, "r1"); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM recipe_tags`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d orphaned tags left after delete", n)
	}
}
