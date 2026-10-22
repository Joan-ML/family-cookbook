package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

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

func userAdd(dataDir, name string, stdin io.Reader, stderr io.Writer) error {
	username, err := auth.NormalizeUsername(name)
	if err != nil {
		return err
	}
	hash, err := readNewPassword(username, stdin, stderr)
	if err != nil {
		return err
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	err = st.CreateUser(context.Background(), username, hash, time.Now())
	if errors.Is(err, store.ErrExists) {
		return fmt.Errorf("user %s already exists", username)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "created user %s\n", username)
	return nil
}

func userPasswd(dataDir, name string, stdin io.Reader, stderr io.Writer) error {
	username, err := auth.NormalizeUsername(name)
	if err != nil {
		return err
	}
	st, err := openStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	// Check first, so nobody types a password for a user that doesn't exist.
	if _, err := st.GetUser(context.Background(), username); errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no user %s", username)
	}
	hash, err := readNewPassword(username, stdin, stderr)
	if err != nil {
		return err
	}
	if err := st.SetPassword(context.Background(), username, hash); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "changed the password of %s and logged them out everywhere\n", username)
	return nil
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
		fmt.Fprintf(stdout, "%s\tcreated %s\n", u.Username, u.CreatedAt.Local().Format("2006-01-02"))
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
	err = st.DeleteUser(context.Background(), username)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("no user %s", username)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stderr, "removed user %s and logged them out everywhere\n", username)
	return nil
}

// readNewPassword asks for a password and returns its hash. On a terminal it
// asks twice without echo; otherwise it reads one line from stdin.
func readNewPassword(username string, stdin io.Reader, stderr io.Writer) (string, error) {
	var password string
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprintf(stderr, "New password for %s: ", username)
		first, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(stderr, "Again: ")
		second, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(second) {
			return "", errors.New("the passwords don't match")
		}
		password = string(first)
	} else {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !(errors.Is(err, io.EOF) && line != "") {
			return "", errors.New("no password given on standard input")
		}
		password = strings.TrimRight(line, "\r\n")
	}
	if err := auth.CheckNewPassword(password); err != nil {
		return "", err
	}
	return auth.HashPassword(password)
}
