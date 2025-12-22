# Deployment Guide

This guide covers deploying Conductor in production environments using Docker, Kubernetes, and bare metal installations.

## Deployment Options

Choose the deployment method that best fits your infrastructure:

- **Docker** - Simple containerized deployment for single-node setups
- **Kubernetes** - Scalable orchestration for multi-node production environments
- **Bare Metal** - Direct installation on servers for maximum control

## Prerequisites

All deployment methods require:

- Linux, macOS, or Windows host
- Network access to LLM provider APIs (Anthropic, OpenAI, etc.)
- Valid API keys for LLM providers
- Sufficient resources (see [Resource Requirements](#resource-requirements))

## Resource Requirements

### Minimum Requirements

- **CPU**: 2 cores
- **Memory**: 4 GB RAM
- **Disk**: 10 GB available space
- **Network**: Outbound HTTPS to LLM provider APIs

### Recommended for Production

- **CPU**: 4+ cores
- **Memory**: 8+ GB RAM
- **Disk**: 50 GB available space (for logs and workflow state)
- **Network**: Low-latency connection to LLM providers

### Scaling Considerations

- Each concurrent workflow execution consumes ~100-200 MB memory
- LLM API calls are the primary latency factor
- CPU usage is minimal except during JSON parsing and template rendering

## Docker Deployment

### Quick Start

Run Conductor as a Docker container:

```bash
docker run -d \
  --name conductor \
  -p 8080:8080 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v /path/to/workflows:/workflows \
  -v conductor-data:/data \
  conductor:latest
```

### Building the Image

If building from source:

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o conductor ./cmd/conductor

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/conductor /usr/local/bin/conductor

EXPOSE 8080
ENTRYPOINT ["conductor"]
CMD ["serve", "--port", "8080"]
```

Build and run:

```bash
docker build -t conductor:latest .

docker run -d \
  --name conductor \
  -p 8080:8080 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  conductor:latest
```

### Docker Compose

For multi-container setups with dependencies:

```yaml
# docker-compose.yml
version: '3.8'

services:
  conductor:
    image: conductor:latest
    container_name: conductor
    ports:
      - "8080:8080"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - LOG_LEVEL=info
      - CONDUCTOR_DATA_DIR=/data
    volumes:
      - ./workflows:/workflows:ro
      - conductor-data:/data
      - conductor-logs:/var/log/conductor
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "conductor", "health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  conductor-data:
  conductor-logs:
```

Start the stack:

```bash
docker-compose up -d
```

### Persistent Storage

Mount volumes for persistent data:

```bash
docker run -d \
  --name conductor \
  -p 8080:8080 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v /opt/conductor/workflows:/workflows:ro \
  -v /opt/conductor/data:/data \
  -v /var/log/conductor:/var/log/conductor \
  conductor:latest
```

**Volume purposes:**

- `/workflows` - Workflow YAML files (read-only)
- `/data` - Workflow state and execution history
- `/var/log/conductor` - Application logs

### Environment Variables

Configure the container using environment variables:

```bash
docker run -d \
  --name conductor \
  -p 8080:8080 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
  -e LOG_LEVEL=info \
  -e CONDUCTOR_DATA_DIR=/data \
  -e CONDUCTOR_WEBHOOK_SECRET="${WEBHOOK_SECRET}" \
  conductor:latest
```

**Key environment variables:**

- `ANTHROPIC_API_KEY` - Anthropic API key (required if using Claude)
- `OPENAI_API_KEY` - OpenAI API key (required if using GPT models)
- `LOG_LEVEL` - Logging level (debug, info, warn, error)
- `CONDUCTOR_DATA_DIR` - Data directory path
- `CONDUCTOR_WEBHOOK_SECRET` - Secret for webhook validation

## Kubernetes Deployment

### Namespace Setup

Create a dedicated namespace:

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: conductor
```

Apply:

```bash
kubectl apply -f namespace.yaml
```

### Secret Management

Store API keys as Kubernetes secrets:

```bash
kubectl create secret generic conductor-secrets \
  --from-literal=anthropic-api-key="${ANTHROPIC_API_KEY}" \
  --from-literal=openai-api-key="${OPENAI_API_KEY}" \
  --from-literal=webhook-secret="${WEBHOOK_SECRET}" \
  --namespace conductor
```

Or using a YAML manifest:

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: conductor-secrets
  namespace: conductor
type: Opaque
stringData:
  anthropic-api-key: "${ANTHROPIC_API_KEY}"
  openai-api-key: "${OPENAI_API_KEY}"
  webhook-secret: "${WEBHOOK_SECRET}"
```

:::caution[Secret Security]
Never commit secrets to version control. Use placeholder values in YAML files and inject secrets at deployment time using sealed secrets, external secrets operators, or CI/CD pipelines.
:::


Apply:

```bash
# Replace placeholders before applying
envsubst < secrets.yaml | kubectl apply -f -
```

### ConfigMap for Workflows

Store workflow definitions in a ConfigMap:

```yaml
# workflows-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: conductor-workflows
  namespace: conductor
data:
  hello-world.yaml: |
    name: hello-world
    description: Simple greeting workflow
    inputs:
      - name: user
        type: string
        required: true
    steps:
      - id: greet
        type: llm
        inputs:
          model: fast
          prompt: "Say hello to {{.user}}"
    outputs:
      - name: greeting
        value: "{{$.greet.content}}"
```

Apply:

```bash
kubectl apply -f workflows-configmap.yaml
```

### Deployment

Create the Conductor deployment:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: conductor
  namespace: conductor
  labels:
    app: conductor
spec:
  replicas: 2
  selector:
    matchLabels:
      app: conductor
  template:
    metadata:
      labels:
        app: conductor
    spec:
      containers:
      - name: conductor
        image: conductor:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: anthropic-api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: openai-api-key
        - name: CONDUCTOR_WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: webhook-secret
        - name: LOG_LEVEL
          value: "info"
        - name: CONDUCTOR_DATA_DIR
          value: "/data"
        volumeMounts:
        - name: workflows
          mountPath: /workflows
          readOnly: true
        - name: data
          mountPath: /data
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: workflows
        configMap:
          name: conductor-workflows
      - name: data
        persistentVolumeClaim:
          claimName: conductor-data
```

### Persistent Volume Claim

Create storage for workflow state:

```yaml
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: conductor-data
  namespace: conductor
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
  storageClassName: standard
```

Apply:

```bash
kubectl apply -f pvc.yaml
```

### Service

Expose Conductor within the cluster:

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: conductor
  namespace: conductor
  labels:
    app: conductor
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: conductor
```

Apply:

```bash
kubectl apply -f service.yaml
```

### Ingress (Optional)

Expose Conductor externally:

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: conductor
  namespace: conductor
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - conductor.example.com
    secretName: conductor-tls
  rules:
  - host: conductor.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: conductor
            port:
              number: 8080
```

Apply:

```bash
kubectl apply -f ingress.yaml
```

### Deploy All Components

Deploy everything in order:

```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f workflows-configmap.yaml
kubectl apply -f pvc.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
```

Verify deployment:

```bash
# Check pod status
kubectl get pods -n conductor

# View logs
kubectl logs -n conductor -l app=conductor --tail=100 -f

# Test health endpoint
kubectl port-forward -n conductor svc/conductor 8080:8080
curl http://localhost:8080/health
```

### Horizontal Pod Autoscaling

Scale based on CPU usage:

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: conductor
  namespace: conductor
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: conductor
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

Apply:

```bash
kubectl apply -f hpa.yaml
```

## Bare Metal Deployment

### System Requirements

Supported operating systems:

- Linux (Ubuntu 20.04+, RHEL 8+, Debian 11+)
- macOS (11.0+)
- Windows Server 2019+

### Installation Steps

#### 1. Install Binary

Download and install the latest release:

```bash
# Download latest release
wget https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64

# Make executable
chmod +x conductor-linux-amd64

# Move to PATH
sudo mv conductor-linux-amd64 /usr/local/bin/conductor

# Verify installation
conductor --version
```

Or install from source:

```bash
# Install Go 1.21+
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Clone and build
git clone https://github.com/tombee/conductor.git
cd conductor
go build -o conductor ./cmd/conductor
sudo mv conductor /usr/local/bin/
```

#### 2. Create System User

Run Conductor as a dedicated user:

```bash
sudo useradd --system --no-create-home --shell /bin/false conductor
```

#### 3. Create Directories

Set up data and workflow directories:

```bash
sudo mkdir -p /opt/conductor/workflows
sudo mkdir -p /opt/conductor/data
sudo mkdir -p /var/log/conductor
sudo chown -R conductor:conductor /opt/conductor
sudo chown -R conductor:conductor /var/log/conductor
```

#### 4. Configure Environment

Store API keys securely:

```bash
# Create environment file
sudo tee /etc/conductor/env <<EOF
ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
OPENAI_API_KEY=${OPENAI_API_KEY}
CONDUCTOR_DATA_DIR=/opt/conductor/data
LOG_LEVEL=info
EOF

# Secure the file
sudo chown root:conductor /etc/conductor/env
sudo chmod 640 /etc/conductor/env
```

#### 5. Create Systemd Service

Set up automatic startup:

```ini
# /etc/systemd/system/conductor.service
[Unit]
Description=Conductor Workflow Engine
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=conductor
Group=conductor
EnvironmentFile=/etc/conductor/env
ExecStart=/usr/local/bin/conductor serve --port 8080
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

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
sudo systemctl status conductor
```

#### 6. Configure Log Rotation

Prevent logs from consuming disk space:

```bash
# /etc/logrotate.d/conductor
/var/log/conductor/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 conductor conductor
    sharedscripts
    postrotate
        systemctl reload conductor > /dev/null 2>&1 || true
    endscript
}
```

Test log rotation:

```bash
sudo logrotate -f /etc/logrotate.d/conductor
```

### Reverse Proxy Setup

#### Nginx

Configure Nginx as a reverse proxy:

```nginx
# /etc/nginx/sites-available/conductor
server {
    listen 80;
    server_name conductor.example.com;

    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name conductor.example.com;

    ssl_certificate /etc/ssl/certs/conductor.crt;
    ssl_certificate_key /etc/ssl/private/conductor.key;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Proxy settings
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Webhook endpoint
    location /webhooks/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Limit request size for webhooks
        client_max_body_size 10M;
    }
}
```

Enable and test:

```bash
sudo ln -s /etc/nginx/sites-available/conductor /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

#### Apache

Configure Apache as a reverse proxy:

```apache
# /etc/apache2/sites-available/conductor.conf
<VirtualHost *:80>
    ServerName conductor.example.com
    Redirect permanent / https://conductor.example.com/
</VirtualHost>

<VirtualHost *:443>
    ServerName conductor.example.com

    SSLEngine on
    SSLCertificateFile /etc/ssl/certs/conductor.crt
    SSLCertificateKeyFile /etc/ssl/private/conductor.key

    ProxyPreserveHost On
    ProxyPass / http://localhost:8080/
    ProxyPassReverse / http://localhost:8080/

    # WebSocket support
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://localhost:8080/$1" [P,L]

    ErrorLog ${APACHE_LOG_DIR}/conductor-error.log
    CustomLog ${APACHE_LOG_DIR}/conductor-access.log combined
</VirtualHost>
```

Enable modules and site:

```bash
sudo a2enmod ssl proxy proxy_http rewrite
sudo a2ensite conductor
sudo apache2ctl configtest
sudo systemctl reload apache2
```

### Health Checks

Verify Conductor is running:

```bash
# Check service status
sudo systemctl status conductor

# Test health endpoint
curl http://localhost:8080/health

# View logs
sudo journalctl -u conductor -f
```

### Updates

Update Conductor to a new version:

```bash
# Stop service
sudo systemctl stop conductor

# Backup current binary
sudo cp /usr/local/bin/conductor /usr/local/bin/conductor.backup

# Download new version
wget https://github.com/tombee/conductor/releases/download/v1.2.0/conductor-linux-amd64

# Replace binary
sudo mv conductor-linux-amd64 /usr/local/bin/conductor
sudo chmod +x /usr/local/bin/conductor

# Start service
sudo systemctl start conductor

# Verify
conductor --version
sudo systemctl status conductor
```

## High Availability

### Load Balancing

Use a load balancer for redundancy:

```nginx
# Nginx load balancer config
upstream conductor_backend {
    least_conn;
    server conductor1.internal:8080 max_fails=3 fail_timeout=30s;
    server conductor2.internal:8080 max_fails=3 fail_timeout=30s;
    server conductor3.internal:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    server_name conductor.example.com;

    ssl_certificate /etc/ssl/certs/conductor.crt;
    ssl_certificate_key /etc/ssl/private/conductor.key;

    location / {
        proxy_pass http://conductor_backend;
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        proxy_connect_timeout 5s;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### State Management

For multi-node deployments, use shared storage:

- **NFS** - Network file system for workflow state
- **S3-compatible storage** - Object storage for artifacts
- **Distributed filesystem** - GlusterFS or Ceph for high availability

## Monitoring

See [Monitoring Guide](monitoring.md) for detailed monitoring setup including:

- Prometheus metrics collection
- Grafana dashboards
- Alert rules
- Log aggregation

## Security

See [Security Guide](security.md) for production hardening including:

- Credential management
- Network security
- Sandboxing configuration
- Webhook authentication

## Troubleshooting

If deployment fails, check:

1. **Network connectivity** - Can the host reach LLM provider APIs?
2. **API keys** - Are credentials configured correctly?
3. **Ports** - Is port 8080 available and not blocked by firewall?
4. **Resources** - Does the host have sufficient CPU and memory?
5. **Logs** - Check service logs for error messages

See [Troubleshooting Guide](troubleshooting.md) for detailed diagnostics.

## Next Steps

After deployment:

- Configure [monitoring](monitoring.md) and alerting
- Review [security hardening](security.md) checklist
- Set up [webhook integrations](../reference/connectors/index.md)
- Create your first production workflow
