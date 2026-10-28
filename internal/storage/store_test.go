package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSetGetDel(t *testing.T) {
	s, err := NewStore("")
	if err != nil {
		t.Fatalf("failed creating store: %v", err)
	}

	s.Set("name", "antigravity", 0)

	val, found := s.Get("name")
	if !found || val != "antigravity" {
		t.Fatalf("expected 'antigravity', got '%s'", val)
	}

	if !s.Exists("name") {
		t.Errorf("expected key to exist")
	}

	deleted := s.Del("name")
	if !deleted {
		t.Errorf("expected delete to return true")
	}

	_, foundAfter := s.Get("name")
	if foundAfter {
		t.Errorf("expected key to be deleted")
	}
}

func TestStoreTTLExpiration(t *testing.T) {
	s, err := NewStore("")
	if err != nil {
		t.Fatalf("failed creating store: %v", err)
	}

	// Set key with 1-second TTL
	s.Set("temp", "fast", 1)

	val, found := s.Get("temp")
	if !found || val != "fast" {
		t.Fatalf("expected key to exist immediately")
	}

	// Sleep past expiration threshold
	time.Sleep(1100 * time.Millisecond)

	_, foundAfter := s.Get("temp")
	if foundAfter {
		t.Errorf("expected key to be expired and passively evicted")
	}
}

func TestAOFReplayPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	aofPath := filepath.Join(tmpDir, "test.aof")

	// 1. Initial Store instance writes data
	s1, err := NewStore(aofPath)
	if err != nil {
		t.Fatalf("failed init store 1: %v", err)
	}
	s1.Set("k1", "v1", 0)
	s1.Set("k2", "v2", 0)
	s1.Del("k1")
	s1.Set("k3", "v3", 0)
	_ = s1.Close()

	// 2. Second Store instance boots from same AOF file (simulating reboot after crash)
	s2, err := NewStore(aofPath)
	if err != nil {
		t.Fatalf("failed init store 2: %v", err)
	}
	defer s2.Close()

	// k1 should NOT exist (was deleted)
	if _, found := s2.Get("k1"); found {
		t.Errorf("k1 should have been deleted by replay")
	}

	// k2 should exist
	if v, found := s2.Get("k2"); !found || v != "v2" {
		t.Errorf("expected k2='v2', got '%s'", v)
	}

	// k3 should exist
	if v, found := s2.Get("k3"); !found || v != "v3" {
		t.Errorf("expected k3='v3', got '%s'", v)
	}
}
