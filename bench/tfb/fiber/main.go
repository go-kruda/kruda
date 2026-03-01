// Fiber TFB benchmark app — same optimization level as Kruda for fair comparison.
// Uses: pgx.Batch, manual JSON serialization, sync.Pool buffers, flat-array cache.
package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

type World struct {
	ID           int32 `json:"id"`
	RandomNumber int32 `json:"randomNumber"`
}

type Fortune struct {
	ID      int32
	Message string
}

var (
	jsonResponse = []byte(`{"message":"Hello, World!"}`)
	textResponse = []byte("Hello, World!")
)

// ---------------------------------------------------------------------------
// DB + Cache
// ---------------------------------------------------------------------------

var (
	db    *pgxpool.Pool
	cache WorldCache
)

type WorldCache struct {
	data [10001]World
}

func (c *WorldCache) Get(id int) World { return c.data[id] }

func (c *WorldCache) Warmup(pool *pgxpool.Pool) {
	const chunkSize = 500
	for start := 1; start <= 10000; start += chunkSize {
		batch := &pgx.Batch{}
		end := start + chunkSize
		if end > 10001 {
			end = 10001
		}
		for id := start; id < end; id++ {
			batch.Queue("selectWorld", id)
		}
		br := pool.SendBatch(context.Background(), batch)
		for id := start; id < end; id++ {
			var w World
			if err := br.QueryRow().Scan(&w.ID, &w.RandomNumber); err != nil {
				log.Fatalf("cache warmup: %v", err)
			}
			c.data[id] = w
		}
		br.Close()
	}
}

// ---------------------------------------------------------------------------
// Pools
// ---------------------------------------------------------------------------

var (
	smallPool      = sync.Pool{New: func() any { b := make([]byte, 0, 1024); return &b }}
	mediumPool     = sync.Pool{New: func() any { b := make([]byte, 0, 8192); return &b }}
	largePool      = sync.Pool{New: func() any { b := make([]byte, 0, 32768); return &b }}
	worldSlicePool = sync.Pool{New: func() any { s := make([]World, 0, 500); return &s }}
)

func getBuffer(size int) *[]byte {
	switch {
	case size <= 1024:
		bp := smallPool.Get().(*[]byte)
		*bp = (*bp)[:0]
		return bp
	case size <= 8192:
		bp := mediumPool.Get().(*[]byte)
		*bp = (*bp)[:0]
		return bp
	default:
		bp := largePool.Get().(*[]byte)
		*bp = (*bp)[:0]
		return bp
	}
}

func putBuffer(bp *[]byte) {
	if cap(*bp) > 65536 {
		return
	}
	switch {
	case cap(*bp) <= 1024:
		smallPool.Put(bp)
	case cap(*bp) <= 8192:
		mediumPool.Put(bp)
	default:
		largePool.Put(bp)
	}
}

func getWorldSlice() *[]World {
	sp := worldSlicePool.Get().(*[]World)
	*sp = (*sp)[:0]
	return sp
}
func putWorldSlice(sp *[]World) { worldSlicePool.Put(sp) }

// ---------------------------------------------------------------------------
// Date header cache
// ---------------------------------------------------------------------------

var dateHeader atomic.Value

func init() {
	updateDate()
	go func() {
		t := time.NewTicker(time.Second)
		for range t.C {
			updateDate()
		}
	}()
}
func updateDate()     { dateHeader.Store([]byte(time.Now().UTC().Format(http.TimeFormat))) }
func getDate() []byte { return dateHeader.Load().([]byte) }

// ---------------------------------------------------------------------------
// Serializers (identical to Kruda app — fair comparison)
// ---------------------------------------------------------------------------

func serializeWorldJSON(buf []byte, w World) []byte {
	buf = append(buf, `{"id":`...)
	buf = strconv.AppendInt(buf, int64(w.ID), 10)
	buf = append(buf, `,"randomNumber":`...)
	buf = strconv.AppendInt(buf, int64(w.RandomNumber), 10)
	buf = append(buf, '}')
	return buf
}

func serializeWorldsJSON(buf []byte, worlds []World) []byte {
	buf = append(buf, '[')
	for i := range worlds {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = serializeWorldJSON(buf, worlds[i])
	}
	buf = append(buf, ']')
	return buf
}

func htmlEscape(buf []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			buf = append(buf, "&lt;"...)
		case '>':
			buf = append(buf, "&gt;"...)
		case '&':
			buf = append(buf, "&amp;"...)
		case '"':
			buf = append(buf, "&quot;"...)
		case '\'':
			buf = append(buf, "&#39;"...)
		default:
			buf = append(buf, s[i])
		}
	}
	return buf
}

func serializeFortunesHTML(buf []byte, fortunes []Fortune) []byte {
	buf = append(buf, "<!DOCTYPE html><html><head><title>Fortunes</title></head><body><table><tr><th>id</th><th>message</th></tr>"...)
	for i := range fortunes {
		buf = append(buf, "<tr><td>"...)
		buf = strconv.AppendInt(buf, int64(fortunes[i].ID), 10)
		buf = append(buf, "</td><td>"...)
		buf = htmlEscape(buf, fortunes[i].Message)
		buf = append(buf, "</td></tr>"...)
	}
	buf = append(buf, "</table></body></html>"...)
	return buf
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseQueries(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	if n > 500 {
		return 500
	}
	return n
}

func randomWorldID() int32 { return int32(rand.IntN(10000) + 1) }

func generateUniqueIDs(n int) []int32 {
	seen := make(map[int32]struct{}, n)
	ids := make([]int32, 0, n)
	for len(ids) < n {
		id := randomWorldID()
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

func newRandomNumber(current int32) int32 {
	for {
		n := randomWorldID()
		if n != current {
			return n
		}
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func jsonHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	c.Set("Content-Type", "application/json")
	return c.Send(jsonResponse)
}

func plaintextHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	c.Set("Content-Type", "text/plain")
	return c.Send(textResponse)
}

func dbHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	var w World
	id := randomWorldID()
	if err := db.QueryRow(context.Background(), "selectWorld", id).Scan(&w.ID, &w.RandomNumber); err != nil {
		return c.SendStatus(500)
	}
	buf := getBuffer(64)
	*buf = serializeWorldJSON(*buf, w)
	c.Set("Content-Type", "application/json")
	err := c.Send(*buf)
	putBuffer(buf)
	return err
}

func queriesHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	n := parseQueries(c.Query("queries"))

	batch := &pgx.Batch{}
	for i := 0; i < n; i++ {
		batch.Queue("selectWorld", randomWorldID())
	}
	br := db.SendBatch(context.Background(), batch)

	worlds := getWorldSlice()
	*worlds = (*worlds)[:n]
	for i := 0; i < n; i++ {
		if err := br.QueryRow().Scan(&(*worlds)[i].ID, &(*worlds)[i].RandomNumber); err != nil {
			br.Close()
			putWorldSlice(worlds)
			return c.SendStatus(500)
		}
	}
	br.Close()

	buf := getBuffer(n * 30)
	*buf = serializeWorldsJSON(*buf, *worlds)
	putWorldSlice(worlds)

	c.Set("Content-Type", "application/json")
	err := c.Send(*buf)
	putBuffer(buf)
	return err
}

func updatesHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	n := parseQueries(c.Query("queries"))
	ctx := context.Background()

	ids := generateUniqueIDs(n)

	readBatch := &pgx.Batch{}
	for _, id := range ids {
		readBatch.Queue("selectWorld", id)
	}
	br := db.SendBatch(ctx, readBatch)

	worlds := getWorldSlice()
	*worlds = (*worlds)[:n]
	for i := 0; i < n; i++ {
		if err := br.QueryRow().Scan(&(*worlds)[i].ID, &(*worlds)[i].RandomNumber); err != nil {
			br.Close()
			putWorldSlice(worlds)
			return c.SendStatus(500)
		}
	}
	br.Close()

	for i := range *worlds {
		(*worlds)[i].RandomNumber = newRandomNumber((*worlds)[i].RandomNumber)
	}

	// Bulk update via UNNEST — single query instead of N individual UPDATEs
	ids2 := make([]int32, n)
	nums := make([]int32, n)
	for i, w := range *worlds {
		ids2[i] = w.ID
		nums[i] = w.RandomNumber
	}
	if _, err := db.Exec(ctx,
		"UPDATE world SET randomnumber = v.r FROM UNNEST($1::int[], $2::int[]) AS v(id, r) WHERE world.id = v.id",
		ids2, nums,
	); err != nil {
		putWorldSlice(worlds)
		return c.SendStatus(500)
	}

	buf := getBuffer(n * 30)
	*buf = serializeWorldsJSON(*buf, *worlds)
	putWorldSlice(worlds)

	c.Set("Content-Type", "application/json")
	sendErr := c.Send(*buf)
	putBuffer(buf)
	return sendErr
}

func fortunesHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	rows, err := db.Query(context.Background(), "selectFortune")
	if err != nil {
		return c.SendStatus(500)
	}
	defer rows.Close()

	fortunes := make([]Fortune, 0, 16)
	for rows.Next() {
		var f Fortune
		if err := rows.Scan(&f.ID, &f.Message); err != nil {
			return c.SendStatus(500)
		}
		fortunes = append(fortunes, f)
	}
	fortunes = append(fortunes, Fortune{ID: 0, Message: "Additional fortune added at request time."})

	slices.SortFunc(fortunes, func(a, b Fortune) int {
		return strings.Compare(a.Message, b.Message)
	})

	buf := getBuffer(2048)
	*buf = serializeFortunesHTML(*buf, fortunes)

	c.Set("Content-Type", "text/html; charset=utf-8")
	sendErr := c.Send(*buf)
	putBuffer(buf)
	return sendErr
}

func cachedQueriesHandler(c *fiber.Ctx) error {
	c.Response().Header.SetBytesV("Date", getDate())
	n := parseQueries(c.Query("count"))

	worlds := getWorldSlice()
	*worlds = (*worlds)[:n]
	for i := 0; i < n; i++ {
		(*worlds)[i] = cache.Get(int(randomWorldID()))
	}

	buf := getBuffer(n * 30)
	*buf = serializeWorldsJSON(*buf, *worlds)
	putWorldSlice(worlds)

	c.Set("Content-Type", "application/json")
	err := c.Send(*buf)
	putBuffer(buf)
	return err
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world?sslmode=disable"
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal(err)
	}

	gomaxprocs := runtime.GOMAXPROCS(0)
	workers := gomaxprocs
	if w, err := strconv.Atoi(os.Getenv("FIBER_WORKERS")); err == nil && w > 0 {
		workers = w
	}
	perWorker := max(256/workers, 4)
	poolConfig.MaxConns = int32(min(perWorker, 256))
	poolConfig.MinConns = int32(max(poolConfig.MaxConns/4, 1))
	poolConfig.MaxConnLifetime = time.Hour

	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Prepare(ctx, "selectWorld", "SELECT id, randomnumber FROM world WHERE id = $1"); err != nil {
			return err
		}
		if _, err := conn.Prepare(ctx, "selectFortune", "SELECT id, message FROM fortune"); err != nil {
			return err
		}
		return nil
	}

	db, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	cache.Warmup(db)

	prefork := os.Getenv("FIBER_PREFORK") == "1"

	app := fiber.New(fiber.Config{
		ServerHeader:          "Fiber",
		Prefork:               prefork,
		DisableStartupMessage: true,
		Immutable:             false,
	})

	app.Get("/json", jsonHandler)
	app.Get("/plaintext", plaintextHandler)
	app.Get("/db", dbHandler)
	app.Get("/queries", queriesHandler)
	app.Get("/updates", updatesHandler)
	app.Get("/fortunes", fortunesHandler)
	app.Get("/cached-queries", cachedQueriesHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Fiber TFB listening on :%s (GOMAXPROCS=%d, prefork=%v)", port, gomaxprocs, prefork)
	log.Fatal(app.Listen(":" + port))
}
