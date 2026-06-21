// Package models is one of three same-named packages (m1/m2/m3) used to
// exercise OpenAPI component-key disambiguation when distinct types share both
// a short name ("User") and a trailing package segment ("models").
package models

// User is the m1 variant; its unique field "a" lets tests confirm this exact
// schema survives disambiguation rather than being overwritten.
type User struct {
	ID string `json:"id"`
	A  string `json:"a"`
}
