// Package wing is a deprecation shim for the Wing transport that was moved
// into the core kruda package in v1.2.0.
//
// Deprecated: import "github.com/go-kruda/kruda" directly. This package will
// be removed in v2.0.0. The same symbols are available unprefixed from kruda
// on all platforms (Linux/Darwin run the real implementation; other platforms
// get a stub that returns "unsupported platform" on Listen).
//
//	// Old (v1.1.x)
//	import "github.com/go-kruda/kruda/transport/wing"
//	t := wing.New(wing.Config{Workers: 4})
//
//	// New (v1.2.0+)
//	import "github.com/go-kruda/kruda"
//	t := kruda.NewWingTransport(kruda.WingConfig{Workers: 4})
package wing

import "github.com/go-kruda/kruda"

// Config is an alias for kruda.WingConfig.
//
// Deprecated: use kruda.WingConfig.
type Config = kruda.WingConfig

// Transport is an alias for kruda.Transport (the Wing transport struct).
//
// Deprecated: use kruda.Transport.
type Transport = kruda.Transport

// Feather is an alias for kruda.Feather.
//
// Deprecated: use kruda.Feather.
type Feather = kruda.Feather

// FeatherOption is an alias for kruda.FeatherOption.
//
// Deprecated: use kruda.FeatherOption.
type FeatherOption = kruda.FeatherOption

// FeatherTable is an alias for kruda.FeatherTable.
//
// Deprecated: use kruda.FeatherTable.
type FeatherTable = kruda.FeatherTable

// DispatchMode is an alias for kruda.DispatchMode.
//
// Deprecated: use kruda.DispatchMode.
type DispatchMode = kruda.DispatchMode

// RawRequest is an alias for kruda.RawRequest.
//
// Deprecated: use kruda.RawRequest.
type RawRequest = kruda.RawRequest

// New returns a Wing transport.
//
// Deprecated: use kruda.NewWingTransport.
func New(cfg Config) *Transport { return kruda.NewWingTransport(cfg) }

// NewFeatherTable builds a FeatherTable from per-route Feather hints.
//
// Deprecated: use kruda.NewFeatherTable.
func NewFeatherTable(routes map[string]Feather, def Feather) FeatherTable {
	return kruda.NewFeatherTable(routes, def)
}

// Dispatch returns a FeatherOption that sets the dispatch mode.
//
// Deprecated: use kruda.Dispatch.
func Dispatch(m DispatchMode) FeatherOption { return kruda.Dispatch(m) }

// Static returns a FeatherOption that sets a pre-built static response.
//
// Deprecated: use kruda.Static.
func Static(resp []byte) FeatherOption { return kruda.Static(resp) }

// Dispatch mode constants.
//
// Deprecated: use kruda.Inline, kruda.Pool, kruda.Spawn, kruda.Takeover.
const (
	Inline   = kruda.Inline
	Pool     = kruda.Pool
	Spawn    = kruda.Spawn
	Takeover = kruda.Takeover
)

// Feather presets.
//
// Deprecated: use kruda.Bolt, kruda.Arrow, kruda.Spear, kruda.Plaintext,
// kruda.JSON, kruda.Query, kruda.Render directly.
var (
	Bolt      = kruda.Bolt
	Arrow     = kruda.Arrow
	Spear     = kruda.Spear
	Plaintext = kruda.Plaintext
	JSON      = kruda.JSON
	Query     = kruda.Query
	Render    = kruda.Render
)
