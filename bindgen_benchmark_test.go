package kruda

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	krudajson "github.com/go-kruda/kruda/json"
)

var bindgenGeneratedParse func(*Ctx) (reflect.Value, error)
var bindgenGeneratedFactory func(*App, string, func(*C[generatedBinderInput]) (*generatedBinderOutput, error)) HandlerFunc
var bindgenValueSink reflect.Value

func bindgenArm() string {
	if arm := os.Getenv("KRUDA_BINDGEN_ARM"); arm != "" {
		return arm
	}
	return "normal"
}

func bindgenApp(tb testing.TB, variant string) *App {
	tb.Helper()
	app := New(Wing(), WithValidator(NewValidator()))
	if variant == "pipeline" {
		app.Use(func(c *Ctx) error {
			c.Set("request-source", "benchmark")
			return c.Next()
		}, func(c *Ctx) error {
			c.SetHeader("X-Benchmark", "bindgen")
			return c.Next()
		})
		app.OnParse(func(_ *Ctx, input any) error {
			in := input.(*generatedBinderInput)
			in.Name = strings.TrimSpace(in.Name)
			return nil
		})
	}
	handler := func(c *C[generatedBinderInput]) (*generatedBinderOutput, error) {
		return &generatedBinderOutput{ID: c.In.ID, Page: c.In.Page, Active: c.In.Active, Score: c.In.Score, Name: c.In.Name}, nil
	}
	var h HandlerFunc
	switch bindgenArm() {
	case "normal":
		h = buildTypedHandler(app, "GET", "/users/:id", handler, nil)
	case "generated":
		if bindgenGeneratedFactory == nil {
			tb.Fatal("generated binder unavailable in stock control")
		}
		h = bindgenGeneratedFactory(app, "/users/:id", handler)
	default:
		tb.Fatalf("unknown binder arm %q", bindgenArm())
	}
	app.Get("/users/:id", h, JSON)
	app.Get("/plaintext", func(c *Ctx) error { return c.Text("Hello, World!") }, Plaintext)
	app.Compile()
	return app
}

func bindgenQuery() map[string]string {
	return map[string]string{"id": "9", "page": "3", "active": "false", "score": "2.5", "name": "  alice  "}
}

func BenchmarkBindgenBind(b *testing.B) {
	parse := buildInputParser[generatedBinderInput]().parse
	if bindgenArm() == "generated" {
		if bindgenGeneratedParse == nil {
			b.Fatal("generated binder unavailable in stock control")
		}
		parse = bindgenGeneratedParse
	}
	c := bindCtx("GET", "/users/42", map[string]string{"id": "42"}, bindgenQuery(), nil)
	v, err := parse(c)
	if err != nil || v.Interface().(generatedBinderInput).ID != 42 {
		b.Fatalf("warmup parse: %v, %v", v, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, err := parse(c)
		if err != nil {
			b.Fatal(err)
		}
		bindgenValueSink = v
	}
}

func BenchmarkBindgenRoute(b *testing.B) {
	for _, variant := range []string{"validation", "pipeline"} {
		b.Run(variant, func(b *testing.B) {
			app := bindgenApp(b, variant)
			req := &mockRequest{method: "GET", path: "/users/42", query: bindgenQuery()}
			w := newMockResponse()
			app.ServeKruda(w, req)
			name := "  alice  "
			if variant == "pipeline" {
				name = "alice"
			}
			want := fmt.Sprintf(`{"id":42,"page":3,"active":false,"score":2.5,"name":%q}`, name)
			if w.statusCode != 200 || string(w.body) != want {
				b.Fatalf("warmup response: status=%d body=%s want=%s", w.statusCode, w.body, want)
			}
			if variant == "pipeline" && w.headers.Get("X-Benchmark") != "bindgen" {
				b.Fatal("middleware header missing")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				w.body = w.body[:0]
				w.statusCode = 0
				clear(w.headers.h)
				app.ServeKruda(w, req)
			}
		})
	}
}

func TestBindgenBenchmarkServer(t *testing.T) {
	if os.Getenv("KRUDA_BINDGEN_SERVER") != "1" {
		t.Skip("benchmark server is opt-in")
	}
	variant := os.Getenv("KRUDA_BINDGEN_VARIANT")
	if variant != "validation" && variant != "pipeline" {
		t.Fatalf("unknown variant %q", variant)
	}
	addr := os.Getenv("KRUDA_BINDGEN_ADDR")
	if addr == "" {
		t.Fatal("KRUDA_BINDGEN_ADDR is required")
	}
	app := bindgenApp(t, variant)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	errCh := make(chan error, 1)
	fmt.Printf("bindgen arm=%s variant=%s go=%s engine=%s addr=%s\n", bindgenArm(), variant, runtime.Version(), krudajson.ActiveEngine(), addr)
	go func() { errCh <- app.Listen(addr) }()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		if err := <-errCh; err != nil {
			t.Error(err)
		}
	}
}
