package session

import (
	"sync"
	"testing"
	"time"
)

func TestUntouchedNewSessionIsNotStored(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	s := Session{id: "new", data: &SessionData{}, store: store, config: &Config{IdleTimeout: time.Minute}, isNew: true}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 0 {
		t.Fatal("untouched session was stored")
	}
	s.Set("user", "alice")
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("new")
	if err != nil || got == nil || got.Values["user"] != "alice" {
		t.Fatalf("modified session not saved: %#v %v", got, err)
	}
}

func TestMemoryStoreCloseConcurrent(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); store.Close() }()
	}
	wg.Wait()
}

func TestReadOnlySessionRefreshPreservesCurrentData(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	if err := store.Save("id", &SessionData{Values: map[string]any{"user": "new"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	before := store.entries["id"].expiresAt
	store.mu.Unlock()
	s := Session{id: "id", data: &SessionData{Values: map[string]any{"user": "stale"}}, store: store, config: &Config{IdleTimeout: time.Hour}}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("id")
	if err != nil || got == nil || got.Values["user"] != "new" {
		t.Fatalf("refresh replaced current values: %#v %v", got, err)
	}
	store.mu.Lock()
	after := store.entries["id"].expiresAt
	store.mu.Unlock()
	if !after.After(before) {
		t.Fatal("expiry not refreshed")
	}
	if err := store.Delete("id"); err != nil {
		t.Fatal(err)
	}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 0 {
		t.Fatal("read-only refresh resurrected deleted session")
	}
}

func TestReadOnlyRefreshDoesNotReviveExpiredSession(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	data := &SessionData{Values: map[string]any{"user": "expired"}}
	if err := store.Save("id", data, -time.Second); err != nil {
		t.Fatal(err)
	}
	s := Session{id: "id", data: data, store: store, config: &Config{IdleTimeout: time.Hour}}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("id")
	if err != nil || got != nil {
		t.Fatalf("expired session revived: %#v %v", got, err)
	}
}
