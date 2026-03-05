// Hexagonal Architecture (Ports & Adapters).
// Domain and ports are pure. Adapters plug in from outside.
package main

import (
	"github.com/go-kruda/kruda"
	httpAdapter "github.com/go-kruda/kruda/examples/hexagonal/adapter/http"
	"github.com/go-kruda/kruda/examples/hexagonal/adapter/storage"
	"github.com/go-kruda/kruda/examples/hexagonal/service"
)

func main() {
	repo := storage.NewMemoryUserRepo()
	svc := service.NewUserService(repo)
	h := httpAdapter.NewUserHandler(svc)

	app := kruda.New(kruda.NetHTTP())
	h.Register(app)

	app.Listen(":3000")
}
