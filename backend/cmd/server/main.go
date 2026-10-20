package main

import (
	"log"
	"net/http"
	"os"

	"family-cookbook/backend/internal/api"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, api.NewServer()); err != nil {
		log.Fatal(err)
	}
}
