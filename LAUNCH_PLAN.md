# Kruda v1.0.0 Launch Plan

## Overview
Public launch of Kruda Go web framework with comprehensive CI/CD, documentation, playground, and TechEmpower Framework Benchmarks submission.

**Target Date:** TBD  
**Repository:** github.com/go-kruda/kruda  
**Infrastructure:** Intel i5-13500, Ubuntu 24.04  

---

## 1. CI/CD Pipeline Enhancement

### 1.1 Release Workflow (.github/workflows/release.yml)

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write
  packages: write

jobs:
  test:
    uses: ./.github/workflows/test.yml
    
  release:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          
      - uses: actions/setup-go@v5
        with:
          go-version: stable
          
      - name: Generate changelog
        run: |
          # Extract version from tag
          VERSION=${GITHUB_REF#refs/tags/}
          
          # Generate changelog from commits since last tag
          PREV_TAG=$(git describe --tags --abbrev=0 HEAD^ 2>/dev/null || echo "")
          if [ -n "$PREV_TAG" ]; then
            git log --oneline --no-merges ${PREV_TAG}..HEAD > CHANGELOG_CURRENT.md
          else
            git log --oneline --no-merges > CHANGELOG_CURRENT.md
          fi
          
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          body_path: CHANGELOG_CURRENT.md
          generate_release_notes: true
          prerelease: ${{ contains(github.ref, 'alpha') || contains(github.ref, 'beta') || contains(github.ref, 'rc') }}
```

### 1.2 Pre-Release Quality Gates

**Required Checks Before v1.0.0 Tag:**
- [ ] All tests pass (Go 1.25 + stable × Linux/macOS/Windows)
- [ ] golangci-lint clean
- [ ] Benchmark regression < 5% vs baseline
- [ ] TFB verification passes locally
- [ ] Documentation builds successfully
- [ ] Security scan (govulncheck) clean
- [ ] Go mod tidy + vendor check

### 1.3 Enhanced Test Matrix (.github/workflows/test.yml updates)

Add to existing test.yml:

```yaml
  security:
    name: Security Scan
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: Run govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...
          
  mod-verify:
    name: Module Verification
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: Verify dependencies
        run: |
          go mod tidy
          go mod verify
          git diff --exit-code go.mod go.sum
```

---

## 2. TechEmpower Framework Benchmarks Submission

### 2.1 TFB Directory Structure
```
frameworks/Go/kruda/
├── benchmark_config.json
├── kruda.dockerfile
├── src/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── handlers.go
│   └── pool.go
└── README.md
```

### 2.2 Local TFB Verification Script

```bash
#!/usr/bin/env bash
# scripts/verify-tfb.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

log() { echo -e "\033[0;36m[tfb-verify]\033[0m $*"; }
ok() { echo -e "\033[0;32m[OK]\033[0m $*"; }
fail() { echo -e "\033[0;31m[FAIL]\033[0m $*"; exit 1; }

# Check TFB structure
log "Verifying TFB directory structure..."
TFB_DIR="$ROOT_DIR/frameworks/Go/kruda"
[ -f "$TFB_DIR/benchmark_config.json" ] || fail "Missing benchmark_config.json"
[ -f "$TFB_DIR/kruda.dockerfile" ] || fail "Missing kruda.dockerfile"
[ -f "$TFB_DIR/src/main.go" ] || fail "Missing src/main.go"
ok "TFB structure valid"

# Build Docker image
log "Building TFB Docker image..."
cd "$TFB_DIR"
docker build -f kruda.dockerfile -t kruda-tfb . || fail "Docker build failed"
ok "Docker image built"

# Test container startup
log "Testing container startup..."
CONTAINER_ID=$(docker run -d -p 8080:8080 kruda-tfb)
sleep 5

# Verify endpoints
ENDPOINTS=("/json" "/plaintext" "/db" "/queries?queries=20" "/fortunes" "/cached-queries?count=20" "/updates?queries=20")
for endpoint in "${ENDPOINTS[@]}"; do
    log "Testing $endpoint..."
    if curl -sf "http://localhost:8080$endpoint" >/dev/null; then
        ok "$endpoint responds"
    else
        fail "$endpoint failed"
    fi
done

# Cleanup
docker stop "$CONTAINER_ID" >/dev/null
docker rm "$CONTAINER_ID" >/dev/null
ok "TFB verification complete"
```

### 2.3 TFB Configuration Files

**benchmark_config.json:**
```json
{
  "framework": "kruda",
  "tests": [{
    "default": {
      "json_url": "/json",
      "plaintext_url": "/plaintext", 
      "db_url": "/db",
      "query_url": "/queries?queries=",
      "fortune_url": "/fortunes",
      "cached_query_url": "/cached-queries?count=",
      "update_url": "/updates?queries=",
      "port": 8080,
      "approach": "Realistic",
      "classification": "Micro",
      "database": "Postgres",
      "framework": "kruda",
      "language": "Go",
      "flavor": "None",
      "orm": "Raw",
      "platform": "Go",
      "webserver": "None",
      "os": "Linux",
      "database_os": "Linux",
      "display_name": "kruda",
      "notes": "Kruda web framework with Wing transport (io_uring)",
      "versus": "go"
    }
  }]
}
```

---

## 3. Documentation Site (kruda.dev)

### 3.1 VitePress Setup

**docs/package.json:**
```json
{
  "name": "kruda-docs",
  "scripts": {
    "dev": "vitepress dev",
    "build": "vitepress build",
    "preview": "vitepress preview"
  },
  "devDependencies": {
    "vitepress": "^1.0.0",
    "@types/node": "^20.0.0"
  }
}
```

**docs/.vitepress/config.ts:**
```typescript
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Kruda',
  description: 'High-performance Go web framework',
  base: '/',
  
  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'API', link: '/api/' },
      { text: 'Benchmarks', link: '/benchmarks' },
      { text: 'Playground', link: 'https://play.kruda.dev' }
    ],
    
    sidebar: {
      '/guide/': [
        { text: 'Getting Started', link: '/guide/' },
        { text: 'Routing', link: '/guide/routing' },
        { text: 'Middleware', link: '/guide/middleware' },
        { text: 'Context', link: '/guide/context' },
        { text: 'Transport', link: '/guide/transport' },
        { text: 'Deployment', link: '/guide/deployment' }
      ]
    },
    
    socialLinks: [
      { icon: 'github', link: 'https://github.com/go-kruda/kruda' }
    ]
  }
})
```

### 3.2 Deployment Strategy

**Option A: GitHub Pages (Recommended)**
- Free hosting
- Automatic deployment via existing docs.yml workflow
- Custom domain: kruda.dev → GitHub Pages
- SSL certificate managed by GitHub

**Option B: Vercel**
- Better performance (global CDN)
- Preview deployments for PRs
- Requires Vercel account setup

**DNS Configuration:**
```
kruda.dev.        A     185.199.108.153
kruda.dev.        A     185.199.109.153  
kruda.dev.        A     185.199.110.153
kruda.dev.        A     185.199.111.153
www.kruda.dev.    CNAME go-kruda.github.io.
```

---

## 4. Playground (play.kruda.dev)

### 4.1 Architecture Options

**Option A: Go Playground API Wrapper (Recommended)**
```go
// Simple proxy to official Go Playground
func playgroundHandler(c *kruda.Context) error {
    // Forward to play.golang.org with Kruda imports pre-added
    // Add security headers, rate limiting
}
```

**Option B: Custom Sandbox**
- Docker containers with timeout
- Code execution in isolated environment
- More complex but full control

### 4.2 Deployment

**Infrastructure:**
- Single binary on Intel i5-13500 server
- Nginx reverse proxy with rate limiting
- SSL certificate via Let's Encrypt
- Systemd service for auto-restart

**Security:**
- Rate limiting: 10 requests/minute per IP
- Code size limit: 64KB
- Execution timeout: 10 seconds
- No network access from sandbox

### 4.3 Implementation

```go
// cmd/playground/main.go
package main

import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/transport/wing"
)

func main() {
    app := kruda.New()
    
    // CORS for frontend
    app.Use(func(c *kruda.Context) error {
        c.Set("Access-Control-Allow-Origin", "https://kruda.dev")
        return c.Next()
    })
    
    // Rate limiting middleware
    app.Use(rateLimitMiddleware(10, time.Minute))
    
    // Playground endpoints
    app.POST("/compile", compileHandler)
    app.POST("/share", shareHandler)
    app.GET("/p/:id", loadSharedHandler)
    
    app.Listen(":8080", wing.New())
}
```

---

## 5. GitHub Release Automation

### 5.1 Release Process

**Manual Steps:**
1. Update CHANGELOG.md
2. Update version in documentation
3. Run pre-release checklist
4. Create and push tag: `git tag v1.0.0 && git push origin v1.0.0`

**Automated Steps:**
1. GitHub Actions triggers on tag push
2. Runs full test suite
3. Generates release notes from commits
4. Creates GitHub Release with artifacts
5. Updates documentation site
6. Notifies relevant channels

### 5.2 Pre-Release Checklist Script

```bash
#!/usr/bin/env bash
# scripts/pre-release-check.sh
set -euo pipefail

VERSION="$1"
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Usage: $0 v1.0.0"
    exit 1
fi

echo "🔍 Pre-release checks for $VERSION"

# Test suite
echo "Running tests..."
go test -race -tags kruda_stdjson ./... || exit 1

# Linting
echo "Running linter..."
golangci-lint run --build-tags kruda_stdjson || exit 1

# Security scan
echo "Security scan..."
govulncheck ./... || exit 1

# Benchmark regression
echo "Benchmark regression check..."
go test -bench=. -benchmem -count=3 -tags kruda_stdjson ./bench/... > new_bench.txt
if [ -f baseline_bench.txt ]; then
    benchstat baseline_bench.txt new_bench.txt || echo "⚠️  Benchmark changes detected"
fi

# TFB verification
echo "TFB verification..."
./scripts/verify-tfb.sh || exit 1

# Documentation build
echo "Documentation build..."
cd docs && npm ci && npm run build || exit 1

echo "✅ All checks passed for $VERSION"
echo "Ready to tag: git tag $VERSION && git push origin $VERSION"
```

---

## 6. Infrastructure Setup

### 6.1 Server Configuration (Intel i5-13500, Ubuntu 24.04)

**System Setup:**
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install dependencies
sudo apt install -y nginx certbot python3-certbot-nginx docker.io

# Configure firewall
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp  
sudo ufw allow 443/tcp
sudo ufw enable

# Install Go 1.24+
wget https://go.dev/dl/go1.24.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

**Nginx Configuration:**
```nginx
# /etc/nginx/sites-available/kruda
server {
    server_name kruda.dev www.kruda.dev;
    return 301 https://go-kruda.github.io$request_uri;
}

server {
    server_name play.kruda.dev;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        # Rate limiting
        limit_req zone=playground burst=20 nodelay;
    }
}

# Rate limiting zone
http {
    limit_req_zone $binary_remote_addr zone=playground:10m rate=10r/m;
}
```

### 6.2 Deployment Scripts

**deploy-playground.sh:**
```bash
#!/usr/bin/env bash
set -euo pipefail

APP_DIR="/opt/kruda-playground"
SERVICE_NAME="kruda-playground"

# Build latest
cd "$APP_DIR"
git pull origin main
go build -o playground ./cmd/playground/

# Restart service
sudo systemctl restart "$SERVICE_NAME"
sudo systemctl status "$SERVICE_NAME"

echo "✅ Playground deployed"
```

**Systemd Service:**
```ini
# /etc/systemd/system/kruda-playground.service
[Unit]
Description=Kruda Playground
After=network.target

[Service]
Type=simple
User=kruda
WorkingDirectory=/opt/kruda-playground
ExecStart=/opt/kruda-playground/playground
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## 7. Launch Timeline

### Phase 1: Infrastructure (Week 1)
- [ ] Set up Intel i5-13500 server
- [ ] Configure Nginx + SSL certificates
- [ ] Deploy playground service
- [ ] Set up DNS (kruda.dev, play.kruda.dev)

### Phase 2: CI/CD (Week 2)  
- [ ] Enhance GitHub Actions workflows
- [ ] Create release automation
- [ ] Set up TFB verification
- [ ] Test full pipeline with beta tags

### Phase 3: Documentation (Week 3)
- [ ] Complete VitePress documentation
- [ ] Set up GitHub Pages deployment
- [ ] Create API reference
- [ ] Add benchmark results page

### Phase 4: TFB Submission (Week 4)
- [ ] Finalize TFB implementation
- [ ] Submit PR to TechEmpower/FrameworkBenchmarks
- [ ] Address review feedback
- [ ] Merge and verify in official results

### Phase 5: Launch (Week 5)
- [ ] Final pre-release checks
- [ ] Tag v1.0.0
- [ ] Announce on social media
- [ ] Submit to Go community channels
- [ ] Monitor metrics and feedback

---

## 8. Success Metrics

**Technical Metrics:**
- CI/CD pipeline success rate > 95%
- Documentation site uptime > 99.9%
- Playground response time < 2s
- TFB ranking in top 10 Go frameworks

**Community Metrics:**
- GitHub stars growth
- Documentation page views
- Playground usage statistics
- Community feedback and contributions

**Performance Targets:**
- TFB plaintext: > 400K req/s
- TFB JSON: > 350K req/s  
- Memory usage: < 50MB baseline
- Cold start: < 100ms

---

## 9. Risk Mitigation

**Technical Risks:**
- TFB submission rejection → Early community review
- Performance regression → Automated benchmark gates
- Security vulnerabilities → Regular govulncheck scans
- Infrastructure downtime → Monitoring + alerts

**Timeline Risks:**
- Delayed TFB review → Submit early, iterate
- Documentation incomplete → Prioritize core features
- Server setup issues → Cloud backup plan (DigitalOcean)

**Community Risks:**
- Low adoption → Focus on performance benchmarks
- Negative feedback → Responsive issue handling
- Competition → Emphasize unique Wing transport

---

This plan provides a comprehensive roadmap for launching Kruda v1.0.0 with professional-grade infrastructure, documentation, and community presence.