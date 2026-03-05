// Layered pattern — handler → service → repository.
// Best for: typical CRUD apps, small-to-medium APIs.
package main

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/examples/layered/handler"
	"github.com/go-kruda/kruda/examples/layered/repository"
	"github.com/go-kruda/kruda/examples/layered/service"
)

func main() {
	repo := repository.NewUserRepo()
	svc := service.NewUserService(repo)
	h := handler.NewUserHandler(svc)

	app := kruda.New()

	app.Get("/users", h.List)
	app.Get("/users/:id", h.Get)
	app.Post("/users", h.Create)
	app.Put("/users/:id", h.Update)
	app.Delete("/users/:id", h.Delete)

	app.Listen(":3000")
}
