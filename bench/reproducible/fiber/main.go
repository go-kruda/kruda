package main

import (
	"context"
	"fmt"
	"html"
	"math/rand/v2"
	"os"
	"sort"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type World struct {
	ID           int32 `json:"id"`
	RandomNumber int32 `json:"randomNumber"`
}

type Fortune struct {
	ID      int32
	Message string
}

type JSONBody struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type JSONResponse struct {
	Message string   `json:"message"`
	Data    JSONBody `json:"data"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

var pool *pgxpool.Pool

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world?pool_max_conns=64&pool_min_conns=8"
	}

	var err error
	pool, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgxpool: %v\n", err)
		os.Exit(1)
	}

	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Get("/json", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Hello, World!"})
	})

	app.Get("/users/:id", func(c *fiber.Ctx) error {
		id, _ := strconv.Atoi(c.Params("id"))
		return c.JSON(UserResponse{ID: id, Name: "Tiger", Email: "tiger@kruda.dev"})
	})

	app.Post("/json", func(c *fiber.Ctx) error {
		var body JSONBody
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(JSONResponse{Message: "received", Data: body})
	})

	app.Get("/db", func(c *fiber.Ctx) error {
		w := World{ID: int32(rand.IntN(10000) + 1)}
		pool.QueryRow(context.Background(),
			"SELECT randomnumber FROM world WHERE id=$1", w.ID,
		).Scan(&w.RandomNumber)
		return c.JSON(w)
	})

	app.Get("/queries", func(c *fiber.Ctx) error {
		n := clamp(queryCount(c.Query("q")), 1, 500)
		worlds := make([]World, n)
		for i := range worlds {
			worlds[i].ID = int32(rand.IntN(10000) + 1)
		}
		batch := &pgx.Batch{}
		for i := range worlds {
			batch.Queue("SELECT randomnumber FROM world WHERE id=$1", worlds[i].ID)
		}
		br := pool.SendBatch(context.Background(), batch)
		for i := range worlds {
			br.QueryRow().Scan(&worlds[i].RandomNumber)
		}
		br.Close()
		return c.JSON(worlds)
	})

	app.Get("/fortunes", func(c *fiber.Ctx) error {
		rows, err := pool.Query(context.Background(), "SELECT id, message FROM fortune")
		if err != nil {
			return c.Status(500).SendString(err.Error())
		}
		defer rows.Close()
		fortunes := make([]Fortune, 0, 13)
		for rows.Next() {
			var f Fortune
			rows.Scan(&f.ID, &f.Message)
			fortunes = append(fortunes, f)
		}
		fortunes = append(fortunes, Fortune{Message: "Additional fortune added at request time."})
		sort.Slice(fortunes, func(i, j int) bool { return fortunes[i].Message < fortunes[j].Message })
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(fortunesHTML(fortunes))
	})

	app.Get("/updates", func(c *fiber.Ctx) error {
		n := clamp(queryCount(c.Query("q")), 1, 500)
		worlds := make([]World, n)
		for i := range worlds {
			worlds[i].ID = int32(rand.IntN(10000) + 1)
		}
		batch := &pgx.Batch{}
		for i := range worlds {
			batch.Queue("SELECT randomnumber FROM world WHERE id=$1", worlds[i].ID)
		}
		br := pool.SendBatch(context.Background(), batch)
		for i := range worlds {
			br.QueryRow().Scan(&worlds[i].RandomNumber)
			worlds[i].RandomNumber = int32(rand.IntN(10000) + 1)
		}
		br.Close()
		ids := make([]int32, n)
		nums := make([]int32, n)
		for i, w := range worlds {
			ids[i] = w.ID
			nums[i] = w.RandomNumber
		}
		pool.Exec(context.Background(),
			"UPDATE world SET randomnumber=v.r FROM (SELECT unnest($1::int[]) id, unnest($2::int[]) r) v WHERE world.id=v.id",
			ids, nums,
		)
		return c.JSON(worlds)
	})

	fmt.Printf("[fiber] listening on :%s\n", port)
	app.Listen(":" + port)
}

func queryCount(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func fortunesHTML(ff []Fortune) string {
	buf := make([]byte, 0, 512+len(ff)*128)
	buf = append(buf, "<!DOCTYPE html><html><head><title>Fortunes</title></head><body><table><tr><th>id</th><th>message</th></tr>"...)
	for _, f := range ff {
		buf = append(buf, "<tr><td>"...)
		buf = strconv.AppendInt(buf, int64(f.ID), 10)
		buf = append(buf, "</td><td>"...)
		buf = append(buf, html.EscapeString(f.Message)...)
		buf = append(buf, "</td></tr>"...)
	}
	buf = append(buf, "</table></body></html>"...)
	return string(buf)
}
