# Health Checks

Auto-discovers `HealthChecker` services from the DI container and runs them in parallel with configurable timeout.

## Run

```bash
go run -tags kruda_stdjson ./examples/health-checks/
```

## Test

```bash
# All healthy (200)
curl http://localhost:3000/health

# Break database
curl http://localhost:3000/break

# Unhealthy (503)
curl http://localhost:3000/health

# Restore database
curl http://localhost:3000/fix
```
