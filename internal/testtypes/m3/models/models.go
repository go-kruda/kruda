// Package models is one of three same-named packages (m1/m2/m3) used to
// exercise OpenAPI component-key disambiguation when distinct types share both
// a short name ("User") and a trailing package segment ("models").
package models

// User is the m3 variant; its unique field "c" lets tests confirm this exact
// schema survives disambiguation rather than being overwritten.
type User struct {
	ID string `json:"id"`
	C  string `json:"c"`
}
