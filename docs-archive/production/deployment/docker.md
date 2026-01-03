# Docker Deployment Guide

This guide covers deploying Conductor in Docker containers with security hardening and best practices.

**Important:** Conductor does not provide process-level isolation without running in a container. Users who need isolation must run Conductor itself in a container environment.

## Quick Start

### Basic Docker Run

```bash
docker run -d \
  --name conductor \
  -v /path/to/workspace:/workspace \
  -v /path/to/config:/config \
  -e CONDUCTOR_CONFIG=/config/conductor.yaml \
  conductor:latest
```

### Security-Hardened Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  conductor:
    image: conductor:latest
    container_name: conductor
    restart: unless-stopped

    # Security hardening
    security_opt:
      - no-new-privileges=true
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Only if binding to ports <1024
    read_only: true

    # Temporary filesystem for writable areas
    tmpfs:
      - /tmp
      - /var/tmp

    # Resource limits
    mem_limit: 2g
    memswap_limit: 2g
    cpus: 2
    pids_limit: 100

    # Volumes
    volumes:
      - ./workspace:/workspace
      - ./config:/config:ro
      - conductor-data:/data

    # Environment
    environment:
      - CONDUCTOR_CONFIG=/config/conductor.yaml
      - CONDUCTOR_LOG_LEVEL=info

    # Network
    networks:
      - conductor-net

volumes:
  conductor-data:

networks:
  conductor-net:
    driver: bridge
```

Start the service:

```bash
docker-compose up -d
```

## Security Hardening

### Read-Only Filesystem

Running with a read-only root filesystem prevents container escapes and runtime modifications:

```yaml
read_only: true
tmpfs:
  - /tmp
  - /var/tmp
```

### Drop All Capabilities

Remove all Linux capabilities and only add back what's strictly necessary:

```yaml
cap_drop:
  - ALL
cap_add:
  - NET_BIND_SERVICE  # Only if needed
```

### No New Privileges

Prevent privilege escalation:

```yaml
security_opt:
  - no-new-privileges=true
```

### Additional Security Options

For AppArmor:
```yaml
security_opt:
  - apparmor=docker-default
```

For SELinux:
```yaml
security_opt:
  - label:type:container_runtime_t
```

## Network Isolation

### Host Network (Not Recommended for Production)

```yaml
network_mode: host
```

### Bridge Network (Default)

```yaml
networks:
  - conductor-net
```

### No Network (Air-Gapped)

For completely offline deployments:

```yaml
network_mode: none
```

### Custom Bridge Network

```yaml
networks:
  conductor-net:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

## Volume Mounting Best Practices

### Workspace Volume

Mount your workflow workspace:

```yaml
volumes:
  - ./workspace:/workspace
```

### Configuration (Read-Only)

Always mount config files as read-only:

```yaml
volumes:
  - ./config:/config:ro
```

### Data Persistence

Use named volumes for persistent data:

```yaml
volumes:
  - conductor-data:/data

volumes:
  conductor-data:
    driver: local
```

### Avoiding Sensitive Directories

Never mount these as writable:
- `/etc`
- `/usr`
- `/bin`
- `/sbin`
- `~/.ssh`
- `~/.aws`
- `~/.config/conductor/credentials`

## Resource Limits

### Memory Limits

```yaml
mem_limit: 2g        # Hard limit
memswap_limit: 2g    # Memory + swap limit
mem_reservation: 1g  # Soft limit
```

### CPU Limits

```yaml
cpus: 2              # Number of CPUs
cpu_shares: 1024     # Relative CPU weight
```

### Process Limits

```yaml
pids_limit: 100      # Maximum number of processes
```

### File Descriptor Limits

```yaml
ulimits:
  nofile:
    soft: 1024
    hard: 2048
```

## Secrets Management

### Docker Secrets (Swarm)

```yaml
version: '3.8'

services:
  conductor:
    secrets:
      - anthropic_key
      - openai_key

secrets:
  anthropic_key:
    file: ./secrets/anthropic.key
  openai_key:
    file: ./secrets/openai.key
```

Access in container:
```bash
# Secrets mounted at /run/secrets/
ANTHROPIC_API_KEY=$(cat /run/secrets/anthropic_key)
```

### Environment Variables

For non-Swarm deployments:

```yaml
environment:
  - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
```

Store in `.env` file (never commit to git):
```bash
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
```

### External Secret Stores

For production, use:
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Google Secret Manager

## Deployment Scenarios

### Development

```yaml
version: '3.8'

services:
  conductor:
    image: conductor:latest
    volumes:
      - ./workspace:/workspace
      - ./config:/config
    environment:
      - CONDUCTOR_LOG_LEVEL=debug
      - CONDUCTOR_SECURITY_PROFILE=unrestricted
```

### Production

```yaml
version: '3.8'

services:
  conductor:
    image: conductor:v1.0.0  # Use specific version
    restart: always
    security_opt:
      - no-new-privileges=true
    cap_drop:
      - ALL
    read_only: true
    tmpfs:
      - /tmp
    mem_limit: 2g
    cpus: 2
    volumes:
      - ./config:/config:ro
      - conductor-data:/data
    environment:
      - CONDUCTOR_SECURITY_PROFILE=standard
    networks:
      - conductor-net
```

### Air-Gapped

For completely offline environments:

```yaml
version: '3.8'

services:
  conductor:
    image: conductor:latest
    network_mode: none  # No network access
    security_opt:
      - no-new-privileges=true
    cap_drop:
      - ALL
    read_only: true
    volumes:
      - ./input:/input:ro
      - ./output:/output
      - ./config:/config:ro
```

## Multi-Container Setup

### With Reverse Proxy

```yaml
version: '3.8'

services:
  nginx:
    image: nginx:alpine
    ports:
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    networks:
      - conductor-net
    depends_on:
      - conductor

  conductor:
    image: conductor:latest
    security_opt:
      - no-new-privileges=true
    cap_drop:
      - ALL
    read_only: true
    networks:
      - conductor-net

networks:
  conductor-net:
    internal: true  # No external access
```

### With Monitoring

```yaml
version: '3.8'

services:
  conductor:
    image: conductor:latest
    # ... security config ...
    networks:
      - conductor-net

  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
    networks:
      - conductor-net

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    networks:
      - conductor-net

networks:
  conductor-net:
```

## Troubleshooting

### Permission Denied Errors

If you encounter permission issues with volumes:

```bash
# Check ownership
ls -la /path/to/workspace

# Fix permissions
chown -R $(id -u):$(id -g) /path/to/workspace
```

Or use user mapping:
```yaml
user: "${UID}:${GID}"
```

### Read-Only Filesystem Issues

If application needs write access:

```yaml
tmpfs:
  - /tmp
  - /var/tmp
  - /app/cache  # Application-specific writable area
```

### Network Connectivity

Test network from container:

```bash
docker exec conductor ping -c 3 api.anthropic.com
```

### Resource Exhaustion

Monitor resource usage:

```bash
docker stats conductor
```

Adjust limits if needed.

### Container Crashes

View logs:

```bash
docker logs conductor
docker logs --tail 100 conductor
docker logs -f conductor  # Follow
```

## Health Checks

Add health checks to ensure container is running correctly:

```yaml
healthcheck:
  test: ["CMD", "conductor", "health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

## Best Practices Summary

1. **Always use specific image tags**, not `latest` in production
2. **Enable read-only filesystem** with tmpfs for writable areas
3. **Drop all capabilities** and only add necessary ones
4. **Set resource limits** to prevent resource exhaustion
5. **Use secrets management** for sensitive data
6. **Mount config files read-only** (`ro` flag)
7. **Enable health checks** for automatic restart on failure
8. **Use bridge networks**, avoid host networking
9. **Set restart policies** (`unless-stopped` or `always`)
10. **Run as non-root user** when possible

## What Isolation Conductor Does NOT Provide

Conductor itself does not provide:
- Container namespace isolation (PID, IPC, Network)
- Filesystem isolation via bind mounts
- Seccomp/AppArmor enforcement at the tool execution level
- Resource limit enforcement via cgroups for individual tools

All process isolation must be configured at the container runtime level (Docker, Podman, Kubernetes). Conductor provides:
- Allowlist-based tool restrictions (SecurityInterceptor)
- Command injection prevention
- Path traversal protection
- Network request validation

For isolation guarantees, run Conductor in a container with appropriate security settings as described in this guide.
