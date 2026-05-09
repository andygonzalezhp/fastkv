package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/server"
	"github.com/andygonzalezhp/fastkv/internal/snapshot"
	"github.com/andygonzalezhp/fastkv/internal/store"
	"github.com/andygonzalezhp/fastkv/internal/wal"
)

func main() {
	addr := flag.String("addr", ":6380", "TCP address for FastKV server")
	walPath := flag.String("wal", "data/fastkv.wal", "path to write-ahead log file")
	snapshotPath := flag.String("snapshot", "data/fastkv.snapshot", "path to snapshot file")
	syncPolicy := flag.String("sync-policy", "always", "WAL sync policy: always or none")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvStore := store.NewStore()

	if err := snapshot.Load(*snapshotPath, kvStore); err != nil {
		log.Fatalf("failed to load snapshot: %v", err)
	}

	if err := wal.Replay(*walPath, kvStore); err != nil {
		log.Fatalf("failed to replay WAL: %v", err)
	}

	writeAheadLog, err := wal.Open(*walPath, wal.SyncPolicy(*syncPolicy))
	if err != nil {
		log.Fatalf("failed to open WAL: %v", err)
	}
	defer writeAheadLog.Close()

	kvStore.StartExpirationWorker(ctx, 1*time.Second)

	srv := server.NewServer(*addr, kvStore, writeAheadLog, *snapshotPath)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
