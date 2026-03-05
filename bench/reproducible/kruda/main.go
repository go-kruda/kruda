package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/go-kruda/kruda"
	krudajson "github.com/go-kruda/kruda/json"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TFB types
type World struct {
	ID           int32 `json:"id"`
	RandomNumber int32 `json:"randomNumber"`
}

type Fortune struct {
	ID      int32
	Message string
}

type JSONMessage struct {
	Message string `json:"message"`
}

// bench types
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
		port = "3000"
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world?pool_max_conns=64&pool_min_conns=8"
	}

	fmt.Printf("[kruda] JSON encoder: %s\n", krudajson.EncoderName)

	var err error
	cfg, _ := pgxpool.ParseConfig(dsn)
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		conn.Prepare(ctx, "worldSelect", "SELECT randomnumber FROM world WHERE id=$1")
		conn.Prepare(ctx, "fortuneSelect", "SELECT id, message FROM fortune")
		conn.Prepare(ctx, "worldUpdate", "UPDATE world SET randomnumber=$1 WHERE id=$2")
		return nil
	}
	pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgxpool: %v\n", err)
		os.Exit(1)
	}

	go func() {
		fmt.Println("[pprof] listening on :6060")
		_ = http.ListenAndServe(":6060", nil)
	}()

	app := kruda.New(kruda.Wing())

	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("Hello, World!")
	}, kruda.WingPlaintext())

	// TFB: JSON serialization
	app.Get("/json", func(c *kruda.Ctx) error {
		return c.JSON(JSONMessage{Message: "Hello, World!"})
	}, kruda.WingJSON())

	app.Get("/users/:id", func(c *kruda.Ctx) error {
		id, _ := c.ParamInt("id")
		return c.JSON(UserResponse{ID: id, Name: "Tiger", Email: "tiger@kruda.dev"})
	}, kruda.WingParamJSON())

	app.Post("/json", func(c *kruda.Ctx) error {
		var body JSONBody
		if err := c.Bind(&body); err != nil {
			return c.Status(400).JSON(map[string]string{"error": err.Error()})
		}
		return c.JSON(JSONResponse{Message: "received", Data: body})
	}, kruda.WingPostJSON())

	// TFB: single DB query
	app.Get("/db", func(c *kruda.Ctx) error {
		w := World{ID: int32(rand.IntN(10000) + 1)}
		pool.QueryRow(context.Background(), "worldSelect", w.ID).Scan(&w.RandomNumber)
		return c.JSON(w)
	}, kruda.WingQuery())

	// TFB: multiple queries — pipeline via SendBatch
	app.Get("/queries", func(c *kruda.Ctx) error {
		n := clamp(queryCount(c), 1, 500)
		worlds := make([]World, n)
		for i := range worlds {
			worlds[i].ID = int32(rand.IntN(10000) + 1)
		}
		batch := &pgx.Batch{}
		for i := range worlds {
			batch.Queue("worldSelect", worlds[i].ID)
		}
		br := pool.SendBatch(context.Background(), batch)
		for i := range worlds {
			br.QueryRow().Scan(&worlds[i].RandomNumber)
		}
		br.Close()
		return c.JSON(worlds)
	}, kruda.WingQuery())

	// TFB: fortunes
	app.Get("/fortunes", func(c *kruda.Ctx) error {
		rows, err := pool.Query(context.Background(), "fortuneSelect")
		if err != nil {
			return c.Status(500).Text(err.Error())
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

		return c.HTML(fortunesHTML(fortunes))
	}, kruda.WingRender())

	// TFB: updates — batch SELECT + batch UPDATE
	app.Get("/updates", func(c *kruda.Ctx) error {
		n := clamp(queryCount(c), 1, 500)
		worlds := make([]World, n)
		for i := range worlds {
			worlds[i].ID = int32(rand.IntN(10000) + 1)
		}
		batch := &pgx.Batch{}
		for i := range worlds {
			batch.Queue("worldSelect", worlds[i].ID)
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
	}, kruda.WingQuery())

	app.Listen(":" + port)
}

func queryCount(c *kruda.Ctx) int {
	n, _ := strconv.Atoi(c.Query("q"))
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

var fortuneBufPool = sync.Pool{
	New: func() any { b := make([]byte, 0, 4096); return &b },
}

func fortunesHTML(ff []Fortune) string {
	bp := fortuneBufPool.Get().(*[]byte)
	buf := (*bp)[:0]
	buf = append(buf, "<!DOCTYPE html><html><head><title>Fortunes</title></head><body><table><tr><th>id</th><th>message</th></tr>"...)
	for _, f := range ff {
		buf = append(buf, "<tr><td>"...)
		buf = strconv.AppendInt(buf, int64(f.ID), 10)
		buf = append(buf, "</td><td>"...)
		buf = appendHTMLEscape(buf, f.Message)
		buf = append(buf, "</td></tr>"...)
	}
	buf = append(buf, "</table></body></html>"...)
	s := string(buf)
	*bp = buf
	fortuneBufPool.Put(bp)
	return s
}

func appendHTMLEscape(buf []byte, s string) []byte {
	last := 0
	for i := 0; i < len(s); i++ {
		var esc string
		switch s[i] {
		case '&':
			esc = "&amp;"
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		case '"':
			esc = "&#34;"
		case '\'':
			esc = "&#39;"
		default:
			continue
		}
		buf = append(buf, s[last:i]...)
		buf = append(buf, esc...)
		last = i + 1
	}
	return append(buf, s[last:]...)
}
