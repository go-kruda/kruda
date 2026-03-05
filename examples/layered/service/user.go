package service

import (
	"github.com/go-kruda/kruda/examples/layered/model"
	"github.com/go-kruda/kruda/examples/layered/repository"
)

type UserService struct {
	repo *repository.UserRepo
}

func NewUserService(repo *repository.UserRepo) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) List() []*model.User        { return s.repo.FindAll() }
func (s *UserService) Get(id string) (*model.User, error) { return s.repo.FindByID(id) }
func (s *UserService) Create(u *model.User) *model.User   { return s.repo.Create(u) }
func (s *UserService) Update(id string, u *model.User) (*model.User, error) { return s.repo.Update(id, u) }
func (s *UserService) Delete(id string) error              { return s.repo.Delete(id) }
