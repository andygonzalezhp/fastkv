package main

import (
	"flag"
	"log"

	"github.com/andygonzalezhp/fastkv/internal/server"
	"github.com/andygonzalezhp/fastkv/internal/store"
)

func main() {
	addr := flag.String("addr", ":6380", "TCP address for FastKV server")
	flag.Parse()

	kvStore := store.NewStore()
	srv := server.NewServer(*addr, kvStore)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
