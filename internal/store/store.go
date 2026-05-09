package store

import (
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
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok {
		return false
	}

	entry.ExpiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
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
