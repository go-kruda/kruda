package cache

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_GetNonExistent(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	resp, err := store.Get("non-existent-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != nil {
		t.Fatal("expected nil for non-existent key")
	}
}

func TestMemoryStore_SetAndGet(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	original := &CachedResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"ok":true}`),
		CachedAt:   time.Now(),
	}

	if err := store.Set("key-1", original, 10*time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	resp, err := store.Get("key-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected cached response, got nil")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("expected body {\"ok\":true}, got %s", resp.Body)
	}
	if resp.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", resp.Headers["Content-Type"])
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	resp := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("hello"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("key-1", resp, 10*time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	if err := store.Delete("key-1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	got, err := store.Get("key-1")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemoryStore_DeleteNonExistent(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	// Deleting a non-existent key should not error.
	if err := store.Delete("does-not-exist"); err != nil {
		t.Fatalf("unexpected error deleting non-existent key: %v", err)
	}
}

func TestMemoryStore_Expiry(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	resp := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("ephemeral"),
		CachedAt:   time.Now(),
	}
	// Save with a very short TTL.
	if err := store.Set("key-1", resp, 1*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Wait for expiration.
	time.Sleep(10 * time.Millisecond)

	got, err := store.Get("key-1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for expired entry")
	}

	// The expired entry should have been lazily removed.
	if store.Len() != 0 {
		t.Fatalf("expected 0 entries after lazy removal, got %d", store.Len())
	}
}

func TestMemoryStore_MaxEntries(t *testing.T) {
	store := NewMemoryStore(3)
	defer store.Close()

	for i := 0; i < 5; i++ {
		resp := &CachedResponse{
			StatusCode: 200,
			Body:       []byte("data"),
			CachedAt:   time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := store.Set("key-"+string(rune('A'+i)), resp, 10*time.Minute); err != nil {
			t.Fatalf("set %d failed: %v", i, err)
		}
	}

	// Should have at most 3 entries.
	if store.Len() != 3 {
		t.Fatalf("expected max 3 entries, got %d", store.Len())
	}
}

func TestMemoryStore_MaxEntries_EvictsOldest(t *testing.T) {
	store := NewMemoryStore(2)
	defer store.Close()

	// Entry A (oldest)
	respA := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("A"),
		CachedAt:   time.Now().Add(-10 * time.Second),
	}
	if err := store.Set("A", respA, 10*time.Minute); err != nil {
		t.Fatalf("set A failed: %v", err)
	}

	// Entry B (newer)
	respB := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("B"),
		CachedAt:   time.Now().Add(-5 * time.Second),
	}
	if err := store.Set("B", respB, 10*time.Minute); err != nil {
		t.Fatalf("set B failed: %v", err)
	}

	// Entry C should evict A (oldest).
	respC := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("C"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("C", respC, 10*time.Minute); err != nil {
		t.Fatalf("set C failed: %v", err)
	}

	if store.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", store.Len())
	}

	// A should be evicted.
	got, _ := store.Get("A")
	if got != nil {
		t.Fatal("expected A to be evicted")
	}

	// B and C should still exist.
	got, _ = store.Get("B")
	if got == nil {
		t.Fatal("expected B to still exist")
	}
	got, _ = store.Get("C")
	if got == nil {
		t.Fatal("expected C to still exist")
	}
}

func TestMemoryStore_MaxEntries_OverwriteExisting(t *testing.T) {
	store := NewMemoryStore(2)
	defer store.Close()

	resp := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("v1"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("A", resp, 10*time.Minute); err != nil {
		t.Fatalf("set A failed: %v", err)
	}

	resp2 := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("v2"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("B", resp2, 10*time.Minute); err != nil {
		t.Fatalf("set B failed: %v", err)
	}

	// Overwrite A -- should NOT evict anything since key already exists.
	resp3 := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("v3"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("A", resp3, 10*time.Minute); err != nil {
		t.Fatalf("overwrite A failed: %v", err)
	}

	if store.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", store.Len())
	}

	got, _ := store.Get("A")
	if got == nil || string(got.Body) != "v3" {
		t.Fatalf("expected A body to be v3, got %v", got)
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	// Use a very short cleanup interval.
	store := NewMemoryStore(100, 50*time.Millisecond)
	defer store.Close()

	expired := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("expired"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("expired", expired, 1*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Also save a non-expired entry.
	longLived := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("long-lived"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("long-lived", longLived, 10*time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Wait for cleanup to run.
	time.Sleep(100 * time.Millisecond)

	// The expired entry should have been cleaned up.
	if store.Len() != 1 {
		t.Fatalf("expected 1 entry after cleanup, got %d", store.Len())
	}

	// The long-lived entry should still exist.
	got, err := store.Get("long-lived")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-expired entry to still exist")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore(1000)
	defer store.Close()

	const goroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "key-" + string(rune('A'+id%26))
				resp := &CachedResponse{
					StatusCode: 200,
					Body:       []byte("data"),
					CachedAt:   time.Now(),
				}
				_ = store.Set(key, resp, 10*time.Minute)
				_, _ = store.Get(key)
				if j%10 == 0 {
					_ = store.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without a race condition panic, the test passes.
}

func TestMemoryStore_CloseStopsCleanup(t *testing.T) {
	store := NewMemoryStore(100, 10*time.Millisecond)

	resp := &CachedResponse{
		StatusCode: 200,
		Body:       []byte("data"),
		CachedAt:   time.Now(),
	}
	if err := store.Set("key-1", resp, 1*time.Millisecond); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Close the store to stop the cleanup goroutine.
	store.Close()

	// Wait some time -- if cleanup were running it would panic on closed channel
	// or remove the entry. Since we closed, it should have stopped gracefully.
	time.Sleep(50 * time.Millisecond)
}

func TestMemoryStore_Len(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	if store.Len() != 0 {
		t.Fatalf("expected 0 entries, got %d", store.Len())
	}

	for i := 0; i < 5; i++ {
		resp := &CachedResponse{
			StatusCode: 200,
			Body:       []byte("data"),
			CachedAt:   time.Now(),
		}
		if err := store.Set("key-"+string(rune('0'+i)), resp, 10*time.Minute); err != nil {
			t.Fatalf("set failed: %v", err)
		}
	}

	if store.Len() != 5 {
		t.Fatalf("expected 5 entries, got %d", store.Len())
	}
}

func TestMemoryStore_DefaultCleanupInterval(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()

	if store.cleanupInterval != time.Minute {
		t.Fatalf("expected default cleanup interval of 1 minute, got %v", store.cleanupInterval)
	}
}

func TestMemoryStore_CustomCleanupInterval(t *testing.T) {
	store := NewMemoryStore(100, 5*time.Second)
	defer store.Close()

	if store.cleanupInterval != 5*time.Second {
		t.Fatalf("expected 5s cleanup interval, got %v", store.cleanupInterval)
	}
}

func TestMemoryStore_UnlimitedEntries(t *testing.T) {
	// maxEntries=0 means unlimited.
	store := NewMemoryStore(0)
	defer store.Close()

	for i := 0; i < 100; i++ {
		resp := &CachedResponse{
			StatusCode: 200,
			Body:       []byte("data"),
			CachedAt:   time.Now(),
		}
		if err := store.Set("key-"+string(rune('A'+i%26))+"-"+string(rune('0'+i/26)), resp, 10*time.Minute); err != nil {
			t.Fatalf("set %d failed: %v", i, err)
		}
	}

	// With unlimited entries and possible key collisions, just verify it didn't panic.
	if store.Len() == 0 {
		t.Fatal("expected some entries")
	}
}
