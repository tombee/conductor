# Bare Metal Deployment

Install Conductor directly on Linux servers for maximum control.

## Supported Systems

- Linux (Ubuntu 20.04+, RHEL 8+, Debian 11+)
- macOS (11.0+)

## Installation

### Download Binary

```bash
# Download latest release
curl -LO https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64.tar.gz
tar xzf conductor-linux-amd64.tar.gz

# Install
sudo mv conductor conductord /usr/local/bin/
sudo chmod +x /usr/local/bin/conductor /usr/local/bin/conductord

# Verify
conductor --version
```

### Create System User

```bash
sudo useradd --system --no-create-home --shell /bin/false conductor
```

### Create Directories

```bash
sudo mkdir -p /etc/conductor
sudo mkdir -p /opt/conductor/data
sudo mkdir -p /var/log/conductor
sudo chown -R conductor:conductor /opt/conductor
sudo chown -R conductor:conductor /var/log/conductor
```

### Configure Environment

```bash
sudo tee /etc/conductor/env <<EOF
ANTHROPIC_API_KEY=your-api-key-here
LOG_LEVEL=info
EOF

sudo chown root:conductor /etc/conductor/env
sudo chmod 640 /etc/conductor/env
```

### Systemd Service

```ini
# /etc/systemd/system/conductor.service
[Unit]
Description=Conductor Workflow Engine
After=network.target

[Service]
Type=simple
User=conductor
Group=conductor
EnvironmentFile=/etc/conductor/env
ExecStart=/usr/local/bin/conductord
Restart=on-failure
RestartSec=5s
StandardOutput=append:/var/log/conductor/stdout.log
StandardError=append:/var/log/conductor/stderr.log

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/conductor/data /var/log/conductor

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
sudo systemctl status conductor
```

## Log Rotation

```bash
# /etc/logrotate.d/conductor
/var/log/conductor/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 conductor conductor
}
```

## Reverse Proxy (Nginx)

```nginx
# /etc/nginx/sites-available/conductor
server {
    listen 443 ssl http2;
    server_name conductor.example.com;

    ssl_certificate /etc/ssl/certs/conductor.crt;
    ssl_certificate_key /etc/ssl/private/conductor.key;

    location / {
        proxy_pass http://localhost:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

```bash
sudo ln -s /etc/nginx/sites-available/conductor /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## Updates

```bash
sudo systemctl stop conductor

# Backup
sudo cp /usr/local/bin/conductor /usr/local/bin/conductor.backup

# Download and install new version
curl -LO https://github.com/tombee/conductor/releases/download/v1.2.0/conductor-linux-amd64.tar.gz
tar xzf conductor-linux-amd64.tar.gz
sudo mv conductor conductord /usr/local/bin/

sudo systemctl start conductor
```

## Health Check

```bash
sudo systemctl status conductor
curl http://localhost:9000/health
sudo journalctl -u conductor -f
```
