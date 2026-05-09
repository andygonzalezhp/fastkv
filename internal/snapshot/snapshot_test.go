package snapshot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/andygonzalezhp/fastkv/internal/store"
)

func TestSaveAndLoadSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.snapshot")

	original := store.NewStore()
	original.Set("name", "andy")
	original.Set("language", "go")

	if err := Save(path, original); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	loaded := store.NewStore()

	if err := Load(path, loaded); err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	name, ok := loaded.Get("name")
	if !ok {
		t.Fatal("expected name key to exist after loading snapshot")
	}

	if name != "andy" {
		t.Fatalf("expected name value %q, got %q", "andy", name)
	}

	language, ok := loaded.Get("language")
	if !ok {
		t.Fatal("expected language key to exist after loading snapshot")
	}

	if language != "go" {
		t.Fatalf("expected language value %q, got %q", "go", language)
	}

	if loaded.Size() != 2 {
		t.Fatalf("expected loaded store size 2, got %d", loaded.Size())
	}
}

func TestSnapshotSkipsExpiredKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expired.snapshot")

	original := store.NewStore()
	original.Set("expired", "value")
	original.ExpireAt("expired", time.Now().Add(-1*time.Hour))

	if err := Save(path, original); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	loaded := store.NewStore()

	if err := Load(path, loaded); err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	if loaded.Size() != 0 {
		t.Fatalf("expected expired key to be skipped, got size %d", loaded.Size())
	}

	_, ok := loaded.Get("expired")
	if ok {
		t.Fatal("expected expired key to not exist after loading snapshot")
	}
}

func TestLoadMissingSnapshotDoesNotError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.snapshot")

	s := store.NewStore()

	if err := Load(path, s); err != nil {
		t.Fatalf("expected missing snapshot to not error, got %v", err)
	}

	if s.Size() != 0 {
		t.Fatalf("expected empty store, got size %d", s.Size())
	}
}
