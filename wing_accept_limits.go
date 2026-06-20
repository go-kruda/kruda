package kruda

const (
	acceptCapCeiling  = 262144 // bounds the DERIVED default only; explicit WithMaxConns may exceed
	acceptCapReserve  = 256    // stdio + DB pool + open files + slack
	acceptCapLowFloor = 1024   // below this, warn at startup
)

// rejectKind identifies which accept-side limit refused a connection. Task 6
// replaces rejectWarnOnce with the real warn-once-per-route/kind logger.
type rejectKind int

const (
	rejectKindTotal rejectKind = iota // global total connection cap
)

// rejectWarnOnce is a no-op stub until Task 6 wires the warn-once reject logger.
func rejectWarnOnce(_ *worker, _ rejectKind, _ int) {}

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
