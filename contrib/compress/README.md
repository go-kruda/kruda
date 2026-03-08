# Compress

Gzip and deflate compression middleware for HTTP responses.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/compress
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/compress"

// Auto-compress responses
app.Use(compress.New())

// Manual compression
compress.CompressText(c, "large text data")
compress.Compress(c, data, "application/json")
```

## Config

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Level | int | 6 | Compression level (1-9) |
| MinLength | int | 1024 | Minimum response size |
| Types | []string | text/html, etc. | MIME types to compress |
| Skipper | func | nil | Skip compression function |