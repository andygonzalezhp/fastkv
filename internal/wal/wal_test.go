package wal

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/store"
)

func TestWALReplayReconstructsStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.wal")

	w, err := Open(path, SyncAlways)
	if err != nil {
		t.Fatalf("failed to open WAL: %v", err)
	}

	lines := []string{
		"SET name andy",
		"SET language go",
		"DEL name",
	}

	for _, line := range lines {
		if err := w.Append(line); err != nil {
			t.Fatalf("failed to append WAL line %q: %v", line, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close WAL: %v", err)
	}

	s := store.NewStore()

	if err := Replay(path, s); err != nil {
		t.Fatalf("failed to replay WAL: %v", err)
	}

	if _, ok := s.Get("name"); ok {
		t.Fatal("expected name key to be deleted after WAL replay")
	}

	value, ok := s.Get("language")
	if !ok {
		t.Fatal("expected language key to exist after WAL replay")
	}

	if value != "go" {
		t.Fatalf("expected value %q, got %q", "go", value)
	}
}

func TestWALReplayWithExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-expiry.wal")

	w, err := Open(path, SyncAlways)
	if err != nil {
		t.Fatalf("failed to open WAL: %v", err)
	}

	expiresAt := time.Now().Add(1 * time.Hour)

	lines := []string{
		"SET temp hello",
		fmt.Sprintf("EXPIREAT temp %d", expiresAt.UnixNano()),
	}

	for _, line := range lines {
		if err := w.Append(line); err != nil {
			t.Fatalf("failed to append WAL line %q: %v", line, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close WAL: %v", err)
	}

	s := store.NewStore()

	if err := Replay(path, s); err != nil {
		t.Fatalf("failed to replay WAL: %v", err)
	}

	value, ok := s.Get("temp")
	if !ok {
		t.Fatal("expected temp key to exist after WAL replay")
	}

	if value != "hello" {
		t.Fatalf("expected value %q, got %q", "hello", value)
	}

	ttl, ok := s.TTL("temp")
	if !ok {
		t.Fatal("expected temp key to have TTL")
	}

	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %d", ttl)
	}
}

func TestWALReplayDeletesExpiredKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test-expired.wal")

	w, err := Open(path, SyncAlways)
	if err != nil {
		t.Fatalf("failed to open WAL: %v", err)
	}

	expiresAt := time.Now().Add(-1 * time.Hour)

	lines := []string{
		"SET old value",
		fmt.Sprintf("EXPIREAT old %d", expiresAt.UnixNano()),
	}

	for _, line := range lines {
		if err := w.Append(line); err != nil {
			t.Fatalf("failed to append WAL line %q: %v", line, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close WAL: %v", err)
	}

	s := store.NewStore()

	if err := Replay(path, s); err != nil {
		t.Fatalf("failed to replay WAL: %v", err)
	}

	_, ok := s.Get("old")
	if ok {
		t.Fatal("expected expired key to be removed during WAL replay")
	}
}
