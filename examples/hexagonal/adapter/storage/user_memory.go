package storage

import (
	"fmt"
	"sync"

	"github.com/go-kruda/kruda/examples/hexagonal/domain"
	"github.com/go-kruda/kruda/examples/hexagonal/port"
)

type memoryUserRepo struct {
	mu    sync.RWMutex
	users map[string]*domain.User
	seq   int
}

func NewMemoryUserRepo() port.UserRepository {
	return &memoryUserRepo{users: make(map[string]*domain.User)}
}

func (r *memoryUserRepo) FindAll() []*domain.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		list = append(list, u)
	}
	return list
}

func (r *memoryUserRepo) FindByID(id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *memoryUserRepo) Create(u *domain.User) *domain.User {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	u.ID = fmt.Sprintf("%d", r.seq)
	r.users[u.ID] = u
	return u
}

func (r *memoryUserRepo) Update(id string, u *domain.User) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	existing.Name = u.Name
	existing.Email = u.Email
	return existing, nil
}

func (r *memoryUserRepo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}
