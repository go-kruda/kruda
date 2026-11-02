package cache

import (
	"bytes"
	"errors"
	"github.com/go-kruda/kruda"
	"net/http/httptest"
	"testing"
)

func TestCacheJSONUsesConfiguredEncodingOnce(t *testing.T) {
	for _, stream := range []bool{false, true} {
		calls := 0
		options := []kruda.Option{kruda.WithJSONEncoder(func(any) ([]byte, error) { calls++; return []byte(`"configured"`), nil })}
		if stream {
			options = append(options, kruda.WithJSONStreamEncoder(func(b *bytes.Buffer, _ any) error { calls++; b.WriteString(`"stream"`); return nil }))
		}
		app := kruda.New(options...)
		store := NewMemoryStore(10)
		defer store.Close()
		app.Use(New(Config{Store: store}))
		app.Get("/data", func(c *kruda.Ctx) error { return CacheJSON(c, kruda.Map{"value": 1}) })
		want := `"configured"`
		if stream {
			want = `"stream"`
		}
		for i := 0; i < 2; i++ {
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest("GET", "/data", nil))
			if rec.Body.String() != want {
				t.Fatalf("stream=%v request=%d body=%s want=%s", stream, i, rec.Body.String(), want)
			}
		}
		if calls != 1 {
			t.Fatalf("encoder calls=%d want=1", calls)
		}
	}
}

func TestCacheJSONCustomEncoderAcceptsNonStandardValue(t *testing.T) {
	for _, active := range []bool{false, true} {
		app := kruda.New(kruda.WithJSONEncoder(func(any) ([]byte, error) { return []byte(`"custom"`), nil }))
		if active {
			app.Use(New())
		}
		app.Get("/", func(c *kruda.Ctx) error { return CacheJSON(c, make(chan int)) })
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != 200 || rec.Body.String() != `"custom"` {
			t.Fatalf("active=%v status=%d body=%s", active, rec.Code, rec.Body.String())
		}
	}
}

func TestCacheJSONEncoderErrorIsNotCached(t *testing.T) {
	calls := 0
	store := NewMemoryStore(10)
	defer store.Close()
	app := kruda.New(
		kruda.WithJSONEncoder(func(any) ([]byte, error) {
			calls++
			return nil, errors.New("encode failed")
		}),
		kruda.WithErrorHandler(func(c *kruda.Ctx, err *kruda.KrudaError) {
			c.Status(500).Text("encode failed")
		}),
	)
	app.Use(New(Config{Store: store}))
	app.Get("/", func(c *kruda.Ctx) error { return CacheJSON(c, 1) })

	for range 2 {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != 500 {
			t.Fatal(rec.Code)
		}
	}
	if store.Len() != 0 || calls != 2 {
		t.Fatalf("entries=%d encoder calls=%d", store.Len(), calls)
	}
}
