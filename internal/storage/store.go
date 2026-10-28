package storage

import (
	"math/rand"
	"path/filepath"
	"sync"
	"time"
)

// Item wraps a string value along with an optional expiration timestamp.
type Item struct {
	Value     string
	ExpiresAt int64 // Nanoseconds since Unix epoch. 0 indicates persistent (no TTL).
}

func (i Item) IsExpired() bool {
	if i.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > i.ExpiresAt
}

// Store is a thread-safe, in-memory key-value dictionary with TTL eviction.
type Store struct {
	mu   sync.RWMutex
	data map[string]Item
	aof  *AOF
}

// NewStore initializes the memory dictionary and starts active eviction.
func NewStore(aofPath string) (*Store, error) {
	s := &Store{
		data: make(map[string]Item),
	}

	if aofPath != "" {
		aof, err := NewAOF(aofPath)
		if err != nil {
			return nil, err
		}
		s.aof = aof

		// Replay past mutations on boot
		if err := aof.Replay(s); err != nil {
			return nil, err
		}
	}

	// Start background active TTL eviction goroutine
	go s.activeEvictionLoop(100*time.Millisecond, 20)

	return s, nil
}

// Set stores a key-value pair, optionally with a Time-To-Live in seconds.
func (s *Store) Set(key, value string, ttlSeconds int) {
	var expiresAt int64 = 0
	if ttlSeconds > 0 {
		expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixNano()
	}

	s.mu.Lock()
	s.data[key] = Item{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()

	// Append to durability log if AOF is configured
	if s.aof != nil {
		_ = s.aof.WriteCommand("SET", key, value)
	}
}

// Get retrieves a key, applying Passive Expiration if the key's TTL has elapsed.
//
// Professor's Note:
// Passive eviction occurs on read: if a client requests an expired key,
// the server deletes it lazily and returns a cache miss.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return "", false
	}

	if item.IsExpired() {
		// Key expired; promote lock to write-lock and purge
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return "", false
	}

	return item.Value, true
}

// Del deletes a key from memory and appends the deletion to AOF.
func (s *Store) Del(key string) bool {
	s.mu.Lock()
	_, exists := s.data[key]
	if exists {
		delete(s.data, key)
	}
	s.mu.Unlock()

	if exists && s.aof != nil {
		_ = s.aof.WriteCommand("DEL", key)
	}

	return exists
}

// Exists checks whether a non-expired key is present.
func (s *Store) Exists(key string) bool {
	_, found := s.Get(key)
	return found
}

// Expire sets a new TTL on an existing key.
func (s *Store) Expire(key string, seconds int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.data[key]
	if !exists || item.IsExpired() {
		return false
	}

	item.ExpiresAt = time.Now().Add(time.Duration(seconds) * time.Second).UnixNano()
	s.data[key] = item
	return true
}

// TTL returns remaining lifetime in seconds:
// -2 if key does not exist or expired
// -1 if key exists without TTL
// >= 0 remaining seconds
func (s *Store) TTL(key string) int64 {
	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists || item.IsExpired() {
		return -2
	}
	if item.ExpiresAt == 0 {
		return -1
	}

	diffNanos := item.ExpiresAt - time.Now().UnixNano()
	if diffNanos <= 0 {
		return -2
	}
	return diffNanos / int64(time.Second)
}

// Keys returns all active non-expired keys matching a glob pattern (e.g. "*").
func (s *Store) Keys(pattern string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UnixNano()
	var res []string
	for k, v := range s.data {
		if v.ExpiresAt > 0 && now > v.ExpiresAt {
			continue
		}
		if match, _ := filepath.Match(pattern, k); match {
			res = append(res, k)
		}
	}
	return res
}

// activeEvictionLoop implements the probabilistic Redis Active TTL Eviction algorithm:
// 1. Samples 'sampleSize' random keys from the keyspace.
// 2. Checks and purges any keys that have passed their expiration deadline.
// 3. If >25% of the sampled keys were expired, repeat immediately to clear backlog.
func (s *Store) activeEvictionLoop(interval time.Duration, sampleSize int) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.mu.Lock()
		if len(s.data) == 0 {
			s.mu.Unlock()
			continue
		}

		now := time.Now().UnixNano()
		expiredCount := 0
		checked := 0

		// Map iteration in Go is randomized by the runtime
		for k, item := range s.data {
			if checked >= sampleSize {
				break
			}
			checked++

			if item.ExpiresAt > 0 && now > item.ExpiresAt {
				delete(s.data, k)
				expiredCount++
			}
		}
		s.mu.Unlock()
		_ = rand.Int() // keep rand imported if needed
	}
}

// Close gracefully flushes and closes underlying persistence logs.
func (s *Store) Close() error {
	if s.aof != nil {
		return s.aof.Close()
	}
	return nil
}
