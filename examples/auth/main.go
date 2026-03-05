// Example: Auth — JWT-like Authentication with HMAC-SHA256
//
// Demonstrates a token-based authentication pattern using ONLY stdlib
// crypto packages (no external JWT library):
//   - HMAC-SHA256 token generation and validation
//   - Login endpoint that issues tokens
//   - Auth middleware that validates tokens on protected routes
//   - Token payload with expiration
//
// This is a simplified JWT-like pattern. For production, consider a
// full JWT library or Kruda's future contrib/jwt package.
//
// Run: go run -tags kruda_stdjson ./examples/auth/
// Test:
//
//	curl http://localhost:3000/                                          → public
//	curl -X POST http://localhost:3000/login -d '{"username":"admin","password":"secret"}'
//	curl http://localhost:3000/protected -H "Authorization: Bearer <token>"
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Token system — HMAC-SHA256 based (no external dependencies)
// ---------------------------------------------------------------------------

// secretKey is the HMAC signing key. In production, load from environment.
var secretKey = []byte("kruda-example-secret-key-change-me")

// TokenPayload is the data encoded in the token.
type TokenPayload struct {
	Username  string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// generateToken creates an HMAC-SHA256 signed token.
// Format: base64(payload).base64(signature)
func generateToken(username string, ttl time.Duration) (string, error) {
	payload := TokenPayload{
		Username:  username,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}

	// Encode payload as JSON, then base64
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Sign with HMAC-SHA256
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payloadB64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payloadB64 + "." + sig, nil
}

// validateToken verifies the HMAC signature and checks expiration.
// Returns the payload if valid, or an error describing the failure.
func validateToken(token string) (*TokenPayload, error) {
	// Split into payload and signature
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	payloadB64, sigB64 := parts[0], parts[1]

	// Verify HMAC signature
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(payloadB64))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigB64), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var payload TokenPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	// Check expiration
	if time.Now().Unix() > payload.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	return &payload, nil
}

// ---------------------------------------------------------------------------
// Auth middleware — validates Bearer token on protected routes
// ---------------------------------------------------------------------------

// AuthMiddleware returns middleware that extracts and validates the Bearer
// token from the Authorization header. On success, it stores the username
// in the request context under the "user" key.
func AuthMiddleware() kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		// Extract Authorization header
		auth := c.Header("Authorization")
		if auth == "" {
			return kruda.Unauthorized("missing authorization header")
		}

		// Expect "Bearer <token>"
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			return kruda.Unauthorized("invalid format, expected: Bearer <token>")
		}

		// Validate the token
		payload, err := validateToken(token)
		if err != nil {
			return kruda.Unauthorized(fmt.Sprintf("invalid token: %v", err))
		}

		// Store user info in request-scoped locals for downstream handlers
		c.Set("user", payload.Username)
		return c.Next()
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// loginHandler authenticates credentials and returns a signed token.
// In production, check against a database — this example uses hardcoded creds.
func loginHandler(c *kruda.Ctx) error {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&input); err != nil {
		return kruda.BadRequest("invalid JSON body")
	}

	// Hardcoded credentials for the example
	if input.Username != "admin" || input.Password != "secret" {
		return kruda.Unauthorized("invalid credentials")
	}

	// Generate a token valid for 1 hour
	token, err := generateToken(input.Username, 1*time.Hour)
	if err != nil {
		return kruda.InternalError("failed to generate token")
	}

	return c.JSON(kruda.Map{
		"token":      token,
		"expires_in": 3600,
		"token_type": "Bearer",
	})
}

func protectedHandler(c *kruda.Ctx) error {
	// The auth middleware already validated the token and stored the user
	user, _ := c.Get("user").(string)
	return c.JSON(kruda.Map{
		"message": fmt.Sprintf("Hello, %s! You have access to this protected resource.", user),
	})
}

func profileHandler(c *kruda.Ctx) error {
	user, _ := c.Get("user").(string)
	return c.JSON(kruda.Map{
		"username": user,
		"role":     "admin",
	})
}

func homeHandler(c *kruda.Ctx) error {
	return c.JSON(kruda.Map{
		"message": "Auth example — HMAC-SHA256 token authentication",
		"try": kruda.Map{
			"POST /login":       `{"username":"admin","password":"secret"} → get token`,
			"GET /protected":    "requires Bearer token",
			"GET /protected/me": "requires Bearer token — returns profile",
			"GET /public":       "no auth required",
		},
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	app := kruda.New(kruda.NetHTTP())

	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// Public routes — no auth required
	app.Get("/", homeHandler)
	app.Post("/login", loginHandler)
	app.Get("/public", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"message": "this is a public endpoint"})
	})

	// Protected routes — auth middleware scoped to this group
	protected := app.Group("/protected")
	protected.Use(AuthMiddleware())

	protected.Get("", protectedHandler)
	protected.Get("/me", profileHandler)

	fmt.Println("Auth example listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
