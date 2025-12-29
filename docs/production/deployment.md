# Deployment

Deploy Conductor to production using the method that best fits your infrastructure.

## Quick Start: exe.dev (Recommended)

Deploy Conductor to [exe.dev](https://exe.dev) lightweight VMs for a low-cost, low-maintenance hosting solution.

**Why exe.dev:** Zero-config deployment, built-in TLS, team sharing, and simple SSH-based management.

```bash
# 1. Create a VM
ssh exe.dev new --name=conductor

# 2. Install Conductor
ssh exe.dev ssh conductor
curl -fsSL https://raw.githubusercontent.com/tombee/conductor/main/deploy/exe.dev/install-conductor.sh | bash
# Save the API key that's displayed!

# 3. Share the port (from local machine)
ssh exe.dev share port conductor 9000

# 4. Connect your local CLI
export CONDUCTOR_HOST=https://<url-from-step-3>
export CONDUCTOR_API_KEY=<your-api-key>
conductor runs list
```

### Webhook Support

To enable GitHub/Slack webhooks, expose the public API on a second port:

```bash
# On the VM, enable public API
ssh exe.dev ssh conductor
cat >> ~/.config/conductor/config.yaml << EOF
controller:
  listen:
    public_api:
      enabled: true
      tcp: :9001
EOF
~/stop-conductor.sh && ~/start-conductor.sh

# From local machine, expose it publicly
ssh exe.dev share port conductor 9001 --name conductor-webhooks
ssh exe.dev share set-public conductor-webhooks
```

Configure webhooks to send to `https://conductor-webhooks-<your-id>.exe.dev/webhooks/github/{workflow-name}`.

**Full Documentation:** See [deploy/exe.dev/](https://github.com/tombee/conductor/tree/main/deploy/exe.dev) for detailed setup, team access, backup, and security practices.

## Alternative Deployment Methods

### Docker

Run Conductor as a container:

```bash
docker run -d \
  --name conductor \
  -p 9000:9000 \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v conductor-data:/data \
  ghcr.io/tombee/conductor:latest
```

**Docker Compose:**

```conductor
version: '3.8'
services:
  conductor:
    image: ghcr.io/tombee/conductor:latest
    ports:
      - "9000:9000"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    volumes:
      - conductor-data:/data
    restart: unless-stopped

volumes:
  conductor-data:
```

### Bare Metal (Linux/macOS)

Install directly on servers:

```bash
# Download and install
curl -LO https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64.tar.gz
tar xzf conductor-linux-amd64.tar.gz
sudo mv conductor conductor /usr/local/bin/
sudo chmod +x /usr/local/bin/conductor /usr/local/bin/conductor

# Create system user and directories
sudo useradd --system --no-create-home --shell /bin/false conductor
sudo mkdir -p /etc/conductor /opt/conductor/data /var/log/conductor
sudo chown -R conductor:conductor /opt/conductor /var/log/conductor

# Configure environment
sudo tee /etc/conductor/env <<EOF
ANTHROPIC_API_KEY=your-api-key-here
LOG_LEVEL=info
EOF
sudo chmod 640 /etc/conductor/env

# Create systemd service
sudo tee /etc/systemd/system/conductor.service <<EOF
[Unit]
Description=Conductor Workflow Engine
After=network.target

[Service]
Type=simple
User=conductor
Group=conductor
EnvironmentFile=/etc/conductor/env
ExecStart=/usr/local/bin/conductor
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
```

## Resource Requirements

### Minimum
- **CPU**: 2 cores
- **Memory**: 4 GB RAM
- **Disk**: 10 GB available
- **Network**: Outbound HTTPS to LLM provider APIs

### Recommended for Production
- **CPU**: 4+ cores
- **Memory**: 8+ GB RAM
- **Disk**: 50 GB (for logs and workflow state)

### Scaling Notes
- Each concurrent workflow uses ~100-200 MB memory
- LLM API calls are the primary latency factor
- CPU usage is minimal except during JSON parsing

## Prerequisites

All deployment methods require:
- Network access to LLM provider APIs (Anthropic, OpenAI, etc.)
- Valid API keys for your chosen LLM providers

## Next Steps

After deployment:
- Set up [monitoring](monitoring.md)
- Review [security hardening](security.md)
- Configure [webhook integrations](../reference/connectors/github.md)
