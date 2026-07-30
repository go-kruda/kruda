# Does the JSON engine change req/sec? — 2026-07-29

The engine benchmarks show Sonic about 35% faster to encode and about 6× faster
to decode than `encoding/json`. Those measure one function call, not one request.
This answers whether the difference reaches throughput, which matters because
v1.7.0 silently flips `CGO_ENABLED=0` builds from `encoding/json` to Sonic.

**It depends entirely on payload shape, and the spread is enormous: from nothing
to 2.6×.**

| environment | |
|---|---|
| host | 8-core 13th Gen Intel i5-13500, Linux, shared box |
| server | `KRUDA_WORKERS=8 GOMAXPROCS=8`, `kruda.JSON` preset (inline dispatch) |
| load | `wrk -t4 -c256`, 3 s warm-up, 10 s measured |
| design | 5 rounds, arms round-robin with the order reversed on even rounds; noise band is one arm's own spread across rounds |

Same binary built with and without `kruda_stdjson`. Reproduce with
`bench/reproducible/jsonthroughput/jsonthroughput.sh`.

## Results

| route | payload | encoding/json | sonic | delta | noise band |
|---|---|---|---|---|---|
| `/small` | encode ~30 B | 710,159 | 700,971 | **−1.3%** | ±2.58% — inside |
| `/large` | encode ~8 KB | 256,242 | 315,244 | **+23.0%** | ±1.35% — outside |
| `/decode` | decode ~8 KB POST | 34,203 | 89,054 | **+160%** | ±1.07% — outside |

Paired by round, every route is consistent in direction and tight in spread:

```
/small    -3.0% +0.3% -1.1% -1.4% -1.2%   mean -1.28%
/large   +23.0% +23.9% +21.4% +23.9% +22.9%  mean +23.03%
/decode  +156.3% +161.4% +159.8% +163.1% +161.3%  mean +160.39%
```

## Reading it

**Small responses gain nothing.** −1.3% sits inside the noise band, so the honest
statement is "no measurable difference", not "1.3% slower". This is the shape
TFB-style `/json` benchmarks use, and it is exactly what the syscall floor
predicts: at ~710k req/s a worker spends ~11 µs per request, and encoding 30
bytes is a rounding error against that. Kruda's throughput on this shape was
already at the kernel's TCP cost per request.

**Encoding a few kilobytes gains 23%.** Once the encoder does real work, the
microbenchmark's 35% shows up attenuated but clearly outside noise.

**Decoding is where it is decisive: 2.6×.** Note the absolute numbers — 34k
against 700k on `/small`. Decoding an 8 KB body dominates that route's cost
completely, so the engine's 6× advantage translates almost directly.

## What this means for the v1.7.0 upgrade

A service whose endpoints are small JSON reads will see no throughput change. A
service that accepts JSON bodies of any size — form submissions, batch writes,
anything POSTed — can see its request handling get several times cheaper. List
endpoints returning arrays land in between.

So "does v1.7.0 make it faster" cannot be answered without knowing the traffic
mix. It cannot make small-response throughput worse in any measurable way, and it
can make body-heavy endpoints dramatically cheaper.

Cold start is the other side of the same flip: +3 ms and +7 MB RSS per process,
measured in `../2026-07-29-coldstart/`.
