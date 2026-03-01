package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WorldCache is a flat-array cache for all 10,000 World rows.
// Index 0 is unused; IDs 1–10000 map directly to array indices.
// Read-only after Warmup — no locks needed, zero-alloc lookups.
type WorldCache struct {
	data [10001]World // index 0 unused, IDs 1-10000
}

// Get returns the cached World for the given ID.
// Zero-alloc, no lock, O(1) index lookup.
// Returns zero-value World for out-of-range IDs (explicit bounds check
// required since the binary is built with -gcflags="-B").
func (c *WorldCache) Get(id int) World {
	if id < 1 || id > 10000 {
		return World{}
	}
	return c.data[id]
}

// warmupChunkSize is the number of rows loaded per pgx.Batch during warmup.
const warmupChunkSize = 500

// Warmup loads all 10,000 World rows from the database into the cache.
// Uses pgx.Batch in chunks of 500 (20 batches) to avoid query timeout
// and memory spikes. Blocks until all rows are loaded.
func (c *WorldCache) Warmup(pool *pgxpool.Pool) {
	ctx := context.Background()

	for chunkStart := 1; chunkStart <= 10000; chunkStart += warmupChunkSize {
		chunkEnd := chunkStart + warmupChunkSize // exclusive

		batch := &pgx.Batch{}
		for id := chunkStart; id < chunkEnd; id++ {
			batch.Queue("SELECT id, randomnumber FROM world WHERE id = $1", id)
		}

		br := pool.SendBatch(ctx, batch)

		for id := chunkStart; id < chunkEnd; id++ {
			var w World
			if err := br.QueryRow().Scan(&w.ID, &w.RandomNumber); err != nil {
				log.Fatal(fmt.Sprintf("cache warmup failed for id %d: %v", id, err))
			}
			// Explicit bounds check for -gcflags="-B" safety
			if w.ID >= 1 && w.ID <= 10000 {
				c.data[w.ID] = w
			}
		}

		br.Close()
	}
}
