package storage

import (
	"path/filepath"
	"sync"
	"time"
)

type Item struct {
	Value     string
	ExpiresAt int64
}

func (i Item) IsExpired() bool {
	if i.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > i.ExpiresAt
}

type Store struct {
	mu   sync.RWMutex
	data map[string]Item
	aof  *AOF
}

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

		if err := aof.Replay(s); err != nil {
			return nil, err
		}
	}

	go s.activeEvictionLoop(100*time.Millisecond, 20)
	return s, nil
}

func (s *Store) Set(key, value string, ttlSeconds int) {
	var expiresAt int64
	if ttlSeconds > 0 {
		expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixNano()
	}

	s.mu.Lock()
	s.data[key] = Item{
		Value:     value,
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()

	if s.aof != nil {
		_ = s.aof.WriteCommand("SET", key, value)
	}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return "", false
	}

	if item.IsExpired() {
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return "", false
	}

	return item.Value, true
}

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

func (s *Store) Exists(key string) bool {
	_, found := s.Get(key)
	return found
}

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

	diff := item.ExpiresAt - time.Now().UnixNano()
	if diff <= 0 {
		return -2
	}
	return diff / int64(time.Second)
}

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

func (s *Store) activeEvictionLoop(interval time.Duration, sampleSize int) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		s.mu.Lock()
		if len(s.data) == 0 {
			s.mu.Unlock()
			continue
		}

		now := time.Now().UnixNano()
		checked := 0
		for k, item := range s.data {
			if checked >= sampleSize {
				break
			}
			checked++

			if item.ExpiresAt > 0 && now > item.ExpiresAt {
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Store) Close() error {
	if s.aof != nil {
		return s.aof.Close()
	}
	return nil
}
