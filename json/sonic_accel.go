//go:build !kruda_stdjson && ((amd64 && go1.17 && !go1.28) || (arm64 && go1.20 && !go1.28))

package json

// sonicAccelerated mirrors the build constraint sonic itself uses to decide
// whether to compile its accelerated implementation or route its API to
// encoding/json. Copied verbatim from sonic v1.15.4's sonic.go; its compat.go
// carries the inverse.
//
// Sonic also exposes APIKind. This existing constraint mirror keeps Kruda's
// engine signal a build-time constant; TestSonicConstraintMirrorIsCurrent fails
// when the dependency moves, because a sonic release that supports a new Go
// version makes this copy wrong in the direction that silently loses speed.
const sonicAccelerated = true
