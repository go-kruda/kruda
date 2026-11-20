package cache

import (
	"strconv"
	"testing"
	"time"
)

func sizedResponse(n int) *CachedResponse {
	return &CachedResponse{StatusCode: 200, Body: make([]byte, n), CachedAt: time.Now()}
}

// TestMemoryStore_MaxBytes fills the store past its byte budget. Old entries
// must be evicted so the total stays under budget, and a response larger than
// the whole budget is not stored.
func TestMemoryStore_MaxBytes(t *testing.T) {
	store := NewMemoryStore(0).SetMaxBytes(10 << 10)
	defer store.Close()
	for i := range 20 {
		if err := store.Set("k"+strconv.Itoa(i), sizedResponse(1<<10), time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if n := store.Len(); n == 0 || n > 10 {
		t.Fatalf("Len = %d, want 1..10 under a 10 KiB budget", n)
	}
	if got, _ := store.Get("k19"); got == nil {
		t.Fatal("newest entry was evicted")
	}
	if got, _ := store.Get("k0"); got != nil {
		t.Fatal("oldest entry was kept past the budget")
	}
	if err := store.Set("huge", sizedResponse(11<<10), time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get("huge"); got != nil {
		t.Fatal("a response larger than the budget was stored")
	}
}

// TestMemoryStore_DefaultByteBudget checks that the default store bounds bytes,
// not only entries: 100 distinct 1 MiB responses must not all stay cached.
func TestMemoryStore_DefaultByteBudget(t *testing.T) {
	store := NewMemoryStore(1000)
	defer store.Close()
	for i := range 100 {
		_ = store.Set("q="+strconv.Itoa(i), sizedResponse(1<<20), time.Minute)
	}
	if n := store.Len(); n > int(DefaultMaxBytes>>20) {
		t.Fatalf("Len = %d MiB-sized entries, over the %d MiB default budget", n, DefaultMaxBytes>>20)
	}
}
