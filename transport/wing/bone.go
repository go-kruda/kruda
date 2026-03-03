package wing

// Bone is the engine-level optimization config for Wing.
// Unlike Feather (per-route), Bone affects the entire I/O engine —
// every connection benefits when a Bone is enabled.
//
//	wing.New(wing.Config{
//	    Bone: wing.Bone{
//	        BatchWrite:    true,
//	        SkipTimestamp: true,
//	    },
//	})
type Bone struct {
	// BatchWrite coalesces pipelined responses into a single write syscall.
	// If read buffer contains N requests, parse all N → append N responses → 1 write.
	// Impact: reduces write syscalls up to 16x for pipelined clients.
	BatchWrite bool

	// SkipTimestamp skips time.Now() per request when no timeout is configured.
	// Impact: saves ~25ns per request (time.runtimeNow was 1.6% in pprof).
	SkipTimestamp bool
}

// BoneOption modifies a single Bone setting.
type BoneOption func(*Bone)

func BatchWrite(b *Bone)    { b.BatchWrite = true }
func SkipTimestamp(b *Bone) { b.SkipTimestamp = true }

// Skeleton returns a Bone with common benchmark optimizations enabled.
var Skeleton = Bone{
	BatchWrite:    true,
	SkipTimestamp: true,
}
