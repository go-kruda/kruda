package main

import (
	"context"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"
)

// Package-level vars for DB pool and cache — benchmark app, not production code.
var (
	db    *pgxpool.Pool
	cache *WorldCache
)

// calcPoolConfig computes MaxConns and MinConns.
// In turbo mode each child gets a fair share of the total pool budget
// (totalWorkers read from KRUDA_WORKERS env, set by Supervisor).
// Budget: 256 total conns across all workers, min 4 per worker.
func calcPoolConfig(gomaxprocs int) (maxConns, minConns int32) {
	workers := gomaxprocs // default: one "worker" per OS thread
	if w, err := strconv.Atoi(os.Getenv("KRUDA_WORKERS")); err == nil && w > 0 {
		workers = w
	}
	perWorker := max(256/workers, 4)
	maxConns = int32(min(perWorker, 256))
	minConns = int32(max(maxConns/4, 1))
	return maxConns, minConns
}

func main() {
	// In turbo mode each child must call SetupChild() before anything else
	// (sets GOMAXPROCS=1 so calcPoolConfig sees the correct value).
	if kruda.IsChild() {
		kruda.SetupChild()
	}

	turbo := os.Getenv("KRUDA_TURBO") == "1"

	// Supervisor: fork children and wait. Never reaches DB setup.
	if turbo && kruda.IsSupervisor() {
		// GoMaxProcs=2: allows goroutine scheduling during DB I/O wait.
		// Processes auto-calculated: NumCPU / GoMaxProcs (e.g. 8 cores → 4 processes).
		// Total parallelism stays at NumCPU. Each child gets 2× DB connections.
		gmax := 2
		if v, err := strconv.Atoi(os.Getenv("KRUDA_TURBO_GOMAXPROCS")); err == nil && v > 0 {
			gmax = v
		}
		procs := 0 // 0 = auto (NumCPU / GoMaxProcs)
		if v, err := strconv.Atoi(os.Getenv("KRUDA_TURBO_PROCESSES")); err == nil && v > 0 {
			procs = v
		}
		sv := &kruda.Supervisor{GoMaxProcs: gmax, Processes: procs}
		if err := sv.Run(); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Database setup — prefer DATABASE_URL env var, fallback to TFB default
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://benchmarkdbuser:benchmarkdbpass@tfb-database/hello_world?sslmode=disable"
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("failed to parse pool config: %v", err)
	}

	gomaxprocs := runtime.GOMAXPROCS(0)
	poolConfig.MaxConns, poolConfig.MinConns = calcPoolConfig(gomaxprocs)
	poolConfig.MaxConnLifetime = time.Hour

	// Prepared statements via AfterConnect — includes UNNEST update to skip Parse+Describe overhead.
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Prepare(ctx, "selectWorld",
			"SELECT id, randomnumber FROM world WHERE id = $1"); err != nil {
			return err
		}
		if _, err := conn.Prepare(ctx, "selectFortune",
			"SELECT id, message FROM fortune"); err != nil {
			return err
		}
		if _, err := conn.Prepare(ctx, "selectWorldBatch",
			"SELECT id, randomnumber FROM world WHERE id = ANY($1::int[]) ORDER BY id"); err != nil {
			return err
		}
		if _, err := conn.Prepare(ctx, "updateWorlds",
			"UPDATE world SET randomnumber = v.r FROM UNNEST($1::int[], $2::int[]) AS v(id, r) WHERE world.id = v.id"); err != nil {
			return err
		}
		return nil
	}

	db, err = pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}
	defer db.Close()

	// Cache warmup — blocks until all 10,000 rows are loaded
	cache = &WorldCache{}
	cache.Warmup(db)

	// Kruda app — no middleware, no logging, no dev mode
	app := kruda.New()

	// Register all 7 TFB routes
	app.Get("/json", jsonHandler)
	app.Get("/plaintext", plaintextHandler)
	app.Get("/db", dbHandler)
	app.Get("/queries", queriesHandler)
	app.Get("/updates", updatesHandler)
	app.Get("/fortunes", fortunesHandler)
	app.Get("/cached-queries", cachedQueriesHandler)

	// Compile routes manually since we bypass Listen()
	app.Compile()

	// Direct fasthttp.Server wiring — bypasses Listen/ServeKruda for max perf
	server := &fasthttp.Server{
		Name:                 "Kruda",
		Handler:              app.ServeFast,
		DisableKeepalive:     false,
		TCPKeepalive:         true,
		NoDefaultDate:        true,
		NoDefaultContentType: true,
		ReduceMemoryUsage:    false,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Kruda TFB listening on :%s (GOMAXPROCS=%d, child=%d)", port, runtime.GOMAXPROCS(0), kruda.ChildID())

	// Always use ReuseportListener for optimized socket options
	// (SO_REUSEPORT + TCP_DEFER_ACCEPT + TCP_FASTOPEN).
	// In turbo mode children share the port; in single mode it still benefits
	// from TCP_DEFER_ACCEPT and TCP_FASTOPEN.
	ln, err := kruda.ReuseportListener(":" + port)
	if err != nil {
		log.Fatalf("listener: %v", err)
	}
	if err := server.Serve(ln); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
