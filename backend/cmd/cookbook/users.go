package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store"
	"family-cookbook/backend/internal/store/sqlite"
)

// openStore opens the database, creating it if needed, so the first account
// can be created before the server has ever run.
func openStore(dataDir string) (*sqlite.Store, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	return sqlite.Open(dbPath(dataDir))
}

func userAdd(dataDir, name string, stdout, stderr io.Writer) error {
	username, err := auth.NormalizeUsername(name)
	if err != nil {
		return err
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	// No password: nobody can log in until the setup link is used.
	if err := createUser(st, username, ""); err != nil {
		return err
	}
	token, expires, err := auth.NewService(st, time.Now).IssueSetupLink(context.Background(), username)
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "created user %s\n", username)
	printSetupLink(stdout, stderr, username, token, expires)
	return nil
}

func userAddWithPassword(dataDir, name string, stdin io.Reader, stderr io.Writer) error {
	username, err := auth.NormalizeUsername(name)
	if err != nil {
		return err
	}
	hash, err := readPassword(stdin)
	if err != nil {
		return err
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := createUser(st, username, hash); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "created user %s\n", username)
	return nil
}

func createUser(st *sqlite.Store, username, hash string) error {
	err := st.CreateUser(context.Background(), username, hash, time.Now())
	if errors.Is(err, store.ErrExists) {
		return fmt.Errorf("user %s already exists", username)
	}
	return err
}

func userReset(dataDir, name string, stdout, stderr io.Writer) error {
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	token, expires, err := auth.NewService(st, time.Now).ResetPassword(context.Background(), name)
	if errors.Is(err, auth.ErrNoSuchUser) {
		return fmt.Errorf("no user %s", name)
	}
	if err != nil {
		return err
	}
	username, _ := auth.NormalizeUsername(name)
	fmt.Fprintf(stderr, "disabled the password of %s and logged them out everywhere\n", username)
	printSetupLink(stdout, stderr, username, token, expires)
	return nil
}

// printSetupLink prints the link alone on stdout, so scripts can capture it,
// and how to use it on stderr.
func printSetupLink(stdout, stderr io.Writer, username, token string, expires time.Time) {
	path := "/set-password#" + token // after "#", the token never reaches server logs
	base := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")
	fmt.Fprintf(stderr, "send %s this link to choose a password; it works once, until %s:\n",
		username, expires.Local().Format("2006-01-02 15:04"))
	fmt.Fprintln(stdout, base+path)
	if base == "" {
		fmt.Fprintln(stderr, "(add it to the app's address, or set PUBLIC_URL to print complete links)")
	}
}

func userList(dataDir string, stdout io.Writer) error {
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	users, err := st.ListUsers(context.Background())
	if err != nil {
		return err
	}
	for _, u := range users {
		status := ""
		if u.PasswordHash == "" {
			status = "\tno password yet (setup link sent)"
		}
		fmt.Fprintf(stdout, "%s\tcreated %s%s\n", u.Username, u.CreatedAt.Local().Format("2006-01-02"), status)
	}
	return nil
}

func userRemove(dataDir, name string, stderr io.Writer) error {
	username, err := auth.NormalizeUsername(name)
	if err != nil {
		return err
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	removal, err := st.DeleteUser(context.Background(), username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no user %s", username)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "removed user %s and logged them out everywhere\n", username)
	for _, name := range removal.DeletedCookbooks {
		fmt.Fprintf(stderr, "deleted cookbook %q, which had no other members\n", name)
	}
	for _, name := range slices.Sorted(maps.Keys(removal.Promoted)) {
		fmt.Fprintf(stderr, "made %s an admin of %q, which had no other admin\n", removal.Promoted[name], name)
	}
	return nil
}

// readPassword reads a new password as one line from stdin and returns its
// hash.
func readPassword(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", errors.New("no password given on standard input")
	}
	password := strings.TrimRight(line, "\r\n")
	if err := auth.CheckNewPassword(password); err != nil {
		return "", err
	}
	return auth.HashPassword(password)
}
