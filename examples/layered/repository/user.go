package repository

import (
	"errors"
	"fmt"
	"sync"

	"github.com/go-kruda/kruda/examples/layered/model"
)

var ErrNotFound = errors.New("user not found")

type UserRepo struct {
	mu    sync.RWMutex
	users map[string]*model.User
	seq   int
}

func NewUserRepo() *UserRepo {
	return &UserRepo{users: make(map[string]*model.User)}
}

func (r *UserRepo) FindAll() []*model.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*model.User, 0, len(r.users))
	for _, u := range r.users {
		list = append(list, u)
	}
	return list
}

func (r *UserRepo) FindByID(id string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (r *UserRepo) Create(u *model.User) *model.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	u.ID = fmt.Sprintf("%d", r.seq)
	r.users[u.ID] = u
	return u
}

func (r *UserRepo) Update(id string, u *model.User) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	existing.Name = u.Name
	existing.Email = u.Email
	return existing, nil
}

func (r *UserRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[id]; !ok {
		return ErrNotFound
	}
	delete(r.users, id)
	return nil
}
