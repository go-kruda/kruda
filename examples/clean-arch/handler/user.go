package handler

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/examples/clean-arch/domain"
)

type UserHandler struct {
	svc *domain.UserService
}

func NewUserHandler(svc *domain.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) List(c *kruda.Ctx) error {
	return c.JSON(h.svc.List())
}

func (h *UserHandler) Get(c *kruda.Ctx) error {
	u, err := h.svc.Get(c.Param("id"))
	if err == domain.ErrUserNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.JSON(u)
}

func (h *UserHandler) Create(c *kruda.Ctx) error {
	var u domain.User
	if err := c.Bind(&u); err != nil {
		return c.Status(400).JSON(kruda.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(h.svc.Create(&u))
}

func (h *UserHandler) Update(c *kruda.Ctx) error {
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

func (h *UserHandler) Delete(c *kruda.Ctx) error {
	if err := h.svc.Delete(c.Param("id")); err == domain.ErrUserNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.NoContent()
}
