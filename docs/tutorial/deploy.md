# Part 7: Deploy to Production

Run your meal planner on a server for 24/7 automation.

## Quick Setup

```bash
# 1. Install Conductor on server
ssh user@your-server.example.com
curl -L https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64 -o /usr/local/bin/conductor
chmod +x /usr/local/bin/conductor

# 2. Configure provider
conductor provider add

# 3. Copy workflow files
mkdir -p ~/workflows
# (copy 06-complete.yaml and pantry.txt)

# 4. Add trigger
cd ~/workflows
conductor triggers add schedule meal-plan \
  --workflow 06-complete.yaml \
  --cron "0 9 * * 0" \
  --input days=7 \
  --input pantry_file=pantry.txt \
  --input webhook_url="https://hooks.slack.com/your/webhook"

# 5. Start controller
conductor controller start
```

## Systemd Service (Recommended)

For automatic startup on reboot:

```bash
sudo tee /etc/systemd/system/conductor.service << 'EOF'
[Unit]
Description=Conductor Workflow Controller
After=network.target

[Service]
Type=simple
User=your-username
Environment=ANTHROPIC_API_KEY=your-key-here
WorkingDirectory=/home/your-username/workflows
ExecStart=/usr/local/bin/conductor controller start --foreground
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
```

## Verify

```bash
# Test trigger now
conductor triggers test meal-plan

# Check status
conductor controller status
conductor history

# View logs
sudo journalctl -u conductor -f
```

See [Deployment](/production/deployment/) for Docker, Kubernetes, and other options.

## Tutorial Complete!

You've built an automated meal planning system that:
- Reads pantry inventory
- Generates personalized plans
- Refines through AI critique
- Delivers to Slack
- Runs weekly on schedule

**Next:** Explore [Examples](/examples/) and [Patterns](/building-workflows/patterns/).
