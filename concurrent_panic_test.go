package kruda

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConcurrencyHelpersReportPanics(t *testing.T) {
	t.Run("Parallel", func(t *testing.T) {
		err := Parallel(func() error { return nil }, func() error { panic("boom") })
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want the panic as an error", err)
		}
	})
	t.Run("Each", func(t *testing.T) {
		err := Each([]int{1, 2}, func(n int) error {
			if n == 2 {
				panic("boom")
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want the panic as an error", err)
		}
	})
	t.Run("Race", func(t *testing.T) {
		_, err := Race(context.Background(), func(context.Context) (any, error) { panic("boom") })
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want the panic as an error", err)
		}
	})
	t.Run("Race loser", func(t *testing.T) {
		release := make(chan struct{})
		done := make(chan struct{})
		v, err := Race(context.Background(),
			func(context.Context) (any, error) { return "first", nil },
			func(context.Context) (any, error) {
				defer close(done)
				<-release
				panic("late boom")
			},
		)
		close(release)
		<-done
		if err != nil || v != "first" {
			t.Fatalf("got %v, %v; want first, nil", v, err)
		}
	})
}

type panickingChecker struct{}

func (panickingChecker) Check(context.Context) error { panic("db driver bug") }

func TestHealthCheckPanicReportsUnhealthy(t *testing.T) {
	c := NewContainer()
	if err := c.Give(panickingChecker{}); err != nil {
		t.Fatal(err)
	}
	app := New(WithContainer(c))
	app.Get("/health", HealthHandler())
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unhealthy") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
