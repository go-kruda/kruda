//go:build !kruda_stdjson && !((amd64 && go1.17 && !go1.28) || (arm64 && go1.20 && !go1.28))

package json

// sonicAccelerated is false where sonic routes its own API to encoding/json:
// architectures it has no assembly for, and Go versions it has not validated
// (sonic v1.15.4 excludes go1.28 and newer). See sonic_accel.go.
const sonicAccelerated = false
