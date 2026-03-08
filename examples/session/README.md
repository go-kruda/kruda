# Session

Cookie-based session management using `contrib/session` with an in-memory store.

## Run

```bash
cd examples/session
go run .
```

## Test

```bash
# Login (creates session)
curl -c cookies.txt -X POST http://localhost:3000/login

# Read session
curl -b cookies.txt http://localhost:3000/profile

# Logout (destroys session)
curl -b cookies.txt -X POST http://localhost:3000/logout
```
