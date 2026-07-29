package kruda

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

type optInInput struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
}

type optInPlain struct {
	Name string `json:"name"`
}

type optInOut struct {
	ID string `json:"id"`
}

func captureValidateWarning(t *testing.T) *bytes.Buffer {
	t.Helper()
	resetValidateTagWarningForTest()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(prev)
		resetValidateTagWarningForTest()
	})
	return &buf
}

// TestWarnsWhenValidateTagsHaveNoValidator covers the quietest way to ship a
// security bug with this framework: validation is opt-in, so validate: tags
// without a Validator are inert and nothing says so. The request succeeds and
// the handler gets whatever the client sent.
func TestWarnsWhenValidateTagsHaveNoValidator(t *testing.T) {
	buf := captureValidateWarning(t)

	app := New()
	Post[optInInput, optInOut](app, "/users", func(c *C[optInInput]) (*optInOut, error) {
		return &optInOut{ID: "1"}, nil
	})

	out := buf.String()
	if !strings.Contains(out, "no Validator is configured") {
		t.Fatalf("no warning for ignored validate: tags: %q", out)
	}
	if !strings.Contains(out, "WithValidator") {
		t.Errorf("warning does not say how to fix it: %q", out)
	}
	if !strings.Contains(out, "POST /users") {
		t.Errorf("warning does not name the route: %q", out)
	}
}

func TestWarnsOncePerProcessNotPerRoute(t *testing.T) {
	buf := captureValidateWarning(t)

	app := New()
	for _, p := range []string{"/a", "/b", "/c"} {
		Post[optInInput, optInOut](app, p, func(c *C[optInInput]) (*optInOut, error) {
			return &optInOut{ID: "1"}, nil
		})
	}
	if n := strings.Count(buf.String(), "no Validator is configured"); n != 1 {
		t.Fatalf("warned %d times across 3 routes, want 1", n)
	}
}

func TestNoWarningWhenValidatorConfigured(t *testing.T) {
	buf := captureValidateWarning(t)

	app := New(WithValidator(NewValidator()))
	Post[optInInput, optInOut](app, "/users", func(c *C[optInInput]) (*optInOut, error) {
		return &optInOut{ID: "1"}, nil
	})
	if buf.Len() != 0 {
		t.Fatalf("warned even though a Validator is configured: %s", buf.String())
	}
}

func TestNoWarningWithoutValidateTags(t *testing.T) {
	buf := captureValidateWarning(t)

	app := New()
	Post[optInPlain, optInOut](app, "/users", func(c *C[optInPlain]) (*optInOut, error) {
		return &optInOut{ID: "1"}, nil
	})
	if buf.Len() != 0 {
		t.Fatalf("warned for a type with no validate: tags: %s", buf.String())
	}
}
