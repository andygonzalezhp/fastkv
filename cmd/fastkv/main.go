package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
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
	maxKeys := flag.Int("max-keys", 0, "maximum number of keys before LRU eviction; 0 means unlimited")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kvStore := store.NewStoreWithMaxKeys(*maxKeys)

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

	kvStore.StartExpirationWorker(ctx, 1*time.Second)

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *addr, err)
	}

	srv := server.NewServer(*addr, kvStore, writeAheadLog, *snapshotPath)

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- srv.Serve(listener)
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("FastKV ready on %s\n", listener.Addr().String())

	select {
	case <-signalCtx.Done():
		log.Println("shutdown signal received")

		cancel()

		if err := listener.Close(); err != nil {
			log.Printf("failed to close listener: %v\n", err)
		}

		if err := writeAheadLog.Close(); err != nil {
			log.Printf("failed to close WAL: %v\n", err)
		}

		log.Println("FastKV shut down cleanly")

	case err := <-serverErrors:
		cancel()

		if closeErr := writeAheadLog.Close(); closeErr != nil {
			log.Printf("failed to close WAL: %v\n", closeErr)
		}

		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}
}
