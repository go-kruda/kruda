//go:build linux || darwin

package kruda

import "log/slog"

const (
	acceptCapCeiling  = 262144 // bounds the DERIVED default only; explicit WithMaxConns may exceed
	acceptCapReserve  = 256    // stdio + DB pool + open files + slack
	acceptCapLowFloor = 1024   // below this, warn at startup
)

// rejectKind identifies which accept-side limit refused a connection.
type rejectKind int

const (
	rejectKindTotal rejectKind = iota // global total connection cap
	rejectKindIP                      // per-source-IP connection cap
	rejectKindRate                    // per-worker accept-rate token bucket
)

// rejectWarnOnce logs the first rejection of each kind per worker (blocking-advisor
// pattern) so a flood does not flood the log. Each worker may warn once per kind.
func rejectWarnOnce(w *worker, k rejectKind, limit int) {
	if w.rejectWarned[k] {
		return
	}
	w.rejectWarned[k] = true
	lg := w.logger
	if lg == nil {
		lg = slog.Default()
	}
	switch k {
	case rejectKindTotal:
		lg.Warn("kruda/wing: connection cap reached, rejecting (RST); raise WithMaxConns or investigate abuse", "max", limit)
	case rejectKindIP:
		lg.Warn("kruda/wing: per-IP connection limit reached, rejecting (RST)", "perIP", limit)
	case rejectKindRate:
		lg.Warn("kruda/wing: accept-rate limit reached, rejecting (RST)")
	}
}

// tokenBucket is a single-threaded (no locks needed on the event-loop goroutine)
// leaky-bucket rate limiter for accept-side rate limiting. Per-worker, so the
// effective server-wide rate is approximately perSec*workers under SO_REUSEPORT.
type tokenBucket struct {
	tokens      float64
	burst       float64
	ratePerNano float64
	last        int64 // unix nano of last refill
}

// newTokenBucket returns a bucket that allows perSec tokens per second with a
// burst allowance of burst. If burst < perSec, burst is raised to perSec to
// absorb SO_REUSEPORT distribution skew.
func newTokenBucket(perSec, burst int) *tokenBucket {
	if burst < perSec {
		burst = perSec // burst slack absorbs SO_REUSEPORT skew
	}
	return &tokenBucket{tokens: float64(burst), burst: float64(burst), ratePerNano: float64(perSec) / 1e9}
}

// allow refills based on elapsed nanos since last call, then spends one token.
// Returns false when no tokens are available (connection should be rejected).
// When now==0 (e.g. in unit tests using synthetic time), no refill is applied
// on the first call because 0>0 is false; this is the intended behavior.
func (b *tokenBucket) allow(now int64) bool {
	if now > b.last {
		b.tokens += float64(now-b.last) * b.ratePerNano
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// deriveMaxConns computes the default total connection cap from the fd soft
// limit. headroom reserves ~3 fds/worker (listen, epoll, eventfd) plus a fixed
// reserve for stdio/DB-pool/open files. Result is clamped to acceptCapCeiling so
// LimitNOFILE=infinity does not make the default effectively unlimited.
func deriveMaxConns(rlimitSoft uint64, workers int) int {
	headroom := uint64(3*workers + acceptCapReserve)
	if rlimitSoft <= headroom {
		return 1 // pathological tiny ulimit; cap at 1 rather than <=0 (0 means unlimited)
	}
	avail := rlimitSoft - headroom
	if avail > acceptCapCeiling {
		return acceptCapCeiling
	}
	return int(avail)
}
