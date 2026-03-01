package main

import (
	"context"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/jackc/pgx/v5"
)

// ParseQueries parses the "queries" or "count" parameter, clamping to [1, 500].
// Returns 1 on error or values < 1, returns 500 on values > 500.
func ParseQueries(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	if n > 500 {
		return 500
	}
	return n
}

// randomWorldID returns a random int32 in [1, 10000].
func randomWorldID() int32 {
	return int32(rand.IntN(10000) + 1)
}

// generateUniqueIDs returns n unique random IDs in [1, 10000], sorted ascending.
// Uses a pooled bitmap instead of map to avoid allocation per request.
// With 10,000 possible IDs, a [10001]bool (10KB) is cheaper than a map.
var idBitmapPool = sync.Pool{New: func() any { return new([10001]bool) }}

func generateUniqueIDs(n int) []int32 {
	bp := idBitmapPool.Get().(*[10001]bool)
	ids := GetInt32Slice()
	*ids = (*ids)[:0]
	for len(*ids) < n {
		id := randomWorldID()
		if !bp[id] {
			bp[id] = true
			*ids = append(*ids, id)
		}
	}
	// Clear only used entries
	for _, id := range *ids {
		bp[id] = false
	}
	idBitmapPool.Put(bp)
	slices.Sort(*ids)
	// Return the underlying slice; caller owns it until next generateUniqueIDs call.
	// This avoids copying — ids slice is reused directly as UNNEST $1 parameter.
	result := *ids
	// Don't put back to pool here — caller (updatesHandler) doesn't pool this.
	// For queriesHandler path (no updates), ids is short-lived and GC'd.
	return result
}

// newRandomNumber returns a new random int32 in [1, 10000] that differs from current.
func newRandomNumber(current int32) int32 {
	for {
		n := randomWorldID()
		if n != current {
			return n
		}
	}
}

// setDateHeader sets the Date response header from the cached value.
// Uses direct fasthttp header access for zero-overhead on the hot path.
func setDateHeader(c *kruda.Ctx) {
	if h := c.RawResponseHeader(); h != nil {
		h.SetBytesV("Date", GetDateHeader())
		return
	}
	c.SetHeaderBytes("Date", GetDateHeader())
}

// ---------------------------------------------------------------------------
// TFB Handlers — all use kruda.Ctx API exclusively
// ---------------------------------------------------------------------------

// jsonHandler handles GET /json — 0 allocations per request.
func jsonHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	return c.SendStaticWithTypeBytes(ctJSON, jsonResponse)
}

// plaintextHandler handles GET /plaintext — 0 allocations per request.
func plaintextHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	return c.SendStaticWithTypeBytes(ctText, textResponse)
}

// dbHandler handles GET /db — single DB query, ≤1 allocation per request.
func dbHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	var w World
	id := randomWorldID()
	if err := db.QueryRow(context.Background(), "selectWorld", id).Scan(&w.ID, &w.RandomNumber); err != nil {
		c.Status(500)
		return err
	}

	buf := GetBuffer(64)
	*buf = SerializeWorldJSON(*buf, w)
	err := c.SendBytesWithTypeBytes(ctJSON, *buf)
	PutBuffer(buf)
	return err
}

// queriesHandler handles GET /queries?queries=N — batch N SELECTs in single roundtrip.
func queriesHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	n := ParseQueries(c.Query("queries"))

	batch := &pgx.Batch{}
	for i := 0; i < n; i++ {
		batch.Queue("selectWorld", randomWorldID())
	}

	br := db.SendBatch(context.Background(), batch)

	worlds := GetWorldSlice()
	*worlds = (*worlds)[:n]
	for i := 0; i < n; i++ {
		if err := br.QueryRow().Scan(&(*worlds)[i].ID, &(*worlds)[i].RandomNumber); err != nil {
			br.Close()
			PutWorldSlice(worlds)
			c.Status(500)
			return err
		}
	}
	br.Close()

	// Estimate ~30 bytes per World JSON object
	buf := GetBuffer(n * 30)
	*buf = SerializeWorldsJSON(*buf, *worlds)
	PutWorldSlice(worlds)

	err := c.SendBytesWithTypeBytes(ctJSON, *buf)
	PutBuffer(buf)
	return err
}

// updatesHandler handles GET /updates?queries=N — single-query read + bulk UNNEST update.
// Both read and write use prepared statements: 2 round-trips total (vs Fiber's 1 batch + 1 raw).
func updatesHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	n := ParseQueries(c.Query("queries"))
	ctx := context.Background()

	// Generate N unique sorted random IDs (deadlock prevention).
	ids := generateUniqueIDs(n)

	// Single-query read via ANY($1) — 1 Bind+Execute instead of N via pgx.Batch.
	rows, err := db.Query(ctx, "selectWorldBatch", ids)
	if err != nil {
		c.Status(500)
		return err
	}

	worlds := GetWorldSlice()
	*worlds = (*worlds)[:n]
	i := 0
	for rows.Next() {
		if err := rows.Scan(&(*worlds)[i].ID, &(*worlds)[i].RandomNumber); err != nil {
			rows.Close()
			PutWorldSlice(worlds)
			c.Status(500)
			return err
		}
		i++
	}
	rows.Close()

	// Generate new randomNumber + build UNNEST nums array in single pass
	nums := GetInt32Slice()
	*nums = (*nums)[:n]
	for i := range *worlds {
		(*worlds)[i].RandomNumber = newRandomNumber((*worlds)[i].RandomNumber)
		(*nums)[i] = (*worlds)[i].RandomNumber
	}

	// Prepared "updateWorlds" — skips Parse+Describe on every request
	if _, err := db.Exec(ctx, "updateWorlds", ids, *nums); err != nil {
		PutInt32Slice(nums)
		PutWorldSlice(worlds)
		c.Status(500)
		return err
	}
	PutInt32Slice(nums)

	buf := GetBuffer(n * 30)
	*buf = SerializeWorldsJSON(*buf, *worlds)
	PutWorldSlice(worlds)

	sendErr := c.SendBytesWithTypeBytes(ctJSON, *buf)
	PutBuffer(buf)
	return sendErr
}

// fortunesHandler handles GET /fortunes — query + sort + HTML serialize.
func fortunesHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	rows, err := db.Query(context.Background(), "selectFortune")
	if err != nil {
		c.Status(500)
		return err
	}
	defer rows.Close()

	fp := GetFortuneSlice()
	for rows.Next() {
		var f Fortune
		if err := rows.Scan(&f.ID, &f.Message); err != nil {
			PutFortuneSlice(fp)
			c.Status(500)
			return err
		}
		*fp = append(*fp, f)
	}

	*fp = append(*fp, Fortune{ID: 0, Message: "Additional fortune added at request time."})

	slices.SortFunc(*fp, func(a, b Fortune) int {
		return strings.Compare(a.Message, b.Message)
	})

	buf := GetBuffer(2048)
	*buf = SerializeFortunesHTML(*buf, *fp)
	PutFortuneSlice(fp)

	sendErr := c.SendBytesWithTypeBytes(ctHTML, *buf)
	PutBuffer(buf)
	return sendErr
}

// cachedQueriesHandler handles GET /cached-queries?count=N — 0 allocations per request.
func cachedQueriesHandler(c *kruda.Ctx) error {
	setDateHeader(c)
	n := ParseQueries(c.Query("count"))

	worlds := GetWorldSlice()
	*worlds = (*worlds)[:n]
	for i := 0; i < n; i++ {
		(*worlds)[i] = cache.Get(int(randomWorldID()))
	}

	buf := GetBuffer(n * 30)
	*buf = SerializeWorldsJSON(*buf, *worlds)
	PutWorldSlice(worlds)

	err := c.SendBytesWithTypeBytes(ctJSON, *buf)
	PutBuffer(buf)
	return err
}
