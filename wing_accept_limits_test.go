package kruda

import (
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	b := newTokenBucket(10, 10) // 10/s, burst 10
	now := int64(0)
	for i := 0; i < 10; i++ {
		if !b.allow(now) {
			t.Fatalf("burst token %d should be allowed", i)
		}
	}
	if b.allow(now) {
		t.Fatal("11th in same instant must be rejected")
	}
	now += int64(time.Second) // +1s → +10 tokens (capped at burst)
	if !b.allow(now) {
		t.Fatal("after 1s a token should be available")
	}
}

func TestTokenBucket_BurstRaisedToRate(t *testing.T) {
	// burst < perSec → burst should be raised to perSec
	b := newTokenBucket(10, 3)
	if b.burst != 10 {
		t.Fatalf("burst should be raised to perSec=10, got %v", b.burst)
	}
	// Should still have burst=10 tokens initially
	for i := 0; i < 10; i++ {
		if !b.allow(0) {
			t.Fatalf("token %d should be allowed (burst raised to 10)", i)
		}
	}
	if b.allow(0) {
		t.Fatal("11th token must be rejected")
	}
}

func TestDeriveMaxConns(t *testing.T) {
	// headroom = 3*workers + 256; ceiling = 262144
	cases := []struct {
		name    string
		soft    uint64
		workers int
		want    int
	}{
		{"typical", 65536, 8, 65536 - (3*8 + 256)},
		{"low_ulimit", 1024, 4, 1024 - (3*4 + 256)},
		{"infinity_clamped_to_ceiling", 1 << 30, 16, 262144},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveMaxConns(c.soft, c.workers); got != c.want {
				t.Fatalf("deriveMaxConns(%d,%d)=%d want %d", c.soft, c.workers, got, c.want)
			}
		})
	}
}
