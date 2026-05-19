# ETag

ETag middleware with conditional GET/HEAD support and 304 Not Modified responses.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/etag
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/etag"

// Auto-generate ETags
app.Use(etag.New())

// Manual ETag generation
etag.GenerateAndSetETag(c, responseBody)
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Weak | bool | true | Generate weak ETags |
| Skip | func(*kruda.Ctx) bool | nil | Skip ETag function |
