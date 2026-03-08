# Prometheus

Prometheus metrics middleware with request count, duration histogram, and in-flight gauge.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/prometheus
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/prometheus"

app.Use(prometheus.New(prometheus.Config{
    Namespace: "myapp",
}))

app.Get("/metrics", prometheus.Handler())

app.Get("/api/users", func(c *kruda.Ctx) error {
    // Metrics automatically collected
    return c.JSON(kruda.Map{"users": []string{}})
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| Namespace | string | "kruda" | Metric namespace |
| Subsystem | string | "http" | Metric subsystem |
| Skipper | func(*kruda.Ctx) bool | nil | Skip metrics condition |
| Buckets | []float64 | prometheus.DefBuckets | Duration histogram buckets |