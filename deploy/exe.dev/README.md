# Deploying Conductor on exe.dev

Deploy Conductor to [exe.dev](https://exe.dev) lightweight VMs for a low-cost, low-maintenance hosting solution.

**Best for:** Individual developers, small teams, demos, and experiments.

## Quick Start

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

## Prerequisites

- [exe.dev](https://exe.dev) account (sign up at exe.dev)
- SSH client (built-in on macOS/Linux; use WSL on Windows)
- (Optional) LLM API key for running workflows

## Detailed Setup

### Step 1: Create VM

Create a new exe.dev VM:

```bash
ssh exe.dev new --name=conductor
```

This creates a lightweight Linux VM with persistent storage.

### Step 2: Install Conductor

SSH into your VM and run the install script:

```bash
ssh exe.dev ssh conductor
```

Then inside the VM:

```bash
curl -fsSL https://raw.githubusercontent.com/tombee/conductor/main/deploy/exe.dev/install-conductor.sh | bash
```

The script will:
1. Download the Conductor binary
2. Generate an API key (displayed prominently - save this!)
3. Configure the daemon for remote access
4. Start the daemon and verify health

**Important:** Save the API key displayed during installation. You'll need it to connect from your local machine.

### Step 3: Expose the Port

From your local machine, tell exe.dev to proxy port 9000:

```bash
ssh exe.dev share port conductor 9000
```

This returns a URL like `https://conductor-abc123.exe.dev` - this is your Conductor endpoint.

### Step 4: Connect Local CLI

Configure your local CLI to use the remote daemon:

```bash
# Set environment variables
export CONDUCTOR_HOST=https://conductor-abc123.exe.dev
export CONDUCTOR_API_KEY=<your-api-key-from-step-2>

# Add to shell profile for persistence
# Use ~/.zshrc on macOS (zsh) or ~/.bashrc on Linux (bash)
cat >> ~/.bashrc << EOF
export CONDUCTOR_HOST=https://conductor-abc123.exe.dev
export CONDUCTOR_API_KEY=<your-api-key>
EOF

# Test the connection
conductor runs list
```

### Step 5: Configure LLM Providers (Optional)

To run workflows that use LLMs, configure provider API keys on the VM:

```bash
ssh exe.dev ssh conductor

# Add to ~/.bashrc
echo 'export ANTHROPIC_API_KEY=sk-ant-...' >> ~/.bashrc
source ~/.bashrc

# Restart daemon to pick up new environment
~/stop-conductor.sh
~/start-conductor.sh
```

## Team Access

### Invite Teammates

```bash
# Invite by email (they need exe.dev accounts)
ssh exe.dev share add conductor teammate@example.com

# Then securely share the CONDUCTOR_API_KEY with them
```

### CI/CD Access

```bash
# Create a share link
ssh exe.dev share add-link conductor

# Use the returned URL + API key in your CI environment
```

### Revoke Access

```bash
# Remove teammate
ssh exe.dev share remove conductor teammate@example.com

# Remove share link
ssh exe.dev share remove-link conductor <token>

# Full revocation: regenerate API key on VM
ssh exe.dev ssh conductor
# Edit ~/.config/conductor/config.yaml with new key
# Restart daemon
```

## Managing the Daemon

### Start/Stop

```bash
ssh exe.dev ssh conductor

# Start daemon
~/start-conductor.sh

# Stop daemon
~/stop-conductor.sh

# View logs
tail -f ~/.local/share/conductor/conductor.log
```

### Check Status

```bash
# From local machine (if CONDUCTOR_HOST is set)
curl -s "$CONDUCTOR_HOST/health"

# From VM
curl -s http://localhost:9000/health
```

## Backup and Restore

### Backup

```bash
# From local machine - backup SQLite database
scp -o ProxyCommand="ssh -W %h:%p exe.dev" \
    conductor:~/.local/share/conductor/conductor.db \
    ./conductor-backup-$(date +%Y%m%d).db
```

### Restore

```bash
# Stop daemon first
ssh exe.dev ssh conductor -c '~/stop-conductor.sh'

# Copy backup to VM
scp -o ProxyCommand="ssh -W %h:%p exe.dev" \
    ./conductor-backup.db \
    conductor:~/.local/share/conductor/conductor.db

# Start daemon
ssh exe.dev ssh conductor -c '~/start-conductor.sh'
```

## Upgrading

```bash
ssh exe.dev ssh conductor

# Stop daemon
~/stop-conductor.sh

# Download new version
export CONDUCTOR_VERSION=v1.2.3  # or 'latest'
curl -fsSL https://github.com/tombee/conductor/releases/${CONDUCTOR_VERSION}/download/conductor-linux-amd64.tar.gz | tar xz
mv conductor conductord ~/.local/bin/

# Start daemon
~/start-conductor.sh
```

## Troubleshooting

### Daemon won't start

```bash
# Check logs
cat ~/.local/share/conductor/conductor.log

# Check if port is in use
ss -tuln | grep 9000

# Check config syntax
cat ~/.config/conductor/config.yaml
```

### Can't connect from local CLI

```bash
# Verify environment variables
echo $CONDUCTOR_HOST
echo $CONDUCTOR_API_KEY

# Test with curl
curl -H "Authorization: Bearer $CONDUCTOR_API_KEY" "$CONDUCTOR_HOST/health"

# Check exe.dev share status
ssh exe.dev share show conductor
```

### Authentication errors (401)

- Verify `CONDUCTOR_API_KEY` matches the key in `~/.config/conductor/config.yaml` on the VM
- Ensure the daemon was restarted after config changes

### VM not accessible

```bash
# List your VMs
ssh exe.dev ls

# Check VM status
ssh exe.dev status conductor
```

## Uninstall

```bash
# Delete the VM (removes all data)
ssh exe.dev rm conductor
```

## Security Notes

- **Never make Conductor publicly accessible** - it has privileged access to LLM APIs, shell execution, and file system
- Always use both exe.dev perimeter security AND Conductor API key authentication
- Rotate API keys periodically
- Revoke share links immediately after CI/CD setup
- Store API keys in a password manager, not in plaintext files

## Configuration Reference

See [examples/config.yaml](examples/config.yaml) for a fully annotated configuration file.

## Related Documentation

- [Conductor Documentation](https://conductor.dev/docs)
- [exe.dev Documentation](https://exe.dev/docs)
- [Docker Compose Deployment](../docker-compose/) - for full infrastructure control
- [Kubernetes (Helm) Deployment](../helm/) - for enterprise/K8s environments
