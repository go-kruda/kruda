package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
)

func TestGzipCompression(t *testing.T) {
	app := kruda.New()
	app.Use(New(Config{MinSize: 10})) // Lower MinSize for testing
	app.Get("/test", func(c *kruda.Ctx) error {
		return CompressText(c, "hello world")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("expected Content-Encoding: gzip, got %s", w.Header().Get("Content-Encoding"))
	}

	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("expected Vary: Accept-Encoding, got %s", w.Header().Get("Vary"))
	}

	// Decompress and verify
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	body, err := io.ReadAll(gr)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(body))
	}
}

func TestDeflateCompression(t *testing.T) {
	app := kruda.New()
	app.Use(New(Config{MinSize: 10})) // Lower MinSize for testing
	app.Get("/test", func(c *kruda.Ctx) error {
		return CompressText(c, "hello world")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "deflate" {
		t.Errorf("expected Content-Encoding: deflate, got %s", w.Header().Get("Content-Encoding"))
	}

	// Decompress and verify
	dr := flate.NewReader(w.Body)
	defer dr.Close()

	body, err := io.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(body))
	}
}

func TestNoAcceptEncoding(t *testing.T) {
	app := kruda.New()
	app.Use(New())
	app.Get("/test", func(c *kruda.Ctx) error {
		return CompressText(c, "hello world")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Error("should not compress when no Accept-Encoding header")
	}

	if w.Body.String() != "hello world" {
		t.Errorf("expected 'hello world', got %s", w.Body.String())
	}
}

func TestMinSizeSkip(t *testing.T) {
	app := kruda.New()
	app.Use(New(Config{MinSize: 100}))
	app.Get("/test", func(c *kruda.Ctx) error {
		return CompressText(c, "small")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Error("should not compress responses smaller than MinSize")
	}

	if w.Body.String() != "small" {
		t.Errorf("expected 'small', got %s", w.Body.String())
	}
}

func TestImageTypeSkip(t *testing.T) {
	app := kruda.New()
	app.Use(New())
	app.Get("/test", func(c *kruda.Ctx) error {
		return Compress(c, []byte("fake image data"), "image/jpeg")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Error("should not compress image/* content types")
	}
}

func TestQualityValues(t *testing.T) {
	app := kruda.New()
	app.Use(New(Config{MinSize: 10})) // Lower MinSize for testing
	app.Get("/test", func(c *kruda.Ctx) error {
		return CompressText(c, "hello world")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "deflate;q=0.8, gzip;q=0.9")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("expected gzip (higher quality), got %s", w.Header().Get("Content-Encoding"))
	}
}

func TestAlreadyEncoded(t *testing.T) {
	app := kruda.New()
	app.Use(New())
	app.Get("/test", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Encoding", "br")
		return CompressText(c, "already compressed")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	app.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "br" {
		t.Error("should not modify existing Content-Encoding")
	}

	if w.Body.String() != "already compressed" {
		t.Errorf("expected 'already compressed', got %s", w.Body.String())
	}
}