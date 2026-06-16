package observability

import "time"

// Config configures Enable. Zero value is usable: ServiceName falls back to
// OTEL_SERVICE_NAME then "unknown_service"; SetGlobal defaults to true.
type Config struct {
	ServiceName  string
	SetGlobal    *bool         // nil => true; set false to leave OTel globals untouched
	FlushTimeout time.Duration // bounded shutdown flush; default 5s, floor 1s

	MetricsEnabled *bool // nil => true
	TracesEnabled  *bool // nil => true
	HealthEnabled  *bool // nil => true (gates /livez,/readyz,/health)

	MetricsPath   string // default "/metrics"
	MetricsBind   string // separate listener addr; empty => mount on app at MetricsPath
	MetricsPublic bool   // mount /metrics on the app/public port (default false)
	MetricsAuth   string // bearer token; empty => no auth

	LivenessPath  string // default "/livez"
	ReadinessPath string // default "/readyz"
	HealthPath    string // default "/health"; a readiness alias of ReadinessPath. Not mounted when it equals ReadinessPath. To turn off all probes use HealthEnabled:false.

	SampleRatio      float64  // <=0 => use OTEL_TRACES_SAMPLER / ParentBased default
	RedactHeaders    []string // extra header names to redact (case-insensitive)
	PropagateBaggage bool
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// resolved holds defaulted config values, computed once in Enable.
type resolved struct {
	serviceName   string
	setGlobal     bool
	flushTimeout  time.Duration
	metricsOn     bool
	tracesOn      bool
	healthOn      bool
	metricsPath   string
	livenessPath  string
	readinessPath string
	healthPath    string
	sampleRatio   float64
}

func (c Config) resolve() resolved {
	ft := c.FlushTimeout
	if ft <= 0 {
		ft = 5 * time.Second
	}
	if ft < time.Second {
		ft = time.Second
	}
	pathOr := func(v, def string) string {
		if v == "" {
			return def
		}
		return v
	}
	return resolved{
		serviceName:   c.ServiceName,
		setGlobal:     boolOr(c.SetGlobal, true),
		flushTimeout:  ft,
		metricsOn:     boolOr(c.MetricsEnabled, true),
		tracesOn:      boolOr(c.TracesEnabled, true),
		healthOn:      boolOr(c.HealthEnabled, true),
		metricsPath:   pathOr(c.MetricsPath, "/metrics"),
		livenessPath:  pathOr(c.LivenessPath, "/livez"),
		readinessPath: pathOr(c.ReadinessPath, "/readyz"),
		healthPath:    pathOr(c.HealthPath, "/health"),
		sampleRatio:   c.SampleRatio,
	}
}
