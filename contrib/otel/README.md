# OpenTelemetry

OpenTelemetry tracing middleware with server spans and HTTP semantic conventions.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/otel
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/otel"

app.Use(otel.New(otel.Config{
    ServerName: "my-api",
}))

app.Get("/users", func(c *kruda.Ctx) error {
    // Span automatically created and propagated
    return c.JSON(kruda.Map{"users": []string{}})
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| TracerProvider | trace.TracerProvider | global provider | OTel tracer provider |
| Propagators | propagation.TextMapPropagator | global propagator | Propagators for context extraction |
| ServerName | string | "" (omitted) | Server name in span attributes |
| Skip | func(*kruda.Ctx) bool | nil | Skip tracing for matching requests |
| SpanNameFunc | func(*kruda.Ctx) string | "METHOD route" | Custom span names |
| Attributes | []attribute.KeyValue | nil | Extra attributes on every span |
