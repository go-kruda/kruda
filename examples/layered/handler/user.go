package handler

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/examples/layered/model"
	"github.com/go-kruda/kruda/examples/layered/repository"
	"github.com/go-kruda/kruda/examples/layered/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) List(c *kruda.Ctx) error {
	return c.JSON(h.svc.List())
}

func (h *UserHandler) Get(c *kruda.Ctx) error {
	u, err := h.svc.Get(c.Param("id"))
	if err == repository.ErrNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.JSON(u)
}

func (h *UserHandler) Create(c *kruda.Ctx) error {
	var u model.User
	if err := c.Bind(&u); err != nil {
		return c.Status(400).JSON(kruda.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(h.svc.Create(&u))
}

func (h *UserHandler) Update(c *kruda.Ctx) error {
	var body model.User
	if err := c.Bind(&body); err != nil {
		return c.Status(400).JSON(kruda.Map{"error": err.Error()})
	}
	u, err := h.svc.Update(c.Param("id"), &body)
	if err == repository.ErrNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.JSON(u)
}

func (h *UserHandler) Delete(c *kruda.Ctx) error {
	if err := h.svc.Delete(c.Param("id")); err == repository.ErrNotFound {
		return c.Status(404).JSON(kruda.Map{"error": "not found"})
	}
	return c.NoContent()
}
