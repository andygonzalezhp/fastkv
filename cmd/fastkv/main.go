package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/server"
	"github.com/andygonzalezhp/fastkv/internal/store"
)

func main() {
	addr := flag.String("addr", ":6380", "TCP address for FastKV server")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvStore := store.NewStore()
	kvStore.StartExpirationWorker(ctx, 1*time.Second)

	srv := server.NewServer(*addr, kvStore)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
