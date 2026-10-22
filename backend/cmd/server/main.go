package main

import (
	"cmp"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"family-cookbook/backend/internal/api"
	"family-cookbook/backend/internal/store/sqlite"
	"family-cookbook/backend/internal/version"
	"family-cookbook/backend/internal/web"
)

func main() {
	addr := envOr("ADDR", ":8080")
	dataDir := envOr("DATA_DIR", "data")

	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		log.Fatal(err)
	}
	st, err := sqlite.Open(filepath.Join(dataDir, "cookbook.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewServer(st))
	if ui := web.Handler(); ui != nil {
		mux.Handle("/", ui)
	}

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("family cookbook %s (commit %s) listening on %s, data in %s",
			cmp.Or(version.Release(), "unreleased"), version.Commit(), addr, dataDir)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Finish in-flight requests on Ctrl+C or `docker stop` before closing the database.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
