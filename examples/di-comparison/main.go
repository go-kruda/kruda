// DI Comparison — showcases every Kruda DI feature in one app.
// Run: go run .
// Try: curl http://localhost:3000/orders
//      curl -X POST http://localhost:3000/orders -d '{"item":"book","qty":2}'
package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/go-kruda/kruda"
)

// --- Domain types ---

type DB struct {
	DSN  string
	Name string
}

// DB implements Initializer + Shutdowner — lifecycle managed by container.
func (d *DB) OnInit(ctx context.Context) error {
	fmt.Printf("[lifecycle] DB %s: connected to %s\n", d.Name, d.DSN)
	return nil
}

func (d *DB) OnShutdown(ctx context.Context) error {
	fmt.Printf("[lifecycle] DB %s: connection closed\n", d.Name)
	return nil
}

type Cache interface {
	Get(key string) (string, bool)
	Set(key string, val string)
}

type RedisCache struct {
	mu   sync.RWMutex
	data map[string]string
	addr string
}

func (r *RedisCache) OnInit(ctx context.Context) error {
	fmt.Printf("[lifecycle] Redis: connected to %s\n", r.addr)
	return nil
}

func (r *RedisCache) OnShutdown(ctx context.Context) error {
	fmt.Printf("[lifecycle] Redis: disconnected\n")
	return nil
}

func (r *RedisCache) Get(key string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.data[key]
	return v, ok
}

func (r *RedisCache) Set(key string, val string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[key] = val
}

type RequestLogger struct{ ID string }

func (l *RequestLogger) Log(msg string) string {
	return fmt.Sprintf("[req:%s] %s", l.ID, msg)
}

type Order struct {
	ID   string `json:"id"`
	Item string `json:"item"`
	Qty  int    `json:"qty"`
	Via  string `json:"via"` // which DB served this
	Req  string `json:"req"` // request logger ID
}

// --- Main ---

func main() {
	c := kruda.NewContainer()

	// Named: two databases
	c.GiveNamed("write", &DB{DSN: "postgres://primary:5432", Name: "primary"})
	c.GiveNamed("read", &DB{DSN: "postgres://replica:5432", Name: "replica"})

	// Interface binding: swap Redis → Memcached without touching handlers
	c.GiveAs(&RedisCache{data: make(map[string]string), addr: "localhost:6379"}, (*Cache)(nil))

	// Lazy: only init when first used
	c.GiveLazy(func() (*time.Location, error) {
		return time.LoadLocation("Asia/Bangkok")
	})

	// Transient: new instance per resolve
	c.GiveTransient(func() (*RequestLogger, error) {
		return &RequestLogger{ID: fmt.Sprintf("%04x", rand.Intn(0xFFFF))}, nil
	})

	app := kruda.New(kruda.WithContainer(c))

	// POST /orders — uses write DB, cache, transient logger
	app.Post("/orders", func(c *kruda.Ctx) error {
		writeDB := kruda.MustResolveNamed[*DB](c, "write")
		cache := kruda.MustResolve[Cache](c)
		logger := kruda.MustResolve[*RequestLogger](c)

		var input struct {
			Item string `json:"item"`
			Qty  int    `json:"qty"`
		}
		if err := c.Bind(&input); err != nil {
			return c.Status(400).JSON(kruda.Map{"error": err.Error()})
		}

		id := fmt.Sprintf("ord_%04x", rand.Intn(0xFFFF))
		cache.Set(id, input.Item)

		order := Order{
			ID: id, Item: input.Item, Qty: input.Qty,
			Via: writeDB.Name,
			Req: logger.Log("created"),
		}
		return c.Status(201).JSON(order)
	})

	// GET /orders — uses read DB, cache, different transient logger
	app.Get("/orders", func(c *kruda.Ctx) error {
		readDB := kruda.MustResolveNamed[*DB](c, "read")
		cache := kruda.MustResolve[Cache](c)
		logger := kruda.MustResolve[*RequestLogger](c)
		loc := kruda.MustResolve[*time.Location](c) // lazy — loaded once

		_, _ = cache.Get("test")
		return c.JSON(kruda.Map{
			"db":       readDB.Name,
			"logger":   logger.Log("listed"),
			"timezone": loc.String(),
		})
	})

	// Lifecycle: container calls OnInit on all services
	if err := c.Start(context.Background()); err != nil {
		panic(err)
	}

	app.Listen(":3000")
}
