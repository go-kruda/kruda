package kruda

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Property 1: Pool Safety — No Stale Data Leak
//
// For any two consecutive requests r1 and r2 handled by the same Ctx from the
// Pool, after r1 completes and r2 begins, the Ctx should have method and path
// matching r2, status reset to 200, responded reset to false, empty params,
// no stale custom headers from r1, no stale cookies from r1, no stale locals
// from r1, and no stale body data from r1.
//
// **Validates: Requirements 7.1, 7.2, 7.3, 7.4, 7.5, 7.6**
// ---------------------------------------------------------------------------

// staleCheckResult captures what the second request's handler observed.
type staleCheckResult struct {
	found  bool
	detail string
}

// TestPropertyPoolSafety_NoStaleDataLeak uses testing/quick to generate random
// stale data for request 1, then verifies request 2 sees none of it.
//
// Approach: single-goroutine ServeHTTP calls — sync.Pool reliably returns the
// same Ctx when Get/Put happen on the same goroutine with no concurrent access.
func TestPropertyPoolSafety_NoStaleDataLeak(t *testing.T) {
	app := New()

	// Shared state between handler invocations — safe because single goroutine.
	var reqNum int
	var stale staleCheckResult
	// Capture what random data request 1 set, so request 2 can check for it.
	var r1HeaderKey, r1HeaderVal string
	var r1CookieName string
	var r1LocalKey string

	app.Post("/:id", func(c *Ctx) error {
		reqNum++
		if reqNum%2 == 1 {
			// === Request 1: pollute the context with stale data ===
			c.SetHeader(r1HeaderKey, r1HeaderVal)
			c.SetCookie(&Cookie{Name: r1CookieName, Value: "stale-cookie-val"})
			c.Set(r1LocalKey, "stale-local-val")
			// Read body to set bodyParsed + bodyBytes
			c.BodyBytes()
			// Set a non-default status and respond
			return c.Status(201).JSON(map[string]string{"polluted": "true"})
		}

		// === Request 2: verify no stale data from request 1 ===

		// 7.1: method and path must match current request
		if c.method != "POST" {
			stale = staleCheckResult{true, fmt.Sprintf("method=%q, want POST", c.method)}
			return c.Text("fail")
		}
		if c.path != "/clean" {
			stale = staleCheckResult{true, fmt.Sprintf("path=%q, want /clean", c.path)}
			return c.Text("fail")
		}

		// 7.2: status=200, responded=false
		if c.status != 200 {
			stale = staleCheckResult{true, fmt.Sprintf("status=%d, want 200", c.status)}
			return c.Text("fail")
		}
		if c.responded {
			stale = staleCheckResult{true, "responded=true, want false"}
			return c.Text("fail")
		}

		// 7.2: handlers should be set (not nil — they're set by ServeHTTP before calling)
		// routeIndex should be 0
		if c.routeIndex != 0 {
			stale = staleCheckResult{true, fmt.Sprintf("routeIndex=%d, want 0", c.routeIndex)}
			return c.Text("fail")
		}

		// 7.3: no stale response headers from r1
		if len(c.respHeaders) > 0 {
			stale = staleCheckResult{true, fmt.Sprintf("respHeaders has %d entries, want 0", len(c.respHeaders))}
			return c.Text("fail")
		}
		if c.contentType != "" {
			stale = staleCheckResult{true, fmt.Sprintf("contentType=%q, want empty", c.contentType)}
			return c.Text("fail")
		}

		// 7.4: no stale cookies from r1
		if len(c.cookies) > 0 {
			stale = staleCheckResult{true, fmt.Sprintf("cookies has %d entries, want 0", len(c.cookies))}
			return c.Text("fail")
		}

		// 7.5: no stale locals from r1
		if c.Get(r1LocalKey) != nil {
			stale = staleCheckResult{true, fmt.Sprintf("locals[%q]=%v, want nil", r1LocalKey, c.Get(r1LocalKey))}
			return c.Text("fail")
		}

		// 7.6: no stale body data from r1
		if c.bodyParsed {
			stale = staleCheckResult{true, "bodyParsed=true, want false"}
			return c.Text("fail")
		}
		// bodyBytes should not contain stale data — since bodyParsed is false,
		// calling BodyBytes() would re-read from the new request, which is correct.
		// But we also check the raw field isn't leaking.
		if len(c.bodyBytes) > 0 {
			stale = staleCheckResult{true, fmt.Sprintf("bodyBytes has %d bytes of stale data", len(c.bodyBytes))}
			return c.Text("fail")
		}

		// Additional: params should be empty (cleared before router.find)
		if c.params.count > 1 { // router sets :id param for this request
			stale = staleCheckResult{true, fmt.Sprintf("params has %d entries (expected 1 for :id)", c.params.count)}
			return c.Text("fail")
		}

		return c.Text("clean")
	})
	app.Compile()

	f := func(iterations uint8, headerSuffix uint8, cookieSuffix uint8, localSuffix uint8) bool {
		n := int(iterations)%20 + 1 // 1-20 iterations per quick.Check call
		for i := 0; i < n; i++ {
			reqNum = 0
			stale = staleCheckResult{}

			// Generate random-ish stale data keys using the quick-generated values
			r1HeaderKey = fmt.Sprintf("X-Stale-%d", headerSuffix)
			r1HeaderVal = fmt.Sprintf("val-%d-%d", headerSuffix, i)
			r1CookieName = fmt.Sprintf("stale_cookie_%d", cookieSuffix)
			r1LocalKey = fmt.Sprintf("staleLocal_%d", localSuffix)

			// Request 1: pollute the context
			body := strings.NewReader(fmt.Sprintf(`{"iter":%d}`, i))
			r1 := httptest.NewRequest("POST", "/pollute", body)
			r1.Header.Set("Content-Type", "application/json")
			w1 := httptest.NewRecorder()
			app.ServeHTTP(w1, r1)

			if w1.Code != 201 {
				t.Logf("request 1 failed: status=%d, body=%s", w1.Code, w1.Body.String())
				return false
			}

			// Request 2: verify clean state (same goroutine → likely same Ctx from pool)
			r2 := httptest.NewRequest("POST", "/clean", nil)
			w2 := httptest.NewRecorder()
			app.ServeHTTP(w2, r2)

			if stale.found {
				t.Logf("iteration %d: stale data detected: %s", i, stale.detail)
				t.Logf("  r1 header: %s=%s, cookie: %s, local: %s",
					r1HeaderKey, r1HeaderVal, r1CookieName, r1LocalKey)
				return false
			}
		}
		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property Pool Safety violated: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Property 2: writeHeaders Completeness
//
// For any Ctx with a random combination of content-type, content-length,
// custom headers, cookies, and security headers configured, after
// writeHeaders() executes, the response writer should contain all configured
// headers with correct values — regardless of whether the fast path or
// complex path was taken.
//
// **Validates: Requirements 3.2, 3.3, 3.5**
// ---------------------------------------------------------------------------

// TestPropertyWriteHeadersCompleteness uses testing/quick to generate random
// header configurations and verifies that writeHeaders() writes them all
// correctly through the real ServeHTTP path.
//
// We use JSON()/HTML() (which go through sendBytes → writeHeaders) rather than
// Text() which has a net/http fast path that bypasses writeHeaders entirely.
func TestPropertyWriteHeadersCompleteness(t *testing.T) {
	f := func(
		hasCustomHeader bool,
		hasCookie bool,
		hasCacheControl bool,
		hasLocation bool,
		useSecurityHeaders bool,
	) bool {
		var app *App
		if useSecurityHeaders {
			app = New(WithSecureHeaders())
		} else {
			app = New()
		}

		app.Get("/test", func(c *Ctx) error {
			if hasCustomHeader {
				c.SetHeader("X-Custom", "custom-value")
			}
			if hasCookie {
				c.SetCookie(&Cookie{Name: "testcookie", Value: "cookieval"})
			}
			if hasCacheControl {
				c.SetHeader("Cache-Control", "no-cache")
			}
			if hasLocation {
				// Redirect sets location and calls send() → writeHeaders()
				return c.Redirect("/other", 302)
			}
			// Use HTML() which goes through sendBytes → writeHeaders
			// (unlike Text() which has a net/http fast path bypassing writeHeaders)
			return c.HTML("<p>hello</p>")
		})
		app.Compile()

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)

		resp := w.Result()

		// Verify custom header
		if hasCustomHeader && !hasLocation {
			if resp.Header.Get("X-Custom") != "custom-value" {
				t.Logf("FAIL: X-Custom missing or wrong, got %q", resp.Header.Get("X-Custom"))
				return false
			}
		}

		// Verify cookie
		if hasCookie && !hasLocation {
			cookies := resp.Header.Values("Set-Cookie")
			found := false
			for _, c := range cookies {
				if strings.Contains(c, "testcookie=cookieval") {
					found = true
					break
				}
			}
			if !found {
				t.Logf("FAIL: Set-Cookie testcookie=cookieval not found in %v", cookies)
				return false
			}
		}

		// Verify Cache-Control
		if hasCacheControl && !hasLocation {
			if resp.Header.Get("Cache-Control") != "no-cache" {
				t.Logf("FAIL: Cache-Control missing or wrong, got %q", resp.Header.Get("Cache-Control"))
				return false
			}
		}

		// Verify Location header on redirect
		if hasLocation {
			if resp.Header.Get("Location") != "/other" {
				t.Logf("FAIL: Location missing or wrong, got %q", resp.Header.Get("Location"))
				return false
			}
			if resp.StatusCode != 302 {
				t.Logf("FAIL: redirect status=%d, want 302", resp.StatusCode)
				return false
			}
		}

		// Verify Content-Type for non-redirect responses
		if !hasLocation {
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, "text/html") {
				t.Logf("FAIL: Content-Type missing or wrong, got %q", ct)
				return false
			}
		}

		// Verify security headers presence/absence
		if useSecurityHeaders {
			if resp.Header.Get("X-Content-Type-Options") == "" {
				t.Logf("FAIL: security header X-Content-Type-Options missing with WithSecureHeaders()")
				return false
			}
		} else {
			if resp.Header.Get("X-Content-Type-Options") != "" {
				t.Logf("FAIL: security header X-Content-Type-Options present without WithSecureHeaders")
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("Property writeHeaders Completeness violated: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Property 9: Security Opt-in Isolation
//
// For any App created with kruda.New() without WithSecurity, the response
// should contain zero security headers and no path normalization should occur.
// Conversely, for any App created with WithSecurity(), every response should
// contain the full set of security headers.
//
// **Validates: Requirements 5.1, 5.2**
// ---------------------------------------------------------------------------

// TestPropertySecurityOptInIsolation uses testing/quick to generate random
// request paths and verifies that:
//   - Bare metal app (New()) produces zero security headers
//   - Security app (New(WithSecurity())) produces all security headers
func TestPropertySecurityOptInIsolation(t *testing.T) {
	// The four security headers that WithSecurity() should produce
	// (from SecurityConfig defaults in config.go).
	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-Xss-Protection",
		"Referrer-Policy",
	}

	f := func(pathSuffix uint8) bool {
		path := fmt.Sprintf("/test%d", pathSuffix)

		// === Bare metal: zero security headers ===
		bareApp := New()
		bareApp.Get(path, func(c *Ctx) error {
			// Use HTML() which goes through sendBytes → writeHeaders
			// (Text() has a net/http fast path that bypasses writeHeaders entirely)
			return c.HTML("<p>bare</p>")
		})
		bareApp.Compile()

		r1 := httptest.NewRequest("GET", path, nil)
		w1 := httptest.NewRecorder()
		bareApp.ServeHTTP(w1, r1)

		for _, h := range securityHeaders {
			if w1.Header().Get(h) != "" {
				t.Logf("bare metal has security header %s=%q", h, w1.Header().Get(h))
				return false
			}
		}

		// === WithSecurity: all security headers present ===
		secApp := New(WithSecurity())
		secApp.Get(path, func(c *Ctx) error {
			return c.HTML("<p>secure</p>")
		})
		secApp.Compile()

		r2 := httptest.NewRequest("GET", path, nil)
		w2 := httptest.NewRecorder()
		secApp.ServeHTTP(w2, r2)

		for _, h := range securityHeaders {
			if w2.Header().Get(h) == "" {
				t.Logf("WithSecurity missing header %s", h)
				return false
			}
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("Property Security Opt-in Isolation violated: %v", err)
	}
}
