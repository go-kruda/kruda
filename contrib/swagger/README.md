# Swagger

Serves Swagger UI HTML pointing to OpenAPI JSON endpoint.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/swagger
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/swagger"

app.Get("/docs/*", swagger.New(swagger.Config{
    JSONPath: "/api/openapi.json",
}))

// Serve your OpenAPI spec
app.Get("/api/openapi.json", func(c *kruda.Ctx) error {
    return c.JSON(yourOpenAPISpec)
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| JSONPath | string | "/openapi.json" | OpenAPI JSON endpoint |
| Title | string | "API Documentation" | Swagger UI title |
| DeepLinking | bool | true | Enable deep linking |
| DocExpansion | string | "list" | Default expansion |