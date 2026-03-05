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
    ServiceName: "my-api",
}))

app.Get("/users", func(c *kruda.Ctx) error {
    // Span automatically created and propagated
    return c.JSON(kruda.Map{"users": []string{}})
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| ServiceName | string | "kruda-app" | Service name in traces |
| Skipper | func(*kruda.Ctx) bool | nil | Skip tracing condition |
| SpanNameFormatter | func(*kruda.Ctx) string | "METHOD /path" | Custom span names |
| ExtraAttributes | []attribute.KeyValue | nil | Additional span attributes |