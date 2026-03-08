package jwt

import (
	"math"
	"testing"
	"testing/quick"
	"time"
)

// TestPropertySignVerifyRoundTrip verifies that for all valid Claims,
// signing then verifying produces equivalent claims.
//
// **Validates: Requirements 9.15**
func TestPropertySignVerifyRoundTrip(t *testing.T) {
	secret := []byte("pbt-test-secret-key-32bytes!!")

	algorithms := []string{"HS256", "HS384", "HS512"}

	for _, alg := range algorithms {
		alg := alg
		t.Run(alg, func(t *testing.T) {
			f := func(sub, iss, aud, jti string, extraKey string, extraVal string) bool {
				// Skip empty extra keys (JSON won't round-trip well)
				extra := map[string]any{}
				if extraKey != "" {
					extra[extraKey] = extraVal
				}

				claims := Claims{
					Subject:   sub,
					Issuer:    iss,
					Audience:  aud,
					ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
					IssuedAt:  time.Now().Unix(),
					NotBefore: time.Now().Add(-1 * time.Minute).Unix(),
					ID:        jti,
				}
				if len(extra) > 0 {
					claims.Extra = extra
				}

				token, err := Sign(claims, secret, alg)
				if err != nil {
					t.Logf("Sign failed: %v", err)
					return false
				}

				got, err := Verify(token, secret)
				if err != nil {
					t.Logf("Verify failed: %v", err)
					return false
				}

				if got.Subject != claims.Subject {
					t.Logf("Subject mismatch: %q != %q", got.Subject, claims.Subject)
					return false
				}
				if got.Issuer != claims.Issuer {
					t.Logf("Issuer mismatch: %q != %q", got.Issuer, claims.Issuer)
					return false
				}
				if got.Audience != claims.Audience {
					return false
				}
				if got.ID != claims.ID {
					return false
				}
				if got.ExpiresAt != claims.ExpiresAt {
					return false
				}
				if got.IssuedAt != claims.IssuedAt {
					return false
				}
				// Check extra round-trip
				if extraKey != "" {
					v, ok := got.Extra[extraKey]
					if !ok {
						return false
					}
					if v != extraVal {
						return false
					}
				}

				return true
			}

			if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
				t.Errorf("Property failed for %s: %v", alg, err)
			}
		})
	}
}

// TestPropertySignVerifyWrongKeyFails verifies that verifying with a
// different secret always fails.
//
// **Validates: Requirements 9.13**
func TestPropertySignVerifyWrongKeyFails(t *testing.T) {
	f := func(sub string, secretA, secretB string) bool {
		if secretA == secretB || secretA == "" || secretB == "" {
			return true // skip trivial cases
		}

		claims := Claims{
			Subject:   sub,
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		}

		token, err := Sign(claims, []byte(secretA))
		if err != nil {
			return false
		}

		_, err = Verify(token, []byte(secretB))
		return err == ErrInvalidToken
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("Property failed: %v", err)
	}
}

// TestPropertyExpiredTokensRejected verifies that tokens with past
// expiration are always rejected.
//
// **Validates: Requirements 9.5**
func TestPropertyExpiredTokensRejected(t *testing.T) {
	secret := []byte("expired-test-secret")

	f := func(sub string, hoursAgo uint8) bool {
		if hoursAgo == 0 {
			hoursAgo = 1
		}
		// Cap to avoid overflow
		hours := time.Duration(math.Min(float64(hoursAgo), 200)) * time.Hour

		claims := Claims{
			Subject:   sub,
			ExpiresAt: time.Now().Add(-hours).Unix(),
		}

		token, err := Sign(claims, secret)
		if err != nil {
			return false
		}

		_, err = Verify(token, secret)
		return err == ErrTokenExpired
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 200}); err != nil {
		t.Errorf("Property failed: %v", err)
	}
}
