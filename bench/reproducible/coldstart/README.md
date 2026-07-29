# Cold start and startup RSS

Reproduces the CHANGELOG's claim that `CGO_ENABLED=0` builds — which used to get
`encoding/json` and now get Sonic — pay for Sonic's JIT warm-up at process
start.

```bash
./coldstart.sh                            # 15 spawns, route counts 1 and 25
SPAWNS=25 ROUTE_COUNTS="1 5 25" ./coldstart.sh
```

Run it on Linux. On macOS the driver reports RSS as 0, because it reads
`/proc/<pid>/status`, and first-exec overhead skews the timings.

## What it measures, and why this way

`driver/` is a Go program rather than a shell loop. It starts the clock
immediately before `exec`, inside the process that spawns, so the measurement
includes Go runtime init — which is where a JIT's warm-up lands. `footprint.sh`
starts timing after the shell has already backgrounded the process, and polls
with `curl` at 2 ms granularity; on a ~6 ms measurement that is a 30% error bar.
`footprint.sh` also says of itself that it is not a claim source.

RSS is reported at two defined points: the instant the server first answers, and
after a 500 ms settle. A JIT allocates during warm-up, so the two can differ.
Any published number should say which one it is.

Route count is swept rather than fixed at 25. The claim ties cold start to "25
typed POST routes with distinct 5-field schemas", the premise being that typed
handler registration drives warm-up. If the number does not move with route
count, that framing is incidental.

`app/` declares 32 distinct 5-field input types because generics resolve at
compile time — a loop over a slice of types would share one compiled encoder and
measure nothing.
