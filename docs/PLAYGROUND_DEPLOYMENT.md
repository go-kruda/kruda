# Kruda Playground Deployment

## Architecture

Simple Go Playground API wrapper with rate limiting and security controls.

## Files

```
cmd/playground/
├── main.go              # Main server
├── handlers.go          # Playground handlers
├── middleware.go        # Rate limiting, CORS
└── templates/           # HTML templates
```

## Deployment

### 1. Server Setup (Ubuntu 24.04)

```bash
# Create user
sudo useradd -m -s /bin/bash kruda
sudo usermod -aG docker kruda

# Create directories
sudo mkdir -p /opt/kruda-playground
sudo chown kruda:kruda /opt/kruda-playground

# Clone repository
sudo -u kruda git clone https://github.com/go-kruda/kruda.git /opt/kruda-playground
```

### 2. Build and Install

```bash
cd /opt/kruda-playground
sudo -u kruda go build -o playground ./cmd/playground/
```

### 3. Systemd Service

```ini
# /etc/systemd/system/kruda-playground.service
[Unit]
Description=Kruda Playground
After=network.target

[Service]
Type=simple
User=kruda
Group=kruda
WorkingDirectory=/opt/kruda-playground
ExecStart=/opt/kruda-playground/playground
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/kruda-playground

# Environment
Environment=PORT=8080
Environment=PLAYGROUND_RATE_LIMIT=10
Environment=PLAYGROUND_TIMEOUT=10s

[Install]
WantedBy=multi-user.target
```

### 4. Nginx Configuration

```nginx
# /etc/nginx/sites-available/play.kruda.dev
server {
    listen 80;
    server_name play.kruda.dev;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name play.kruda.dev;

    # SSL configuration (Let's Encrypt)
    ssl_certificate /etc/letsencrypt/live/play.kruda.dev/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/play.kruda.dev/privkey.pem;
    
    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=playground:10m rate=10r/m;
    limit_req zone=playground burst=20 nodelay;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 10s;
        proxy_read_timeout 15s;
    }
}
```

### 5. SSL Certificate

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Get certificate
sudo certbot --nginx -d play.kruda.dev

# Auto-renewal (already configured by certbot)
sudo systemctl status certbot.timer
```

### 6. Deployment Script

```bash
#!/usr/bin/env bash
# deploy-playground.sh
set -euo pipefail

APP_DIR="/opt/kruda-playground"
SERVICE_NAME="kruda-playground"
USER="kruda"

log() { echo -e "\033[0;36m[deploy]\033[0m $*"; }
ok() { echo -e "\033[0;32m[OK]\033[0m $*"; }

log "Deploying Kruda Playground..."

# Pull latest code
cd "$APP_DIR"
sudo -u "$USER" git pull origin main

# Build
log "Building application..."
sudo -u "$USER" go build -ldflags="-s -w" -o playground ./cmd/playground/

# Test build
log "Testing build..."
sudo -u "$USER" timeout 5s ./playground --test || true

# Restart service
log "Restarting service..."
sudo systemctl restart "$SERVICE_NAME"
sleep 2

# Check status
if sudo systemctl is-active --quiet "$SERVICE_NAME"; then
    ok "Service is running"
else
    echo "Service status:"
    sudo systemctl status "$SERVICE_NAME"
    exit 1
fi

# Test endpoint
log "Testing endpoint..."
if curl -sf http://localhost:8080/health >/dev/null; then
    ok "Health check passed"
else
    echo "Health check failed"
    exit 1
fi

ok "Deployment complete"
```

### 7. Monitoring

```bash
# Check logs
sudo journalctl -u kruda-playground -f

# Check status
sudo systemctl status kruda-playground

# Check nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# Check SSL certificate
sudo certbot certificates
```

## Security Features

1. **Rate Limiting**: 10 requests/minute per IP
2. **Request Size Limit**: 64KB max code size
3. **Execution Timeout**: 10 seconds max
4. **CORS**: Restricted to kruda.dev origin
5. **No Network Access**: Sandbox has no internet
6. **Process Isolation**: Each execution in separate process
7. **Resource Limits**: Memory and CPU limits via cgroups

## API Endpoints

- `POST /compile` - Compile and run Go code
- `POST /share` - Share code snippet (returns ID)
- `GET /p/:id` - Load shared code snippet
- `GET /health` - Health check
- `GET /` - Playground UI

## Usage

```bash
# Start service
sudo systemctl start kruda-playground
sudo systemctl enable kruda-playground

# Deploy updates
./deploy-playground.sh

# Check logs
sudo journalctl -u kruda-playground -f
```