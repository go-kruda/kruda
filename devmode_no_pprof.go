//go:build !kruda_pprof

package kruda

const pprofAvailable = false

func (*App) registerPprofRoutes() {}
