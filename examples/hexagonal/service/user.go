package service

import (
	"github.com/go-kruda/kruda/examples/hexagonal/domain"
	"github.com/go-kruda/kruda/examples/hexagonal/port"
)

type userService struct {
	repo port.UserRepository
}

func NewUserService(repo port.UserRepository) port.UserService {
	return &userService{repo: repo}
}

func (s *userService) List() []*domain.User                            { return s.repo.FindAll() }
func (s *userService) Get(id string) (*domain.User, error)             { return s.repo.FindByID(id) }
func (s *userService) Create(u *domain.User) *domain.User              { return s.repo.Create(u) }
func (s *userService) Update(id string, u *domain.User) (*domain.User, error) { return s.repo.Update(id, u) }
func (s *userService) Delete(id string) error                          { return s.repo.Delete(id) }
