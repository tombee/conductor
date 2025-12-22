# Part 7: Deployment

This section covers production deployment of Conductor workflows.

## Server Setup

```bash
# Install
curl -L https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64 \
  -o /usr/local/bin/conductor
chmod +x /usr/local/bin/conductor

# Configure provider
conductor provider add

# Add trigger
conductor triggers add schedule meal-plan \
  --workflow workflow.yaml \
  --cron "0 9 * * 0" \
  --input days=7

# Start controller
conductor controller start
```

## Systemd Service

```bash
sudo tee /etc/systemd/system/conductor.service << 'EOF'
[Unit]
Description=Conductor Controller
After=network.target

[Service]
Type=simple
User=conductor
Environment=ANTHROPIC_API_KEY=...
WorkingDirectory=/opt/conductor
ExecStart=/usr/local/bin/conductor controller start --foreground
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable conductor
sudo systemctl start conductor
```

## Verification

```bash
conductor triggers test meal-plan
conductor controller status
sudo journalctl -u conductor -f
```

Reference: [Deployment](../production/deployment/)
