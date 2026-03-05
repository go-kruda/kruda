package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
