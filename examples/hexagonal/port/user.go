package port

import "github.com/go-kruda/kruda/examples/hexagonal/domain"

// UserRepository is a driven port (outbound) — called by the service.
type UserRepository interface {
	FindAll() []*domain.User
	FindByID(id string) (*domain.User, error)
	Create(u *domain.User) *domain.User
	Update(id string, u *domain.User) (*domain.User, error)
	Delete(id string) error
}

// UserService is a driving port (inbound) — called by HTTP adapter.
type UserService interface {
	List() []*domain.User
	Get(id string) (*domain.User, error)
	Create(u *domain.User) *domain.User
	Update(id string, u *domain.User) (*domain.User, error)
	Delete(id string) error
}
