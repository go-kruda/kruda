# Releasing Kruda

This guide covers how to cut a new release of Kruda (core + all sub-modules).

## Prerequisites

- You have push access to `main` and tags
- All tests pass on `main`
- No uncommitted changes locally

## Version Naming

Kruda follows [Semantic Versioning](https://semver.org/):

- **Major** (`v2.0.0`) — breaking API changes
- **Minor** (`v1.1.0`) — new public API, backward compatible
- **Patch** (`v1.0.1`) — bug fixes only, no API changes

All sub-modules (`transport/wing`, `contrib/*`) use the same version as core.

## The Core Problem: Circular Dependency

Kruda has a structural issue: `transport/wing` imports `github.com/go-kruda/kruda/transport` (sub-package of core), which forces `transport/wing/go.mod` to require the core module. Meanwhile, core's `go.mod` requires `transport/wing`.

This creates a circular dependency at release time:
- Core `v1.X.Y` requires `wing v1.X.Y`
- Wing `v1.X.Y` requires core `v1.X.Y`
- But one must be tagged before the other can reference it

**Solution:** Sub-modules reference the **previous** core version. This breaks the cycle.

## Release Flow (2-Commit Pattern)

### Step 1: Prepare

```bash
git checkout main
git pull
go test -tags kruda_stdjson ./...
```

### Step 2: Commit A — Update sub-module go.mod files

Update all sub-module `go.mod` files to require the **latest published core version**, not the new one being released.

Example: If releasing `v1.1.3` and current published core is `v1.1.2`:

```bash
# In every contrib/*/go.mod and transport/wing/go.mod:
# require github.com/go-kruda/kruda v1.1.2  (NOT v1.1.3)
```

If there are retracted versions, also add `retract` to core `go.mod`:

```go
retract (
    v1.1.2 // reason
    v1.1.1 // reason
)
```

Commit:

```bash
git add go.mod contrib/*/go.mod transport/wing/go.mod
git commit -m "chore: prepare v1.1.3 release"
git push origin main
```

### Step 3: Tag sub-modules at Commit A

```bash
VERSION=v1.1.3
for mod in contrib/cache contrib/compress contrib/etag contrib/jwt \
           contrib/otel contrib/prometheus contrib/ratelimit \
           contrib/session contrib/swagger contrib/ws transport/wing; do
    git tag -a "$mod/$VERSION" -m "$mod $VERSION"
done

git push origin \
    contrib/cache/$VERSION contrib/compress/$VERSION contrib/etag/$VERSION \
    contrib/jwt/$VERSION contrib/otel/$VERSION contrib/prometheus/$VERSION \
    contrib/ratelimit/$VERSION contrib/session/$VERSION contrib/swagger/$VERSION \
    contrib/ws/$VERSION transport/wing/$VERSION
```

### Step 4: Trigger Go module proxy to index sub-module tags

```bash
GONOSUMCHECK=* GONOSUMDB=* go list -m github.com/go-kruda/kruda/transport/wing@$VERSION
```

Must return the version (not error).

### Step 5: Commit B — Update core go.mod to new wing version

```bash
# Update go.mod: require github.com/go-kruda/kruda/transport/wing v1.1.3
GONOSUMCHECK=* GONOSUMDB=* GOWORK=off go mod tidy

git add go.mod go.sum
git commit -m "chore: bump transport/wing to $VERSION"
git push origin main
```

### Step 6: Tag core at Commit B

```bash
git tag -a $VERSION -m "$VERSION"
git push origin $VERSION
```

### Step 7: Verify

```bash
# Trigger proxy
GONOSUMCHECK=* GONOSUMDB=* go list -m github.com/go-kruda/kruda@$VERSION

# Test downstream install
mkdir /tmp/test-kruda && cd /tmp/test-kruda
cat > go.mod << EOF
module testkruda
go 1.25
EOF
cat > main.go << 'EOF'
package main
import "github.com/go-kruda/kruda"
func main() { _ = kruda.New() }
EOF
GOWORK=off go mod tidy
GOWORK=off go build ./...
# Should succeed without errors
```

### Step 8: Trigger pkg.go.dev indexing

```bash
curl -s "https://pkg.go.dev/fetch/github.com/go-kruda/kruda@$VERSION" -X POST
```

Verify at `https://pkg.go.dev/github.com/go-kruda/kruda@$VERSION` (usually available within minutes).

## Critical Rules

### Never do these

- **NEVER force-push a tag** on a public Go module. The Go checksum database (`sum.golang.org`) caches the module hash permanently when a tag is first published. Re-tagging = permanent checksum mismatch for all downstream users.
- **NEVER delete and re-create** a tag at a different commit for the same reason.
- **NEVER skip `go mod tidy`** after updating go.mod version references.
- **NEVER release without testing downstream install** via a clean project.

### If a release is broken

1. **Do not try to "fix" by re-tagging.** The broken tag is permanent.
2. **Bump to the next patch version** (e.g. `v1.1.2` → `v1.1.3`).
3. **Add `retract` directive** in core `go.mod` for the broken version:
   ```go
   retract v1.1.2 // reason for retraction
   ```
4. Release the new version following the flow above.
5. Users running `go get ...@latest` will skip retracted versions. Users explicitly using a retracted version get a warning.

## Troubleshooting

### "unknown revision" from proxy

Proxy hasn't indexed the tag yet. Trigger with:

```bash
GONOSUMCHECK=* GONOSUMDB=* go list -m github.com/go-kruda/kruda/transport/wing@v1.1.3
```

Wait 10–30 seconds and retry.

### "checksum mismatch"

One of:
- A tag was force-pushed (see "Never do these")
- Local `go.sum` is out of date — run `go mod tidy`
- You're using `go.work` locally which hides the real proxy resolution — set `GOWORK=off`

### "missing go.sum entry"

Run `go mod tidy` in the module that shows the error:

```bash
# For core
GOWORK=off go mod tidy

# For wing
cd transport/wing && GOWORK=off go mod tidy

# For contrib
cd contrib/jwt && GOWORK=off go mod tidy
```

## Long-term Improvement

The circular dependency between `core` and `transport/wing` is a structural issue. Future refactoring could:

1. Move `transport/` interfaces into a separate module (e.g. `github.com/go-kruda/kruda-transport`) that neither core nor wing's implementations depend on each other through.
2. This would allow sub-modules to release independently without the 2-commit dance.

Until then, follow the flow above.
