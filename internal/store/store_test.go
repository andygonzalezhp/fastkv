package store

import (
	"context"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	s := NewStore()

	s.Set("name", "andy")

	value, ok := s.Get("name")
	if !ok {
		t.Fatal("expected key to exist")
	}

	if value != "andy" {
		t.Fatalf("expected value %q, got %q", "andy", value)
	}
}

func TestGetMissingKey(t *testing.T) {
	s := NewStore()

	_, ok := s.Get("missing")
	if ok {
		t.Fatal("expected missing key to not exist")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()

	s.Set("name", "andy")

	deleted := s.Delete("name")
	if !deleted {
		t.Fatal("expected delete to return true")
	}

	_, ok := s.Get("name")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestDeleteMissingKey(t *testing.T) {
	s := NewStore()

	deleted := s.Delete("missing")
	if deleted {
		t.Fatal("expected delete to return false for missing key")
	}
}

func TestExpire(t *testing.T) {
	s := NewStore()

	s.Set("temp", "hello")

	ok := s.Expire("temp", 1)
	if !ok {
		t.Fatal("expected expire to return true")
	}

	time.Sleep(1100 * time.Millisecond)

	_, exists := s.Get("temp")
	if exists {
		t.Fatal("expected key to expire")
	}
}

func TestTTLNoExpiry(t *testing.T) {
	s := NewStore()

	s.Set("name", "andy")

	ttl, ok := s.TTL("name")
	if !ok {
		t.Fatal("expected key to exist")
	}

	if ttl != -1 {
		t.Fatalf("expected TTL -1 for key with no expiry, got %d", ttl)
	}
}

func TestTTLMissingKey(t *testing.T) {
	s := NewStore()

	ttl, ok := s.TTL("missing")
	if ok {
		t.Fatal("expected missing key to not exist")
	}

	if ttl != -2 {
		t.Fatalf("expected TTL -2 for missing key, got %d", ttl)
	}
}

func TestDeleteExpired(t *testing.T) {
	s := NewStore()

	s.Set("temp", "hello")
	s.Expire("temp", 1)

	time.Sleep(1100 * time.Millisecond)

	deleted := s.DeleteExpired()
	if deleted != 1 {
		t.Fatalf("expected 1 deleted key, got %d", deleted)
	}

	if s.Size() != 0 {
		t.Fatalf("expected store size 0, got %d", s.Size())
	}
}

func TestExpirationWorker(t *testing.T) {
	s := NewStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.StartExpirationWorker(ctx, 10*time.Millisecond)

	s.Set("temp", "hello")
	s.Expire("temp", 1)

	time.Sleep(1200 * time.Millisecond)

	if s.Size() != 0 {
		t.Fatalf("expected background worker to delete expired key, got size %d", s.Size())
	}
}
