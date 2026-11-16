package kruda

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type namedText string

func serveNamed(t *testing.T, app *App, req *http.Request) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func TestNamedStringFormFieldBinds(t *testing.T) {
	type input struct {
		Title namedText `form:"title"`
	}
	type output struct {
		Title string `json:"title"`
	}
	app := New()
	Post[input, output](app, "/form", func(c *C[input]) (*output, error) {
		return &output{Title: string(c.In.Title)}, nil
	})
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("title", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/form", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if code, body := serveNamed(t, app, req); code != 200 || body != `{"title":"hello"}` {
		t.Fatalf("got %d %s, want 200 {\"title\":\"hello\"}", code, body)
	}
}

func TestStringRulesAcceptNamedStrings(t *testing.T) {
	rules := builtinRules()
	for _, tc := range []struct {
		rule, param, good, bad string
	}{
		{"email", "", "a@b.co", "nope"},
		{"url", "", "https://kruda.dev", "nope"},
		{"uuid", "", "123e4567-e89b-12d3-a456-426614174000", "nope"},
		{"alpha", "", "abc", "abc1"},
		{"alphanum", "", "abc1", "abc-1"},
		{"numeric", "", "-42", "4x"},
		{"contains", "ok", "look", "nope"},
		{"startswith", "ab", "abc", "cab"},
		{"endswith", "bc", "abc", "bca"},
	} {
		t.Run(tc.rule, func(t *testing.T) {
			fn := rules[tc.rule]
			if fn == nil {
				t.Fatalf("no built-in rule %q", tc.rule)
			}
			for _, v := range []any{tc.good, namedText(tc.good)} {
				if !fn(v, tc.param) {
					t.Errorf("%s(%T %q) = false, want true", tc.rule, v, tc.good)
				}
			}
			for _, v := range []any{tc.bad, namedText(tc.bad), 42} {
				if fn(v, tc.param) {
					t.Errorf("%s(%T %v) = true, want false", tc.rule, v, v)
				}
			}
		})
	}
}

func TestNamedStringValidationThroughRoute(t *testing.T) {
	type input struct {
		Email namedText `query:"email" validate:"required,email"`
	}
	type output struct {
		Email string `json:"email"`
	}
	app := New()
	Get[input, output](app, "/v", func(c *C[input]) (*output, error) {
		return &output{Email: string(c.In.Email)}, nil
	})
	if code, body := serveNamed(t, app, httptest.NewRequest(http.MethodGet, "/v?email="+url.QueryEscape("a@b.co"), nil)); code != 200 {
		t.Fatalf("valid email: got %d %s, want 200", code, body)
	}
	if code, body := serveNamed(t, app, httptest.NewRequest(http.MethodGet, "/v?email=nope", nil)); code != 422 {
		t.Fatalf("invalid email: got %d %s, want 422", code, body)
	}
}

func TestCustomRuleStillReceivesNamedType(t *testing.T) {
	type input struct {
		Value namedText `query:"v" validate:"isnamed"`
	}
	type output struct{}
	v := NewValidator().Register("isnamed", func(value any, _ string) bool {
		_, ok := value.(namedText)
		return ok
	})
	app := New(WithValidator(v))
	Get[input, output](app, "/c", func(c *C[input]) (*output, error) { return &output{}, nil })
	if code, body := serveNamed(t, app, httptest.NewRequest(http.MethodGet, "/c?v=x", nil)); code != 200 {
		t.Fatalf("got %d %s, want 200: custom rules must keep receiving the field's own type", code, body)
	}
}
