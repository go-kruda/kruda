//go:build !windows

package kruda

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/valyala/fasthttp"
)

func TestContextDoesNotRetainPreviousTransportIdentity(t *testing.T) {
	type clientKey struct{}
	app := New()
	pooled := app.ctxPool.Get().(*Ctx)
	app.ctxPool = sync.Pool{New: func() any { return pooled }}
	app.Get("/", func(c *Ctx) error {
		owner := c.Cookie("_session")
		if previous := c.Get("owner"); previous != nil {
			t.Errorf("retained locals: %v", previous)
		}
		c.Set("owner", owner)
		c.SetHeader("X-Owner", owner)
		c.SetCookie(&Cookie{Name: "owner", Value: owner})
		value, _ := c.Context().Value(clientKey{}).(string)
		return c.JSON(Map{"identity": owner + "|" + c.Header("X-Client") + "|" + value})
	})
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Cookie", "_session=alice")
	request.Header.Set("X-Client", "alice")
	request = request.WithContext(context.WithValue(request.Context(), clientKey{}, "alice"))
	first := httptest.NewRecorder()
	app.ServeHTTP(first, request)
	if transportIdentity(t, first.Body.Bytes()) != "alice|alice|alice" {
		t.Fatal(first.Body.String())
	}
	generic := httptest.NewRecorder()
	genericRequest := httptest.NewRequest("GET", "/", nil)
	genericRequest.Header.Set("Cookie", "_session=charlie")
	genericRequest.Header.Set("X-Client", "charlie")
	genericRequest = genericRequest.WithContext(context.WithValue(genericRequest.Context(), clientKey{}, "charlie"))
	app.ServeKruda(&fastNetHTTPResponseWriter{w: generic, statusCode: 200}, &fastNetHTTPRequest{r: genericRequest})
	if transportIdentity(t, generic.Body.Bytes()) != "charlie|charlie|charlie" || generic.Header().Get("X-Owner") != "charlie" {
		t.Fatalf("generic reused transport state: %s", generic.Body.String())
	}
	secondHTTP := httptest.NewRecorder()
	app.ServeHTTP(secondHTTP, request)
	var fast fasthttp.RequestCtx
	fast.Request.SetRequestURI("/")
	fast.Request.Header.SetMethod("GET")
	fast.Request.Header.SetCookie("_session", "bob")
	fast.Request.Header.Set("X-Client", "bob")
	fast.SetUserValue(clientKey{}, "bob")
	app.ServeFast(&fast)
	if got := transportIdentity(t, fast.Response.Body()); got != "bob|bob|bob" {
		t.Fatalf("retained previous transport identity: %q", got)
	}
	if string(fast.Response.Header.Peek("X-Owner")) != "bob" || secondHTTP.Header().Get("X-Owner") != "alice" {
		t.Fatal("response writer retained previous transport")
	}
	if !strings.Contains(string(fast.Response.Header.Peek("Set-Cookie")), "owner=bob") || strings.Contains(string(fast.Response.Header.Peek("Set-Cookie")), "alice") {
		t.Fatal("retained response cookie")
	}
	final := httptest.NewRecorder()
	app.ServeHTTP(final, request)
	if transportIdentity(t, final.Body.Bytes()) != "alice|alice|alice" {
		t.Fatal(final.Body.String())
	}
}

func transportIdentity(t *testing.T, body []byte) string {
	t.Helper()
	var value map[string]string
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	return value["identity"]
}
