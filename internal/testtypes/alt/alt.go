// Package alt provides an exported type whose short name ("User") collides with
// internal/testtypes.User but lives in a different package path. Used by OpenAPI
// sanitizer tests to verify cross-package generic instantiations get distinct,
// valid component keys.
package alt

// User is a minimal exported struct sharing the short name "User" with
// internal/testtypes.User, but originating from a different package.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
