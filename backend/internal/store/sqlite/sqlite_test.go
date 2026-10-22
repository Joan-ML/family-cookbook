package sqlite

import (
	"context"
	"os"
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

func TestBackupIsConsistentWhileTheDatabaseIsInUse(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now()
	r := store.Recipe{
		ID:            "r1",
		RecipeContent: store.RecipeContent{Name: "Soup", Tags: []string{"vegan"}, Steps: []string{"Boil."}},
		CreatedAt:     now, CreatedBy: "dev", UpdatedAt: now, UpdatedBy: "dev",
	}
	if err := s.CreateRecipe(ctx, r); err != nil {
		t.Fatal(err)
	}
	// The change is still in the WAL file, so copying cookbook.db alone would miss it.
	if fi, err := os.Stat(filepath.Join(dir, "live.db-wal")); err != nil || fi.Size() == 0 {
		t.Fatalf("expected pending changes in the WAL file (err %v)", err)
	}

	backup := filepath.Join(dir, "backups", "copy.db")
	if err := os.Mkdir(filepath.Dir(backup), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Backup(ctx, backup); err != nil {
		t.Fatalf("backup: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(backup))
	if len(entries) != 1 || entries[0].Name() != "copy.db" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("backup folder has %q, want only copy.db: one self-contained file, no temporary files", names)
	}

	restored, err := Open(backup)
	if err != nil {
		t.Fatalf("opening backup: %v", err)
	}
	defer restored.Close()
	got, err := restored.GetRecipe(ctx, "r1")
	if err != nil {
		t.Fatalf("recipe missing from backup: %v", err)
	}
	if got.Name != "Soup" || len(got.Tags) != 1 || got.Tags[0] != "vegan" {
		t.Errorf("backup has %+v", got)
	}
}

func TestBackupRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	existing := filepath.Join(dir, "existing.db")
	if err := os.WriteFile(existing, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Backup(context.Background(), existing); err == nil {
		t.Fatal("backup overwrote an existing file")
	}

	if b, _ := os.ReadFile(existing); string(b) != "keep me" {
		t.Errorf("existing file changed to %q", b)
	}
}
