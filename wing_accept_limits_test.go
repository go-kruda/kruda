package kruda

import "testing"

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
