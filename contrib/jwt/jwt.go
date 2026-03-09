package jwt

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"
)

// Errors returned by JWT operations.
var (
	ErrInvalidToken     = errors.New("invalid_token")
	ErrTokenExpired     = errors.New("token_expired")
	ErrTokenNotYetValid = errors.New("token_not_yet_valid")
	ErrMissingToken     = errors.New("missing_token")
	ErrAlgNone          = errors.New("jwt: algorithm \"none\" is not allowed")
	ErrUnsupportedAlg   = errors.New("jwt: unsupported algorithm")
	ErrInvalidKey       = errors.New("jwt: invalid signing key")
)

// header is the JWT header.
type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Sign creates a signed JWT string from claims using the given secret.
// The optional alg parameter specifies the algorithm (default: HS256).
// For RSA algorithms, pass the *rsa.PrivateKey as the secret parameter.
func Sign(claims Claims, secret any, alg ...string) (string, error) {
	algorithm := "HS256"
	if len(alg) > 0 && alg[0] != "" {
		algorithm = alg[0]
	}

	if strings.EqualFold(algorithm, "none") {
		return "", ErrAlgNone
	}

	h := header{Alg: algorithm, Typ: "JWT"}
	headerJSON, err := json.Marshal(h)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal header: %w", err)
	}

	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := headerB64 + "." + payloadB64

	sig, err := sign(algorithm, signingInput, secret)
	if err != nil {
		return "", err
	}

	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return signingInput + "." + sigB64, nil
}

// Verify parses and validates a JWT string.
// For HMAC algorithms, pass []byte as the secret.
// For RSA algorithms, pass *rsa.PublicKey as the secret.
func Verify(tokenStr string, secret any) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var h header
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, ErrInvalidToken
	}

	// R9.12: reject alg:none
	if strings.EqualFold(h.Alg, "none") {
		return nil, ErrAlgNone
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := verify(h.Alg, signingInput, signature, secret); err != nil {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	now := time.Now().Unix()

	// Check expiration
	if claims.ExpiresAt > 0 && now > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	// Check not-before
	if claims.NotBefore > 0 && now < claims.NotBefore {
		return nil, ErrTokenNotYetValid
	}

	return &claims, nil
}

// VerifyWithGrace parses and validates a JWT, allowing expired tokens within gracePeriod.
// Returns the claims and whether the token was expired (but within grace).
func VerifyWithGrace(tokenStr string, secret any, gracePeriod time.Duration) (*Claims, bool, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, false, ErrInvalidToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, false, ErrInvalidToken
	}

	var h header
	if err := json.Unmarshal(headerJSON, &h); err != nil {
		return nil, false, ErrInvalidToken
	}

	if strings.EqualFold(h.Alg, "none") {
		return nil, false, ErrAlgNone
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, false, ErrInvalidToken
	}

	if err := verify(h.Alg, signingInput, signature, secret); err != nil {
		return nil, false, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, false, ErrInvalidToken
	}

	now := time.Now().Unix()
	if claims.ExpiresAt > 0 && now > claims.ExpiresAt {
		// Expired — check grace period
		if gracePeriod > 0 && now <= claims.ExpiresAt+int64(gracePeriod.Seconds()) {
			return &claims, true, nil
		}
		return nil, false, ErrTokenExpired
	}

	// Check not-before
	if claims.NotBefore > 0 && now < claims.NotBefore {
		return nil, false, ErrTokenNotYetValid
	}

	return &claims, false, nil
}

// Refresh issues a new token with updated expiration from an existing token.
// The original token's signature is validated. Expired tokens are accepted
// within the gracePeriod. For RSA, pass *rsa.PrivateKey as signingKey.
func Refresh(tokenStr string, verifyKey any, newExpiry time.Duration, gracePeriod time.Duration, signingKey ...any) (string, error) {
	claims, _, err := VerifyWithGrace(tokenStr, verifyKey, gracePeriod)
	if err != nil {
		return "", err
	}

	claims.ExpiresAt = time.Now().Add(newExpiry).Unix()
	claims.IssuedAt = time.Now().Unix()

	// Determine algorithm from original token
	parts := strings.SplitN(tokenStr, ".", 3)
	headerJSON, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var h header
	json.Unmarshal(headerJSON, &h)

	// Use signingKey if provided (for RSA where verify key != sign key)
	key := verifyKey
	if len(signingKey) > 0 {
		key = signingKey[0]
	}

	return Sign(*claims, key, h.Alg)
}

// sign computes the signature for the given algorithm.
func sign(alg, input string, key any) ([]byte, error) {
	switch alg {
	case "HS256":
		return signHMAC(sha256.New, input, key)
	case "HS384":
		return signHMAC(sha512.New384, input, key)
	case "HS512":
		return signHMAC(sha512.New, input, key)
	case "RS256":
		return signRSA(input, key)
	default:
		return nil, ErrUnsupportedAlg
	}
}

// verify checks the signature for the given algorithm.
func verify(alg, input string, signature []byte, key any) error {
	switch alg {
	case "HS256":
		return verifyHMAC(sha256.New, input, signature, key)
	case "HS384":
		return verifyHMAC(sha512.New384, input, signature, key)
	case "HS512":
		return verifyHMAC(sha512.New, input, signature, key)
	case "RS256":
		return verifyRSA(input, signature, key)
	default:
		return ErrUnsupportedAlg
	}
}

func signHMAC(hashFunc func() hash.Hash, input string, key any) ([]byte, error) {
	secret, ok := key.([]byte)
	if !ok {
		return nil, ErrInvalidKey
	}
	mac := hmac.New(hashFunc, secret)
	mac.Write([]byte(input))
	return mac.Sum(nil), nil
}

// verifyHMAC uses hmac.Equal for constant-time comparison (R9.13).
func verifyHMAC(hashFunc func() hash.Hash, input string, signature []byte, key any) error {
	expected, err := signHMAC(hashFunc, input, key)
	if err != nil {
		return err
	}
	if !hmac.Equal(signature, expected) {
		return ErrInvalidToken
	}
	return nil
}

func signRSA(input string, key any) ([]byte, error) {
	privKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidKey
	}
	h := sha256.Sum256([]byte(input))
	return rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, h[:])
}

func verifyRSA(input string, signature []byte, key any) error {
	pubKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return ErrInvalidKey
	}
	h := sha256.Sum256([]byte(input))
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, h[:], signature)
}
