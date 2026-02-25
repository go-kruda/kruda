package kruda

import (
	"testing"
)

func TestTestClientGet(t *testing.T) {
	app := New()
	app.Get("/hello", func(c *Ctx) error {
		return c.JSON(Map{"msg": "hello"})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/hello")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	var body Map
	if err := resp.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["msg"] != "hello" {
		t.Fatalf("expected msg=hello, got %v", body["msg"])
	}
}

func TestTestClientPost(t *testing.T) {
	app := New()
	app.Post("/echo", func(c *Ctx) error {
		var input Map
		if err := c.Bind(&input); err != nil {
			return err
		}
		return c.JSON(input)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Post("/echo", Map{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	var body Map
	if err := resp.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "test" {
		t.Fatalf("expected name=test, got %v", body["name"])
	}
}

func TestTestClientPut(t *testing.T) {
	app := New()
	app.Put("/items/:id", func(c *Ctx) error {
		return c.JSON(Map{
			"id":   c.Param("id"),
			"name": c.BodyString(),
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Put("/items/42", Map{"name": "updated"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	var body Map
	if err := resp.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["id"] != "42" {
		t.Fatalf("expected id=42, got %v", body["id"])
	}
}

func TestTestClientDelete(t *testing.T) {
	app := New()
	app.Delete("/items/:id", func(c *Ctx) error {
		return c.Status(204).NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Delete("/items/1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode())
	}
}

func TestTestClientHeaders(t *testing.T) {
	app := New()
	app.Get("/check-header", func(c *Ctx) error {
		return c.Text(c.Header("X-Custom"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.WithHeader("X-Custom", "myval").Get("/check-header")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "myval" {
		t.Fatalf("expected body=myval, got %q", resp.BodyString())
	}
}

func TestTestClientCookies(t *testing.T) {
	app := New()
	app.Get("/check-cookie", func(c *Ctx) error {
		return c.Text(c.Cookie("session"))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.WithCookie("session", "abc123").Get("/check-cookie")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "abc123" {
		t.Fatalf("expected body=abc123, got %q", resp.BodyString())
	}
}

func TestTestClientRequestBuilder(t *testing.T) {
	app := New()
	app.Post("/echo", func(c *Ctx) error {
		var input Map
		if err := c.Bind(&input); err != nil {
			return err
		}
		return c.JSON(Map{
			"header": c.Header("X-Req"),
			"cookie": c.Cookie("tok"),
			"body":   input,
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("POST", "/echo").
		Header("X-Req", "val").
		Cookie("tok", "xyz").
		Body(Map{"a": 1}).
		ContentType("application/json").
		Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	var body Map
	if err := resp.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["header"] != "val" {
		t.Fatalf("expected header=val, got %v", body["header"])
	}
	if body["cookie"] != "xyz" {
		t.Fatalf("expected cookie=xyz, got %v", body["cookie"])
	}
	inner, ok := body["body"].(map[string]any)
	if !ok {
		t.Fatal("expected body to be a map")
	}
	// JSON numbers unmarshal as float64
	if inner["a"] != float64(1) {
		t.Fatalf("expected body.a=1, got %v", inner["a"])
	}
}

func TestTestClientQueryParams(t *testing.T) {
	app := New()
	app.Get("/search", func(c *Ctx) error {
		return c.JSON(Map{
			"q":    c.Query("q"),
			"page": c.Query("page"),
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("GET", "/search").
		Query("q", "hello").
		Query("page", "2").
		Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	var body Map
	if err := resp.JSON(&body); err != nil {
		t.Fatal(err)
	}
	if body["q"] != "hello" {
		t.Fatalf("expected q=hello, got %v", body["q"])
	}
	if body["page"] != "2" {
		t.Fatalf("expected page=2, got %v", body["page"])
	}
}

func TestTestClientRawBody(t *testing.T) {
	app := New()
	app.Post("/raw", func(c *Ctx) error {
		return c.Text(c.BodyString())
	})
	app.Compile()

	tc := NewTestClient(app)

	// Test with []byte — no JSON marshaling
	resp, err := tc.Post("/raw", []byte("raw bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "raw bytes" {
		t.Fatalf("expected body='raw bytes', got %q", resp.BodyString())
	}

	// Test with string — no JSON marshaling
	resp, err = tc.Post("/raw", "raw string")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "raw string" {
		t.Fatalf("expected body='raw string', got %q", resp.BodyString())
	}
}

func TestTestClientJSONResponse(t *testing.T) {
	app := New()
	app.Get("/data", func(c *Ctx) error {
		return c.JSON(Map{
			"id":     42,
			"name":   "kruda",
			"active": true,
		})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/data")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}

	var result struct {
		ID     float64 `json:"id"`
		Name   string  `json:"name"`
		Active bool    `json:"active"`
	}
	if err := resp.JSON(&result); err != nil {
		t.Fatal(err)
	}
	if result.ID != 42 {
		t.Fatalf("expected id=42, got %v", result.ID)
	}
	if result.Name != "kruda" {
		t.Fatalf("expected name=kruda, got %v", result.Name)
	}
	if !result.Active {
		t.Fatal("expected active=true")
	}
}

func TestTestClientHeadersCleared(t *testing.T) {
	app := New()
	app.Get("/check", func(c *Ctx) error {
		val := c.Header("X-Once")
		if val == "" {
			val = "empty"
		}
		return c.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)

	// First request with header
	resp, err := tc.WithHeader("X-Once", "val").Get("/check")
	if err != nil {
		t.Fatal(err)
	}
	if resp.BodyString() != "val" {
		t.Fatalf("expected body=val, got %q", resp.BodyString())
	}

	// Second request without header — should be cleared
	resp, err = tc.Get("/check")
	if err != nil {
		t.Fatal(err)
	}
	if resp.BodyString() != "empty" {
		t.Fatalf("expected body=empty after header cleared, got %q", resp.BodyString())
	}
}
