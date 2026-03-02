package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("super-secret-key-for-testing-only")

func TestSignVerifyRoundTrip_HS256(t *testing.T) {
	claims := Claims{
		Subject: "user123",
		Issuer:  "kruda",
		Extra:   map[string]any{"role": "admin"},
	}

	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	got, err := Verify(token, testSecret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if got.Subject != "user123" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user123")
	}
	if got.Issuer != "kruda" {
		t.Errorf("Issuer = %q, want %q", got.Issuer, "kruda")
	}
	if got.Extra["role"] != "admin" {
		t.Errorf("Extra[role] = %v, want %q", got.Extra["role"], "admin")
	}
}

func TestSignVerifyRoundTrip_HS384(t *testing.T) {
	claims := Claims{Subject: "user384"}
	token, err := Sign(claims, testSecret, "HS384")
	if err != nil {
		t.Fatalf("Sign HS384: %v", err)
	}
	got, err := Verify(token, testSecret)
	if err != nil {
		t.Fatalf("Verify HS384: %v", err)
	}
	if got.Subject != "user384" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user384")
	}
}

func TestSignVerifyRoundTrip_HS512(t *testing.T) {
	claims := Claims{Subject: "user512"}
	token, err := Sign(claims, testSecret, "HS512")
	if err != nil {
		t.Fatalf("Sign HS512: %v", err)
	}
	got, err := Verify(token, testSecret)
	if err != nil {
		t.Fatalf("Verify HS512: %v", err)
	}
	if got.Subject != "user512" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user512")
	}
}

func TestSignVerifyRoundTrip_RS256(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	claims := Claims{Subject: "rsa-user", Issuer: "kruda"}
	token, err := Sign(claims, privKey, "RS256")
	if err != nil {
		t.Fatalf("Sign RS256: %v", err)
	}

	got, err := Verify(token, &privKey.PublicKey)
	if err != nil {
		t.Fatalf("Verify RS256: %v", err)
	}
	if got.Subject != "rsa-user" {
		t.Errorf("Subject = %q, want %q", got.Subject, "rsa-user")
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	claims := Claims{
		Subject:   "user",
		ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(),
	}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	_, err = Verify(token, testSecret)
	if err != ErrTokenExpired {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	claims := Claims{Subject: "user"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	_, err = Verify(token, []byte("wrong-secret"))
	if err != ErrInvalidToken {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_AlgNoneRejection(t *testing.T) {
	// Manually craft a token with alg:none
	_, err := Sign(Claims{Subject: "hacker"}, testSecret, "none")
	if err != ErrAlgNone {
		t.Errorf("Sign with none: err = %v, want ErrAlgNone", err)
	}

	// Also test verify with a crafted alg:none token
	// header: {"alg":"none","typ":"JWT"}
	header := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"
	payload := "eyJzdWIiOiJoYWNrZXIifQ"
	token := header + "." + payload + "."

	_, err = Verify(token, testSecret)
	if err != ErrAlgNone {
		t.Errorf("Verify alg:none: err = %v, want ErrAlgNone", err)
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"one part", "abc"},
		{"two parts", "abc.def"},
		{"invalid base64 header", "!!!.def.ghi"},
		{"invalid base64 payload", "eyJhbGciOiJIUzI1NiJ9.!!!.ghi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Verify(tt.token, testSecret)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestRefresh_WithinGracePeriod(t *testing.T) {
	claims := Claims{
		Subject:   "user",
		ExpiresAt: time.Now().Add(-30 * time.Second).Unix(),
		IssuedAt:  time.Now().Add(-1 * time.Hour).Unix(),
	}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Refresh with 5 minute grace period — should succeed
	newToken, err := Refresh(token, testSecret, 1*time.Hour, 5*time.Minute)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	got, err := Verify(newToken, testSecret)
	if err != nil {
		t.Fatalf("Verify refreshed: %v", err)
	}
	if got.Subject != "user" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user")
	}
	if got.ExpiresAt <= time.Now().Unix() {
		t.Error("refreshed token should have future expiration")
	}
}

func TestRefresh_ExpiredBeyondGrace(t *testing.T) {
	claims := Claims{
		Subject:   "user",
		ExpiresAt: time.Now().Add(-2 * time.Hour).Unix(),
	}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Refresh with 5 minute grace — should fail (expired 2h ago)
	_, err = Refresh(token, testSecret, 1*time.Hour, 5*time.Minute)
	if err != ErrTokenExpired {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestRefresh_RS256(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	claims := Claims{
		Subject:   "rsa-user",
		ExpiresAt: time.Now().Add(-10 * time.Second).Unix(),
	}
	token, err := Sign(claims, privKey, "RS256")
	if err != nil {
		t.Fatalf("Sign RS256: %v", err)
	}

	// Refresh: verify with public key, sign with private key
	newToken, err := Refresh(token, &privKey.PublicKey, 1*time.Hour, 5*time.Minute, privKey)
	if err != nil {
		t.Fatalf("Refresh RS256: %v", err)
	}

	got, err := Verify(newToken, &privKey.PublicKey)
	if err != nil {
		t.Fatalf("Verify refreshed RS256: %v", err)
	}
	if got.Subject != "rsa-user" {
		t.Errorf("Subject = %q, want %q", got.Subject, "rsa-user")
	}
}

func TestSign_UnsupportedAlgorithm(t *testing.T) {
	_, err := Sign(Claims{}, testSecret, "ES256")
	if err != ErrUnsupportedAlg {
		t.Errorf("err = %v, want ErrUnsupportedAlg", err)
	}
}

func TestExtractToken(t *testing.T) {
	// Test parseLookup
	source, name := parseLookup("query:token")
	if source != "query" || name != "token" {
		t.Errorf("parseLookup(query:token) = %q, %q", source, name)
	}

	source, name = parseLookup("cookie:jwt")
	if source != "cookie" || name != "jwt" {
		t.Errorf("parseLookup(cookie:jwt) = %q, %q", source, name)
	}

	source, name = parseLookup("invalid")
	if source != "header" || name != "Authorization" {
		t.Errorf("parseLookup(invalid) = %q, %q", source, name)
	}
}

func TestTokenFormat(t *testing.T) {
	claims := Claims{Subject: "user"}
	token, err := Sign(claims, testSecret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}

	// No padding characters should be present (RawURLEncoding)
	for i, part := range parts {
		if strings.Contains(part, "=") {
			t.Errorf("part %d contains padding: %q", i, part)
		}
	}
}
