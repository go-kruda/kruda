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
| Level | int | -1 | Compression level; -1 uses the stdlib default |
| MinSize | int | 1024 | Minimum response size |
| Types | []string | text/html, etc. | MIME types to compress |
| Skip | func(*kruda.Ctx) bool | nil | Skip compression function |

Upgrade core alongside this module for fasthttp `Vary` preservation. Older cores
whose queued `AddHeader` overwrites direct fasthttp headers cannot preserve a
`Vary` value previously written by `SetHeader`. The updated core merges both
sources before writing the response.
