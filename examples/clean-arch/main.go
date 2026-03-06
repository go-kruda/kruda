// Clean Architecture — domain core has zero external imports.
// Dependencies point inward: handler → domain ← repository.
package main

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/examples/clean-arch/domain"
	"github.com/go-kruda/kruda/examples/clean-arch/handler"
	"github.com/go-kruda/kruda/examples/clean-arch/repository"
)

func main() {
	repo := repository.NewMemoryUserRepo()
	svc := domain.NewUserService(repo)
	h := handler.NewUserHandler(svc)

	app := kruda.New()

	app.Get("/users", h.List)
	app.Get("/users/:id", h.Get)
	app.Post("/users", h.Create)
	app.Put("/users/:id", h.Update)
	app.Delete("/users/:id", h.Delete)

	app.Listen(":3000")
}
