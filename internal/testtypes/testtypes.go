// Package testtypes provides exported types used only by tests that need a
// generic type argument originating from a sub-package, so that reflect's
// Type.Name() embeds a package path ("/") and a qualifier (".").
package testtypes

// User is a minimal exported struct for OpenAPI sanitizer tests.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
