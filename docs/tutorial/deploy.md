# Part 7: Deploy to Production

Run your meal planner on a server so it works automatically, even when your computer is off.

## What You'll Learn

- Deploying Conductor to a server
- Configuring credentials securely
- Setting up always-on automation

## Why Deploy?

Running on your laptop has limitations:
- Laptop must be on and awake for scheduled triggers
- VPN/network changes can disrupt execution
- Not accessible to other family members

A server deployment runs 24/7, executes on schedule, and can deliver to shared destinations.

## Deployment Options

| Platform | Best For | Cost |
|----------|----------|------|
| Home server/NAS | Free, privacy-focused | Hardware you own |
| Raspberry Pi | Low-power, always-on | ~$50-100 one-time |
| VPS (Hetzner, DigitalOcean) | Reliable, accessible | ~$5-10/month |
| exe.dev | Simplest setup | Subscription |

We'll show a generic Linux server deployment that works on any platform.

## Step 1: Install Conductor on Server

SSH into your server:

```bash
ssh user@your-server.example.com
```

Install Conductor:

```bash
# Option A: Go install (if Go is available)
go install github.com/tombee/conductor/cmd/conductor@latest

# Option B: Download binary
curl -L https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64 -o /usr/local/bin/conductor
chmod +x /usr/local/bin/conductor
```

Verify:
```bash
conductor --version
```

## Step 2: Configure LLM Provider

Set up your LLM provider credentials securely:

```bash
# Interactive setup (recommended)
conductor provider add

# Or set environment variables
export ANTHROPIC_API_KEY="your-key-here"
```

For production, add to your shell profile or use a secrets manager:

```bash
# Add to ~/.bashrc or ~/.profile
echo 'export ANTHROPIC_API_KEY="your-key-here"' >> ~/.bashrc
source ~/.bashrc
```

## Step 3: Copy Your Workflow

Transfer your workflow and pantry file to the server:

```bash
# From your local machine
scp examples/tutorial/06-complete.yaml user@server:~/workflows/
scp examples/tutorial/pantry.txt user@server:~/workflows/
```

Or create them directly on the server:

```bash
mkdir -p ~/workflows
# Then create the files...
```

## Step 4: Add the Schedule Trigger

On the server, configure the weekly trigger:

```bash
cd ~/workflows

conductor triggers add schedule meal-plan \
  --workflow 06-complete.yaml \
  --cron "0 9 * * 0" \
  --input days=7 \
  --input pantry_file=pantry.txt \
  --input webhook_url="https://hooks.slack.com/your/webhook"
```

Verify:
```bash
conductor triggers list
```

## Step 5: Start the Controller

### Development (Foreground)

For testing:
```bash
conductor controller start --foreground
```

### Production (Systemd Service)

Create a systemd service for automatic startup:

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
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable conductor
sudo systemctl start conductor
```

Check status:
```bash
sudo systemctl status conductor
```

## Step 6: Verify It Works

Test the trigger immediately:
```bash
conductor triggers test meal-plan
```

Check logs:
```bash
sudo journalctl -u conductor -f
```

## Updating Your Workflow

When you want to change the workflow:

1. Edit the workflow file
2. Restart the controller (or it auto-reloads on file change)

```bash
# Edit workflow
nano ~/workflows/06-complete.yaml

# Restart controller
sudo systemctl restart conductor
```

## Updating Your Pantry

Your family can update the pantry file directly on the server, or you can sync it from a shared location:

```bash
# Sync from Dropbox, Google Drive, etc.
rclone sync dropbox:family/pantry.txt ~/workflows/pantry.txt
```

Or set up a simple web form that updates the file (advanced topic).

## Monitoring

### Check Run History

```bash
conductor history
```

### View Controller Status

```bash
conductor controller status
```

### Trigger Logs

```bash
conductor triggers show meal-plan
```

## Troubleshooting

**Controller won't start**
```bash
# Check if port is in use
sudo lsof -i :9000

# Check logs
sudo journalctl -u conductor -n 50
```

**Trigger not firing**
```bash
# Verify trigger is registered
conductor triggers list

# Check system time (cron depends on correct time)
date
timedatectl
```

**API key issues**
```bash
# Verify environment variable is set
echo $ANTHROPIC_API_KEY

# Test directly
conductor run 06-complete.yaml -i days=1
```

## You Did It!

You've built a complete automated meal planning system that:

1. Reads your pantry inventory
2. Generates personalized meal plans
3. Refines quality through AI critique
4. Delivers to your preferred destination
5. Runs automatically every week

## What You've Learned

Throughout this tutorial, you've mastered:

| Part | Concept | Pattern |
|------|---------|---------|
| 1 | LLM steps, file output | Read-Process-Write |
| 2 | Parallel execution | Fan-Out/Fan-In |
| 3 | Refinement loops | Critique-Improve |
| 4 | Scheduled triggers | Scheduled Automation |
| 5 | File input | Context Injection |
| 6 | HTTP delivery | Multi-Channel Delivery |
| 7 | Production deployment | Always-On Automation |

## Next Steps

Now that you understand Conductor's core patterns, explore:

- **[Building Workflows](/conductor/building-workflows/patterns)** — More advanced patterns
- **[Examples](/conductor/examples)** — Production-ready workflows
- **[Reference](/conductor/reference/workflow-schema)** — Complete specification
- **[Integrations](/conductor/reference/integrations)** — GitHub, Slack, Jira, and more

Happy automating!
