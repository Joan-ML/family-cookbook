package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"family-cookbook/backend/internal/api"
	"family-cookbook/backend/internal/store/sqlite"
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

	log.Printf("listening on %s, data in %s", addr, dataDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
