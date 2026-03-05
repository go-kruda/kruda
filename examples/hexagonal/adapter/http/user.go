package http

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/examples/hexagonal/domain"
	"github.com/go-kruda/kruda/examples/hexagonal/port"
)

type UserHandler struct {
	svc port.UserService
}

func NewUserHandler(svc port.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Register(app *kruda.App) {
	app.Get("/users", h.list)
	app.Get("/users/:id", h.get)
	app.Post("/users", h.create)
	app.Put("/users/:id", h.update)
	app.Delete("/users/:id", h.delete)
}

func (h *UserHandler) list(c *kruda.Ctx) error {
	return c.JSON(h.svc.List())
}

func (h *UserHandler) get(c *kruda.Ctx) error {
	u, err := h.svc.Get(c.Param("id"))
	if err == domain.ErrUserNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.JSON(u)
}

func (h *UserHandler) create(c *kruda.Ctx) error {
	var u domain.User
	if err := c.Bind(&u); err != nil {
		return c.Status(400).JSON(kruda.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(h.svc.Create(&u))
}

func (h *UserHandler) update(c *kruda.Ctx) error {
	var body domain.User
	if err := c.Bind(&body); err != nil {
		return c.Status(400).JSON(kruda.Map{"error": err.Error()})
	}
	u, err := h.svc.Update(c.Param("id"), &body)
	if err == domain.ErrUserNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.JSON(u)
}

func (h *UserHandler) delete(c *kruda.Ctx) error {
	if err := h.svc.Delete(c.Param("id")); err == domain.ErrUserNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.NoContent()
}
