package store

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type Entry struct {
	Value     string
	ExpiresAt time.Time
	HasExpiry bool
}

type item struct {
	key     string
	entry   Entry
	element *list.Element
}

type Stats struct {
	Keys           int
	MaxKeys        int
	Evictions      int64
	EvictionPolicy string
}

type Store struct {
	mu        sync.RWMutex
	data      map[string]*item
	lru       *list.List
	maxKeys   int
	evictions int64
}

func NewStore() *Store {
	return NewStoreWithMaxKeys(0)
}

func NewStoreWithMaxKeys(maxKeys int) *Store {
	if maxKeys < 0 {
		maxKeys = 0
	}

	return &Store{
		data:    make(map[string]*item),
		lru:     list.New(),
		maxKeys: maxKeys,
	}
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.data[key]; ok {
		existing.entry = Entry{
			Value: value,
		}

		if s.maxKeys > 0 && existing.element != nil {
			s.lru.MoveToFront(existing.element)
		}

		return
	}

	var element *list.Element
	if s.maxKeys > 0 {
		element = s.lru.PushFront(key)
	}

	s.data[key] = &item{
		key: key,
		entry: Entry{
			Value: value,
		},
		element: element,
	}

	s.enforceMaxKeysLocked()
}

func (s *Store) Get(key string) (string, bool) {
	if s.maxKeys > 0 {
		s.mu.Lock()
		defer s.mu.Unlock()

		it, ok := s.data[key]
		if !ok {
			return "", false
		}

		if isExpired(it.entry) {
			s.deleteItemLocked(it)
			return "", false
		}

		if it.element != nil {
			s.lru.MoveToFront(it.element)
		}

		return it.entry.Value, true
	}

	s.mu.RLock()
	it, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return "", false
	}

	if isExpired(it.entry) {
		s.Delete(key)
		return "", false
	}

	return it.entry.Value, true
}

func (s *Store) Exists(key string) bool {
	_, ok := s.Get(key)
	return ok
}

func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.data[key]
	if !ok {
		return false
	}

	s.deleteItemLocked(it)
	return true
}

func (s *Store) Expire(key string, ttlSeconds int) bool {
	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	return s.ExpireAt(key, expiresAt)
}

func (s *Store) ExpireAt(key string, expiresAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	it, ok := s.data[key]
	if !ok {
		return false
	}

	it.entry.ExpiresAt = expiresAt
	it.entry.HasExpiry = true

	return true
}

func (s *Store) TTL(key string) (int, bool) {
	s.mu.RLock()
	it, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return -2, false
	}

	if isExpired(it.entry) {
		s.Delete(key)
		return -2, false
	}

	if !it.entry.HasExpiry {
		return -1, true
	}

	remaining := int(time.Until(it.entry.ExpiresAt).Seconds())
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

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy := "none"
	if s.maxKeys > 0 {
		policy = "lru"
	}

	return Stats{
		Keys:           len(s.data),
		MaxKeys:        s.maxKeys,
		Evictions:      s.evictions,
		EvictionPolicy: policy,
	}
}

func (s *Store) DeleteExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0

	for _, it := range s.data {
		if isExpired(it.entry) {
			s.deleteItemLocked(it)
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

func (s *Store) Snapshot() map[string]Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make(map[string]Entry, len(s.data))

	for key, it := range s.data {
		if isExpired(it.entry) {
			continue
		}

		entries[key] = it.entry
	}

	return entries
}

func (s *Store) Load(entries map[string]Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*item, len(entries))
	s.lru = list.New()
	s.evictions = 0

	for key, entry := range entries {
		if isExpired(entry) {
			continue
		}

		var element *list.Element
		if s.maxKeys > 0 {
			element = s.lru.PushFront(key)
		}

		s.data[key] = &item{
			key:     key,
			entry:   entry,
			element: element,
		}
	}

	s.enforceMaxKeysLocked()
}

func (s *Store) enforceMaxKeysLocked() {
	if s.maxKeys <= 0 {
		return
	}

	s.deleteExpiredLocked()

	for len(s.data) > s.maxKeys {
		back := s.lru.Back()
		if back == nil {
			return
		}

		key := back.Value.(string)
		it, ok := s.data[key]
		if !ok {
			s.lru.Remove(back)
			continue
		}

		s.deleteItemLocked(it)
		s.evictions++
	}
}

func (s *Store) deleteExpiredLocked() int {
	deleted := 0

	for _, it := range s.data {
		if isExpired(it.entry) {
			s.deleteItemLocked(it)
			deleted++
		}
	}

	return deleted
}

func (s *Store) deleteItemLocked(it *item) {
	if it.element != nil {
		s.lru.Remove(it.element)
	}

	delete(s.data, it.key)
}

func isExpired(entry Entry) bool {
	return entry.HasExpiry && time.Now().After(entry.ExpiresAt)
}
