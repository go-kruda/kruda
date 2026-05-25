//go:build linux || darwin

package kruda

import "testing"

func TestServeWingSingleHandlerFlushesLazyBody(t *testing.T) {
	app := New()
	resp := acquireResponse()
	defer releaseResponse(resp)
	req := &wingRequest{method: "GET", path: "/fast", keepAlive: true}

	handled := app.serveWingSingleHandler(resp, req, func(c *Ctx) error {
		if got := c.Method(); got != "GET" {
			t.Fatalf("Method = %q, want GET", got)
		}
		if got := c.Path(); got != "/fast" {
			t.Fatalf("Path = %q, want /fast", got)
		}
		c.SetHeader("X-Test", "yes")
		c.SetContentType("text/plain; charset=utf-8").SetBody([]byte("ok"))
		return nil
	})

	if !handled {
		t.Fatal("Wing single handler fast path was not used")
	}
	if resp.status != 200 {
		t.Fatalf("status = %d, want 200", resp.status)
	}
	if got := string(resp.body); got != "ok" {
		t.Fatalf("body = %q, want ok", got)
	}
	if got := resp.headers.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
	}
	if got := resp.headers.Get("X-Test"); got != "yes" {
		t.Fatalf("X-Test = %q, want yes", got)
	}
}

func TestServeWingSingleHandlerPanicStillReportsHandled(t *testing.T) {
	app := New()
	resp := acquireResponse()
	defer releaseResponse(resp)
	req := &wingRequest{method: "GET", path: "/panic", keepAlive: true}

	handled := app.serveWingSingleHandler(resp, req, func(c *Ctx) error {
		panic("boom")
	})

	if !handled {
		t.Fatal("Wing panic recovery must still report the request as handled")
	}
	if resp.status != 500 {
		t.Fatalf("status = %d, want 500", resp.status)
	}
	if len(resp.body) == 0 {
		t.Fatal("panic response body is empty")
	}
}

func TestServeWingSingleHandlerSkipsLifecycleApps(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error { return nil })
	app.Compile()
	resp := acquireResponse()
	defer releaseResponse(resp)
	req := &wingRequest{method: "GET", path: "/lifecycle", keepAlive: true}
	handlerRan := false

	handled := app.serveWingSingleHandler(resp, req, func(c *Ctx) error {
		handlerRan = true
		return c.Text("wrong")
	})

	if handled {
		t.Fatal("Wing single handler fast path should not handle lifecycle apps")
	}
	if handlerRan {
		t.Fatal("Wing single handler fast path called handler despite lifecycle hooks")
	}
}
