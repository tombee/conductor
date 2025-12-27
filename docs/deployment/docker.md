# Docker Deployment

Run Conductor as a Docker container for simple, single-node deployments.

## Quick Start

```bash
docker run -d \
  --name conductor \
  -p 9000:9000 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v conductor-data:/data \
  ghcr.io/tombee/conductor:latest
```

## Building from Source

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o conductor ./cmd/conductor
RUN CGO_ENABLED=0 go build -o conductord ./cmd/conductord

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/conductor /usr/local/bin/
COPY --from=builder /build/conductord /usr/local/bin/

EXPOSE 9000
ENTRYPOINT ["conductord"]
```

Build and run:

```bash
docker build -t conductor:latest .

docker run -d \
  --name conductor \
  -p 9000:9000 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  conductor:latest
```

## Docker Compose

For multi-container setups:

```yaml
# docker-compose.yml
version: '3.8'

services:
  conductor:
    image: ghcr.io/tombee/conductor:latest
    container_name: conductor
    ports:
      - "9000:9000"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - LOG_LEVEL=info
    volumes:
      - ./workflows:/workflows:ro
      - conductor-data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9000/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  conductor-data:
```

Start:

```bash
docker-compose up -d
```

## Volumes

Mount volumes for persistent data:

| Path | Purpose |
|------|---------|
| `/workflows` | Workflow YAML files (read-only) |
| `/data` | Workflow state and execution history |

```bash
docker run -d \
  --name conductor \
  -p 9000:9000 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v /opt/conductor/workflows:/workflows:ro \
  -v /opt/conductor/data:/data \
  conductor:latest
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `LOG_LEVEL` | debug, info, warn, error |

## Health Check

```bash
curl http://localhost:9000/health
```
