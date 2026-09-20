//go:build linux || darwin

package kruda

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type compositionFixture struct {
	name     string
	groups   int
	query    string
	eligible func(*Validator) bool
	register func(*App)
}

func compositionFixtures() []compositionFixture {
	query := func(groups int) string {
		var parts []string
		for group := 1; group <= groups; group++ {
			suffix := ""
			if group > 1 {
				suffix = strconv.Itoa(group)
			}
			parts = append(parts, "id"+suffix+"=9", "page"+suffix+"=3", "active"+suffix+"=false", "score"+suffix+"=2.5", "name"+suffix+"=alice")
		}
		return strings.Join(parts, "&")
	}
	cases := []compositionFixture{
		{"fields5", 1, query(1), compositionValidationEligible5, func(app *App) {
			app.Get("/users/:id", compositionHandler5(app, func(c *C[composition5Input]) (*composition5Input, error) { return &c.In, nil }))
		}},
		{"fields10", 2, query(2), compositionValidationEligible10, func(app *App) {
			app.Get("/users/:id", compositionHandler10(app, func(c *C[composition10Input]) (*composition10Input, error) { return &c.In, nil }))
		}},
		{"fields30", 6, query(6), compositionValidationEligible30, func(app *App) {
			app.Get("/users/:id", compositionHandler30(app, func(c *C[composition30Input]) (*composition30Input, error) { return &c.In, nil }))
		}},
	}
	suffix := cases[0]
	suffix.name = "fields5_suffix30"
	for i := 0; i < 30; i++ {
		suffix.query += fmt.Sprintf("&unused%d=irrelevant-value", i)
	}
	return append(cases, suffix)
}

func compositionPipeline(app *App) {
	app.Use(func(c *Ctx) error {
		c.Set("request-source", "benchmark")
		return c.Next()
	}, func(c *Ctx) error {
		c.SetHeader("X-Benchmark", "bindgen")
		return c.Next()
	})
	app.OnParse(func(_ *Ctx, value any) error {
		switch in := value.(type) {
		case *composition5Input:
			in.Name = strings.TrimSpace(in.Name)
		case *composition10Input:
			in.Name = strings.TrimSpace(in.Name)
			in.Name2 = strings.TrimSpace(in.Name2)
		case *composition30Input:
			in.Name = strings.TrimSpace(in.Name)
			in.Name2 = strings.TrimSpace(in.Name2)
			in.Name3 = strings.TrimSpace(in.Name3)
			in.Name4 = strings.TrimSpace(in.Name4)
			in.Name5 = strings.TrimSpace(in.Name5)
			in.Name6 = strings.TrimSpace(in.Name6)
		case *compositionPostInput:
			in.Name = strings.TrimSpace(in.Name)
		}
		return nil
	})
}

func compositionApp(tb testing.TB, fixture compositionFixture, configure func(*App), wantEligible bool) *App {
	tb.Helper()
	app := New()
	compositionPipeline(app)
	if configure != nil {
		configure(app)
	}
	if got := fixture.eligible(app.config.Validator); got != wantEligible {
		tb.Fatalf("typed validation eligibility=%t, want %t", got, wantEligible)
	}
	fixture.register(app)
	app.Compile()
	return app
}

func compositionRequest(tb testing.TB, method, path, body string) *wingRequest {
	tb.Helper()
	headers := "Host: localhost\r\n"
	if body != "" {
		headers += "Content-Type: application/json\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n"
	}
	raw := []byte(method + " " + path + " HTTP/1.1\r\n" + headers + "\r\n" + body)
	req, n, ok := parseHTTPRequest(raw, noLimits)
	if !ok || req == nil || n != len(raw) {
		tb.Fatalf("request parse=%t consumed=%d/%d", ok, n, len(raw))
	}
	tb.Cleanup(func() { releaseRequest(req) })
	return req
}

func compositionExpectedJSON(groups int) string {
	var fields []string
	for group := 1; group <= groups; group++ {
		suffix, id := "", 9
		if group == 1 {
			id = 42
		} else {
			suffix = strconv.Itoa(group)
		}
		fields = append(fields, fmt.Sprintf(`"ID%s":%d,"Page%s":3,"Active%s":false,"Score%s":2.5,"Name%s":"alice"`, suffix, id, suffix, suffix, suffix, suffix))
	}
	return "{" + strings.Join(fields, ",") + "}"
}

func assertCompositionResponse(tb testing.TB, w *mockResponseWriter, want string) {
	tb.Helper()
	if w.statusCode != 200 || string(w.body) != want || w.headers.Get("X-Benchmark") != "bindgen" || w.headers.Get("Content-Type") != "application/json; charset=utf-8" {
		tb.Fatalf("response=%d %s headers=%v, want exact 200 JSON %s and pipeline header", w.statusCode, w.body, w.headers.h, want)
	}
}

func TestCompositionValidatedPipeline(t *testing.T) {
	for _, fixture := range compositionFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			app := compositionApp(t, fixture, nil, compositionTypedValidationExpected)
			req := compositionRequest(t, "GET", "/users/42?"+fixture.query, "")
			response := newMockResponse()
			app.ServeKruda(response, req)
			assertCompositionResponse(t, response, compositionExpectedJSON(fixture.groups))
		})
	}
}

func TestCompositionValidationConformance(t *testing.T) {
	fixture := compositionFixtures()[0]
	for _, tt := range []struct {
		name, path, query, hook string
		status                  int
		message                 string
		fields                  []FieldError
	}{
		{name: "query_order", path: "42", query: "page=bad&id=bad", status: 400, message: `invalid query parameter "id": expected int64`},
		{name: "query_before_path", path: "bad", query: "page=bad", status: 400, message: `invalid query parameter "page": expected int`},
		{name: "path_error", path: "bad", query: fixture.query, status: 400, message: `invalid path parameter "id": expected int64`},
		{name: "repair_before_validation", path: "42", query: strings.Replace(fixture.query, "page=3", "page=0", 1), hook: "repair", status: 200},
		{name: "invalidate_before_validation", path: "42", query: fixture.query, hook: "invalidate", status: 422, fields: []FieldError{{Field: "page", Rule: "min", Param: "1", Message: "page must be at least 1", Value: "0"}}},
		{name: "ordered_validation", path: "0", query: "page=0&score=101", status: 422, fields: []FieldError{
			{Field: "id", Rule: "min", Param: "1", Message: "id must be at least 1", Value: "0"},
			{Field: "page", Rule: "min", Param: "1", Message: "page must be at least 1", Value: "0"},
			{Field: "score", Rule: "max", Param: "100", Message: "score must be at most 100", Value: "101"},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed := false
			var gotError error
			var fields []FieldError
			app := compositionApp(t, fixture, func(app *App) {
				app.OnParse(func(_ *Ctx, value any) error {
					parsed = true
					in := value.(*composition5Input)
					if tt.hook == "repair" {
						in.Page = 3
					}
					if tt.hook == "invalidate" {
						in.Page = 0
					}
					return nil
				})
				app.OnError(func(_ *Ctx, err error) {
					gotError = err
					var ve *ValidationError
					if errors.As(err, &ve) {
						fields = append([]FieldError(nil), ve.Errors...)
					}
				})
			}, compositionTypedValidationExpected)
			w := newMockResponse()
			app.ServeKruda(w, compositionRequest(t, "GET", "/users/"+tt.path+"?"+tt.query, ""))
			if w.statusCode != tt.status || parsed != (tt.status != 400) || !reflect.DeepEqual(fields, tt.fields) {
				t.Fatalf("status=%d parsed=%t errors=%+v; want %d errors=%+v", w.statusCode, parsed, fields, tt.status, tt.fields)
			}
			if tt.message != "" {
				var ke *KrudaError
				if !errors.As(gotError, &ke) || ke.Code != 400 || ke.Message != tt.message || ke.Err != nil {
					t.Fatalf("error=%#v, want exact 400 %q", gotError, tt.message)
				}
			}
			if tt.status == 200 {
				assertCompositionResponse(t, w, compositionExpectedJSON(1))
			}
		})
	}
}

func TestCompositionCustomValidatorFallback(t *testing.T) {
	calls := 0
	var fields []FieldError
	app := compositionApp(t, compositionFixtures()[0], func(app *App) {
		original := app.config.Validator.rules["min"]
		app.config.Validator.Register("min", func(value any, param string) bool {
			calls++
			if _, ok := value.(int); ok {
				return false
			}
			return original(value, param)
		})
		app.OnError(func(_ *Ctx, err error) {
			var ve *ValidationError
			if errors.As(err, &ve) {
				fields = append([]FieldError(nil), ve.Errors...)
			}
		})
	}, false)
	w := newMockResponse()
	app.ServeKruda(w, compositionRequest(t, "GET", "/users/42?"+compositionFixtures()[0].query, ""))
	want := []FieldError{{Field: "page", Rule: "min", Param: "1", Message: "page must be at least 1", Value: "3"}}
	if w.statusCode != 422 || calls != 4 || !reflect.DeepEqual(fields, want) {
		t.Fatalf("status=%d calls=%d errors=%+v, want 422/4/%+v", w.statusCode, calls, fields, want)
	}
}

func compositionPostApp() *App {
	app := New()
	compositionPipeline(app)
	Post[compositionPostInput, compositionPostInput](app, "/items", func(c *C[compositionPostInput]) (*compositionPostInput, error) { return &c.In, nil })
	app.Compile()
	return app
}

func TestCompositionJSONPostControl(t *testing.T) {
	w := newMockResponse()
	compositionPostApp().ServeKruda(w, compositionRequest(t, "POST", "/items", `{"name":" alice ","count":3}`))
	assertCompositionResponse(t, w, `{"name":"alice","count":3}`)
}
