package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store"
	"family-cookbook/backend/internal/store/sqlite"
)

// cli runs a command with stdin as its input, failing the test on an
// unexpected exit code. It returns stdout and stderr.
func cli(t *testing.T, wantCode int, stdin string, args ...string) (string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := run(args, strings.NewReader(stdin), &stdout, &stderr); code != wantCode {
		t.Fatalf("cookbook %s: exit code %d, want %d; stderr: %s", strings.Join(args, " "), code, wantCode, &stderr)
	}
	return stdout.String(), stderr.String()
}

// logIn tries a password against the data directory's database.
func logIn(t *testing.T, dataDir, username, password string) error {
	t.Helper()
	st, err := sqlite.Open(dbPath(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_, _, err = auth.NewService(st, st, time.Now).LogIn(context.Background(), username, password)
	return err
}

func TestUserCommands(t *testing.T) {
	dataDir := t.TempDir() // no database yet: user add creates it
	t.Setenv("DATA_DIR", dataDir)

	_, stderr := cli(t, 0, "first password\n", "user", "add", "Joan")
	if !strings.Contains(stderr, "created user joan") {
		t.Errorf("stderr = %q", stderr)
	}
	cli(t, 0, "second password", "user", "add", "anna") // no trailing newline is fine
	if err := logIn(t, dataDir, "joan", "first password"); err != nil {
		t.Fatalf("can't log in with the new account: %v", err)
	}

	stdout, _ := cli(t, 0, "", "user", "list")
	if lines := strings.Split(strings.TrimSpace(stdout), "\n"); len(lines) != 2 ||
		!strings.HasPrefix(lines[0], "anna\t") || !strings.HasPrefix(lines[1], "joan\t") {
		t.Errorf("user list = %q", stdout)
	}

	cli(t, 0, "a new password\n", "user", "passwd", "joan")
	if err := logIn(t, dataDir, "joan", "first password"); err == nil {
		t.Error("the old password still works")
	}
	if err := logIn(t, dataDir, "joan", "a new password"); err != nil {
		t.Errorf("the new password doesn't work: %v", err)
	}

	cli(t, 0, "", "user", "remove", "anna")
	if stdout, _ := cli(t, 0, "", "user", "list"); strings.Contains(stdout, "anna") {
		t.Errorf("anna still listed: %q", stdout)
	}
}

func TestUserCommandErrors(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	cli(t, 0, "first password\n", "user", "add", "joan")

	tests := []struct {
		name  string
		stdin string
		args  []string
		want  string
	}{
		{"duplicate", "another password\n", []string{"user", "add", "JOAN"}, "already exists"},
		{"short password", "short\n", []string{"user", "add", "anna"}, "at least 10 characters"},
		{"no password", "", []string{"user", "add", "anna"}, "no password"},
		{"bad username", "first password\n", []string{"user", "add", "not valid!"}, "usernames are"},
		{"passwd unknown", "a new password\n", []string{"user", "passwd", "nobody"}, "no user nobody"},
		{"remove unknown", "", []string{"user", "remove", "nobody"}, "no user nobody"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := cli(t, 1, tt.stdin, tt.args...)
			if !strings.Contains(stderr, tt.want) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, tt.want)
			}
		})
	}
	if stdout, _ := cli(t, 0, "", "user", "list"); strings.Contains(stdout, "anna") {
		t.Errorf("a failed add created a user: %q", stdout)
	}
}

func TestUserRemoveReportsCookbookChanges(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	cli(t, 0, "first password\n", "user", "add", "joan")
	cli(t, 0, "second password\n", "user", "add", "anna")
	st, err := sqlite.Open(dbPath(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	joan, _ := st.GetUser(ctx, "joan")
	anna, _ := st.GetUser(ctx, "anna")
	now := time.Now()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(st.CreateCookbook(ctx, store.Cookbook{ID: "c1", Name: "Just mine", CreatedAt: now, CreatedBy: "joan"}, joan.ID))
	must(st.CreateCookbook(ctx, store.Cookbook{ID: "c2", Name: "Family", CreatedAt: now, CreatedBy: "joan"}, joan.ID))
	must(st.AddMember(ctx, "c2", anna.ID, store.RoleEditor, now))
	st.Close()

	_, stderr := cli(t, 0, "", "user", "remove", "joan")

	for _, want := range []string{
		`deleted cookbook "Just mine", which had no other members`,
		`made anna an admin of "Family", which had no other admin`,
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q lacks %q", stderr, want)
		}
	}
}
