//go:build !kruda_stdjson && ((amd64 && go1.17 && !go1.27) || (arm64 && go1.20 && !go1.27))

package json

// sonicAccelerated mirrors the build constraint sonic itself uses to decide
// whether to compile its accelerated implementation or route its API to
// encoding/json. Copied verbatim from sonic v1.15.0's sonic.go; its compat.go
// carries the inverse.
//
// Kruda cannot ask sonic which one it got — there is no API for it — and Kruda's
// own guard (`!kruda_stdjson`) says nothing about architecture or Go version. So
// the constraint is mirrored here, and TestSonicConstraintMirrorIsCurrent fails
// when the dependency moves, because a sonic release that supports a new Go
// version makes this copy wrong in the direction that silently loses speed.
const sonicAccelerated = true
