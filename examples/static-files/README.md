# Static Files

Serves embedded static files using `app.StaticFS()` with Go's `embed.FS`.

## Run

```bash
go run ./examples/static-files/
```

## Test

```bash
curl http://localhost:3000/static/hello.txt
curl http://localhost:3000/static/style.css

# Or open http://localhost:3000 in a browser
```
