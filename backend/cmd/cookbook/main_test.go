package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"family-cookbook/backend/internal/store"
	"family-cookbook/backend/internal/store/sqlite"
)

// newDataDir returns a data directory holding a database with one recipe.
func newDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	st, err := sqlite.Open(dbPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	err = st.CreateRecipe(context.Background(), store.Recipe{
		ID:            "r1",
		RecipeContent: store.RecipeContent{Name: "Soup"},
		CreatedAt:     now, CreatedBy: "dev", UpdatedAt: now, UpdatedBy: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// assertHasSoup fails unless the database file at path contains the recipe.
func assertHasSoup(t *testing.T, path string) {
	t.Helper()
	st, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("opening backup: %v", err)
	}
	defer st.Close()
	if r, err := st.GetRecipe(context.Background(), "r1"); err != nil || r.Name != "Soup" {
		t.Fatalf("backup is missing the recipe: %v", err)
	}
}

func TestBackupToFile(t *testing.T) {
	t.Setenv("DATA_DIR", newDataDir(t))
	dest := filepath.Join(t.TempDir(), "backup.db")
	var stdout, stderr bytes.Buffer

	code := run([]string{"backup", dest}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code %d; stderr: %s", code, &stderr)
	}
	assertHasSoup(t, dest)
}

func TestBackupToStdout(t *testing.T) {
	dataDir := newDataDir(t)
	t.Setenv("DATA_DIR", dataDir)
	var stdout, stderr bytes.Buffer

	code := run([]string{"backup", "-"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code %d; stderr: %s", code, &stderr)
	}
	if !bytes.HasPrefix(stdout.Bytes(), []byte("SQLite format 3\x00")) {
		t.Fatalf("stdout is not a SQLite database (%d bytes)", stdout.Len())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr, which would mix with the backup in some shells: %s", &stderr)
	}
	dest := filepath.Join(t.TempDir(), "from-stdout.db")
	if err := os.WriteFile(dest, stdout.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	assertHasSoup(t, dest)
	// The temporary copy is cleaned up.
	if matches, _ := filepath.Glob(filepath.Join(dataDir, ".backup-*")); len(matches) > 0 {
		t.Errorf("left behind %v", matches)
	}
}

func TestBackupWithoutDatabaseFails(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	var stdout, stderr bytes.Buffer

	code := run([]string{"backup", filepath.Join(t.TempDir(), "x.db")}, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "no database") {
		t.Errorf("exit code %d, stderr %q; want 1 and a clear message", code, &stderr)
	}
	// It must not create an empty database as a side effect.
	if _, err := os.Stat(dbPath(dataDir)); err == nil {
		t.Error("backup created a database")
	}
}

func TestUsage(t *testing.T) {
	tests := []struct {
		args []string
		code int
	}{
		{[]string{"help"}, 0},
		{[]string{"backup"}, 2},
		{[]string{"backup", "a", "b"}, 2},
		{[]string{"serve", "extra"}, 2},
		{[]string{"nope"}, 2},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)

			if code != tt.code {
				t.Errorf("exit code %d, want %d", code, tt.code)
			}
			if !strings.Contains(stdout.String()+stderr.String(), "Usage: cookbook") {
				t.Error("usage not printed")
			}
		})
	}
}
