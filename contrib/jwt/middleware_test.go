package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// newJWTTestApp creates a Kruda app with JWT middleware and a simple protected handler.
func newJWTTestApp(cfg ...Config) (*kruda.App, *httptest.Server) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(cfg...))
	app.Get("/protected", func(c *kruda.Ctx) error {
		claims, _ := c.Get("jwt_claims").(*Claims)
		if claims == nil {
			return c.Status(500).JSON(map[string]string{"error": "no claims"})
		}
		return c.JSON(map[string]string{"sub": claims.Subject, "iss": claims.Issuer})
	})
	app.Compile()
	srv := httptest.NewServer(app)
	return app, srv
}

func TestMiddleware_ValidBearerToken(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret})
	defer srv.Close()

	claims := Claims{Subject: "user123", Issuer: "kruda-test"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "user123" {
		t.Errorf("subject = %q, want %q", body["sub"], "user123")
	}
	if body["iss"] != "kruda-test" {
		t.Errorf("issuer = %q, want %q", body["iss"], "kruda-test")
	}
}

func TestMiddleware_MissingToken(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "missing_token" {
		t.Errorf("error = %q, want %q", body["error"], "missing_token")
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret})
	defer srv.Close()

	claims := Claims{
		Subject:   "user",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "token_expired" {
		t.Errorf("error = %q, want %q", body["error"], "token_expired")
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret})
	defer srv.Close()

	// Sign with wrong secret
	claims := Claims{Subject: "user"}
	token, err := Sign(claims, []byte("wrong-secret"))
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "invalid_token" {
		t.Errorf("error = %q, want %q", body["error"], "invalid_token")
	}
}

func TestMiddleware_QueryExtraction(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret, Lookup: "query:token"})
	defer srv.Close()

	claims := Claims{Subject: "query-user"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/protected?token=" + token)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "query-user" {
		t.Errorf("subject = %q, want %q", body["sub"], "query-user")
	}
}

func TestMiddleware_CookieExtraction(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret, Lookup: "cookie:jwt"})
	defer srv.Close()

	claims := Claims{Subject: "cookie-user"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: token})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "cookie-user" {
		t.Errorf("subject = %q, want %q", body["sub"], "cookie-user")
	}
}

func TestMiddleware_CustomHeader(t *testing.T) {
	_, srv := newJWTTestApp(Config{Secret: testSecret, Lookup: "header:X-API-Token"})
	defer srv.Close()

	claims := Claims{Subject: "header-user"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("X-API-Token", token) // no Bearer prefix
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "header-user" {
		t.Errorf("subject = %q, want %q", body["sub"], "header-user")
	}
}

func TestMiddleware_SkipFunction(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Secret: testSecret,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/health"
		},
	}))
	app.Get("/protected", func(c *kruda.Ctx) error { return c.Text("secret") })
	app.Get("/health", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// /health should bypass JWT
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("/health: expected 200, got %d", resp.StatusCode)
	}

	// /protected without token should fail
	resp, err = http.Get(srv.URL + "/protected")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("/protected without token: expected 401, got %d", resp.StatusCode)
	}
}

func TestMiddleware_RSAPublicKey(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	_, srv := newJWTTestApp(Config{PublicKey: &privKey.PublicKey})
	defer srv.Close()

	claims := Claims{Subject: "rsa-user"}
	token, err := Sign(claims, privKey, "RS256")
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "rsa-user" {
		t.Errorf("subject = %q, want %q", body["sub"], "rsa-user")
	}
}

func TestMiddleware_ClaimsInContext(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{Secret: testSecret}))
	app.Get("/claims", func(c *kruda.Ctx) error {
		claims, ok := c.Get("jwt_claims").(*Claims)
		if !ok || claims == nil {
			return c.Status(500).JSON(map[string]string{"error": "claims not found"})
		}
		return c.JSON(map[string]any{
			"sub":  claims.Subject,
			"role": claims.Extra["role"],
		})
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	claims := Claims{
		Subject: "ctx-user",
		Extra:   map[string]any{"role": "admin"},
	}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", srv.URL+"/claims", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "ctx-user" {
		t.Errorf("sub = %v, want %q", body["sub"], "ctx-user")
	}
	if body["role"] != "admin" {
		t.Errorf("role = %v, want %q", body["role"], "admin")
	}
}
