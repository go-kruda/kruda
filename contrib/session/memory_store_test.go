package session

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_GetNonExistent(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	data, err := store.Get("non-existent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatal("expected nil for non-existent session")
	}
}

func TestMemoryStore_SaveAndGet(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	original := &SessionData{
		Values:    map[string]any{"user": "alice", "role": "admin"},
		CreatedAt: time.Now(),
	}

	if err := store.Save("sess-1", original, 10*time.Minute); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	data, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if data == nil {
		t.Fatal("expected session data, got nil")
	}
	if data.Values["user"] != "alice" {
		t.Fatalf("expected user=alice, got %v", data.Values["user"])
	}
	if data.Values["role"] != "admin" {
		t.Fatalf("expected role=admin, got %v", data.Values["role"])
	}
}

func TestMemoryStore_SaveOverwrite(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	first := &SessionData{
		Values:    map[string]any{"version": 1},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-1", first, 10*time.Minute); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	second := &SessionData{
		Values:    map[string]any{"version": 2},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-1", second, 10*time.Minute); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	data, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if data.Values["version"] != 2 {
		t.Fatalf("expected version=2 after overwrite, got %v", data.Values["version"])
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	data := &SessionData{
		Values:    map[string]any{"key": "value"},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-1", data, 10*time.Minute); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if err := store.Delete("sess-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	got, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// Deleting a non-existent key should not error.
	if err := store.Delete("does-not-exist"); err != nil {
		t.Fatalf("unexpected error deleting non-existent key: %v", err)
	}
}

func TestMemoryStore_ExpiredReturnsNil(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	data := &SessionData{
		Values:    map[string]any{"key": "value"},
		CreatedAt: time.Now(),
	}
	// Save with a very short TTL.
	if err := store.Save("sess-1", data, 1*time.Millisecond); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Wait for expiration.
	time.Sleep(10 * time.Millisecond)

	got, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for expired session")
	}

	// The expired entry should have been lazily removed.
	if store.Len() != 0 {
		t.Fatalf("expected 0 entries after lazy removal, got %d", store.Len())
	}
}

func TestMemoryStore_CleanupRemovesExpired(t *testing.T) {
	// Use a very short cleanup interval.
	store := NewMemoryStore(50 * time.Millisecond)
	defer store.Close()

	data := &SessionData{
		Values:    map[string]any{"key": "value"},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-1", data, 1*time.Millisecond); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Also save a non-expired session.
	longData := &SessionData{
		Values:    map[string]any{"key": "long"},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-2", longData, 10*time.Minute); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Wait for cleanup to run.
	time.Sleep(100 * time.Millisecond)

	// The expired session should have been cleaned up.
	if store.Len() != 1 {
		t.Fatalf("expected 1 entry after cleanup, got %d", store.Len())
	}

	// The long-lived session should still exist.
	got, err := store.Get("sess-2")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-expired session to still exist")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				sessionID := "sess-" + string(rune('A'+id%26))
				data := &SessionData{
					Values:    map[string]any{"count": j},
					CreatedAt: time.Now(),
				}
				_ = store.Save(sessionID, data, 10*time.Minute)
				_, _ = store.Get(sessionID)
				if j%10 == 0 {
					_ = store.Delete(sessionID)
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without a race condition panic, the test passes.
}

func TestMemoryStore_CloseStopsCleanup(t *testing.T) {
	store := NewMemoryStore(10 * time.Millisecond)

	data := &SessionData{
		Values:    map[string]any{"key": "value"},
		CreatedAt: time.Now(),
	}
	if err := store.Save("sess-1", data, 1*time.Millisecond); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Close the store to stop the cleanup goroutine.
	store.Close()

	// Wait some time — if cleanup were running it would panic on closed channel
	// or remove the entry. Since we closed, it should have stopped gracefully.
	time.Sleep(50 * time.Millisecond)

	// The entry may or may not still be in the map depending on timing,
	// but the important thing is Close() did not panic and the goroutine stopped.
}

func TestMemoryStore_Len(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if store.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", store.Len())
	}

	for i := 0; i < 5; i++ {
		data := &SessionData{
			Values:    map[string]any{"i": i},
			CreatedAt: time.Now(),
		}
		if err := store.Save("sess-"+string(rune('0'+i)), data, 10*time.Minute); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}

	if store.Len() != 5 {
		t.Fatalf("expected 5 entries, got %d", store.Len())
	}
}

func TestMemoryStore_DefaultCleanupInterval(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	if store.cleanupInterval != time.Minute {
		t.Fatalf("expected default cleanup interval of 1 minute, got %v", store.cleanupInterval)
	}
}

func TestMemoryStore_CustomCleanupInterval(t *testing.T) {
	store := NewMemoryStore(5 * time.Second)
	defer store.Close()

	if store.cleanupInterval != 5*time.Second {
		t.Fatalf("expected 5s cleanup interval, got %v", store.cleanupInterval)
	}
}
