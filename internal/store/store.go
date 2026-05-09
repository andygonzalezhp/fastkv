package store

import (
	"context"
	"sync"
	"time"
)

type Entry struct {
	Value     string
	ExpiresAt time.Time
	HasExpiry bool
}

type Store struct {
	mu   sync.RWMutex
	data map[string]Entry
}

func NewStore() *Store {
	return &Store{
		data: make(map[string]Entry),
	}
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = Entry{
		Value: value,
	}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	entry, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return "", false
	}

	if entry.HasExpiry && time.Now().After(entry.ExpiresAt) {
		s.Delete(key)
		return "", false
	}

	return entry.Value, true
}

func (s *Store) Exists(key string) bool {
	_, ok := s.Get(key)
	return ok
}

func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[key]; !ok {
		return false
	}

	delete(s.data, key)
	return true
}

func (s *Store) Expire(key string, ttlSeconds int) bool {
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	return s.ExpireAt(key, expiresAt)
}

func (s *Store) ExpireAt(key string, expiresAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return false
	}

	entry.ExpiresAt = expiresAt
	entry.HasExpiry = true

	s.data[key] = entry
	return true
}

func (s *Store) TTL(key string) (int, bool) {
	s.mu.RLock()
	entry, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return -2, false
	}

	if entry.HasExpiry && time.Now().After(entry.ExpiresAt) {
		s.Delete(key)
		return -2, false
	}

	if !entry.HasExpiry {
		return -1, true
	}

	remaining := int(time.Until(entry.ExpiresAt).Seconds())
	if remaining < 0 {
		return -2, false
	}

	return remaining, true
}

func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

func (s *Store) DeleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	deleted := 0

	for key, entry := range s.data {
		if entry.HasExpiry && now.After(entry.ExpiresAt) {
			delete(s.data, key)
			deleted++
		}
	}

	return deleted
}

func (s *Store) StartExpirationWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.DeleteExpired()

			case <-ctx.Done():
				return
			}
		}
	}()
}
