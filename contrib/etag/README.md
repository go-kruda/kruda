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
| Weak | bool | false | Generate weak ETags |
| Skipper | func | nil | Skip ETag function |
| Generator | func | nil | Custom ETag generator |