// Command cookbook runs the Family Cookbook server and its admin commands.
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"family-cookbook/backend/internal/api"
	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store/sqlite"
	"family-cookbook/backend/internal/version"
	"family-cookbook/backend/internal/web"
)

const usage = `Usage: cookbook [command]

Commands:
  serve                   Run the web app (the default).
  backup <file>           Write a consistent backup of the database to <file>,
                          while the app keeps running. Use "-" for standard output.
  user add <username>     Create an account and print a setup link, with which
                          the user chooses their password (valid 48 hours).
  user reset <username>   Disable the user's password, log them out everywhere
                          and print a new setup link.
  user list               List accounts.
  user remove <username>  Delete an account and log it out everywhere.

  user add <username> --password-stdin
                          Create an account with the password read as one
                          line from standard input, for scripts and tests.

Environment:
  ADDR              Address to listen on (default ":8080").
  DATA_DIR          Directory holding the database (default "data").
  APP_VERSION       Release version to report, e.g. "v1.2.3".
  INSECURE_COOKIES  "true" to send the session cookie over plain HTTP when the
                    app isn't reached as localhost (not needed behind HTTPS).
  PUBLIC_URL        The app's address, e.g. "https://cookbook.example.com", so
                    setup links are printed in full.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes a command and returns the process exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	dataDir := envOr("DATA_DIR", "data")

	var err error
	switch {
	case cmd == "serve" && len(args) == 0:
		err = serve(envOr("ADDR", ":8080"), dataDir)
	case cmd == "backup" && len(args) == 1:
		err = backup(dataDir, args[0], stdout, stderr)
	case cmd == "user" && len(args) == 2 && args[0] == "add":
		err = userAdd(dataDir, args[1], stdout, stderr)
	case cmd == "user" && len(args) == 3 && args[0] == "add" && args[2] == "--password-stdin":
		err = userAddWithPassword(dataDir, args[1], stdin, stderr)
	case cmd == "user" && len(args) == 2 && args[0] == "reset":
		err = userReset(dataDir, args[1], stdout, stderr)
	case cmd == "user" && len(args) == 1 && args[0] == "list":
		err = userList(dataDir, stdout)
	case cmd == "user" && len(args) == 2 && args[0] == "remove":
		err = userRemove(dataDir, args[1], stderr)
	case cmd == "help" || cmd == "-h" || cmd == "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "cookbook %s: %v\n", cmd, err)
		return 1
	}
	return 0
}

func dbPath(dataDir string) string {
	return filepath.Join(dataDir, "cookbook.db")
}

func serve(addr, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return err
	}
	st, err := sqlite.Open(dbPath(dataDir))
	if err != nil {
		return err
	}
	defer st.Close()

	mux := http.NewServeMux()
	opts := api.Options{InsecureCookies: os.Getenv("INSECURE_COOKIES") == "true"}
	mux.Handle("/api/", api.NewServer(st, auth.NewService(st, time.Now), opts))
	if ui := web.Handler(); ui != nil {
		mux.Handle("/", ui)
	}

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("family cookbook %s (commit %s) listening on %s, data in %s",
			cmp.Or(version.Release(), "unreleased"), version.Commit(), addr, dataDir)
		serveErr <- srv.ListenAndServe()
	}()

	// Finish in-flight requests on Ctrl+C or `docker stop` before closing the database.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// backup writes a backup of the database to dest, or to stdout if dest is "-".
func backup(dataDir, dest string, stdout, stderr io.Writer) error {
	src := dbPath(dataDir)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("no database at %s: %w", src, err)
	}
	st, err := sqlite.Open(src)
	if err != nil {
		return err
	}
	defer st.Close()
	ctx := context.Background()

	if dest != "-" {
		if err := st.Backup(ctx, dest); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "backed up %s to %s\n", src, dest)
		return nil
	}

	// SQLite writes backups to a file, so stream one from a temporary directory
	// next to the database (always writable, even in the container).
	tmpDir, err := os.MkdirTemp(dataDir, ".backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tmp := filepath.Join(tmpDir, "cookbook.db")
	if err := st.Backup(ctx, tmp); err != nil {
		return err
	}
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(stdout, f)
	return err
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
