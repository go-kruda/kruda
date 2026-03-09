package domain

import "errors"

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserRepository is the port — domain defines what it needs.
type UserRepository interface {
	FindAll() []*User
	FindByID(id string) (*User, error)
	Create(u *User) *User
	Update(id string, u *User) (*User, error)
	Delete(id string) error
}

// UserService contains business logic. Depends only on interfaces.
type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) List() []*User                            { return s.repo.FindAll() }
func (s *UserService) Get(id string) (*User, error)             { return s.repo.FindByID(id) }
func (s *UserService) Create(u *User) *User                     { return s.repo.Create(u) }
func (s *UserService) Update(id string, u *User) (*User, error) { return s.repo.Update(id, u) }
func (s *UserService) Delete(id string) error                   { return s.repo.Delete(id) }
