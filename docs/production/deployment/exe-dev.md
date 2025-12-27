# exe.dev Deployment

Deploy Conductor to [exe.dev](https://exe.dev) lightweight VMs for a low-cost, low-maintenance hosting solution.

**Best for:** Individual developers and small teams looking for low-cost, low-maintenance hosting.

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

## Webhook Support

To enable GitHub/Slack webhooks, expose the public API on a second port:

```bash
# On the VM, enable public API
ssh exe.dev ssh conductor
cat >> ~/.config/conductor/config.yaml << EOF
daemon:
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

Now configure GitHub webhooks to send to `https://conductor-webhooks-<your-id>.exe.dev/webhooks/github/{workflow-name}`.

## Full Documentation

See the complete deployment guide at [deploy/exe.dev/](https://github.com/tombee/conductor/tree/main/deploy/exe.dev) which covers:

- Detailed setup instructions
- Webhook configuration with two-port deployment
- Team access management
- Backup and restore procedures
- Upgrading
- Troubleshooting
- Security best practices

## Security Model

exe.dev deployment uses defense-in-depth with a two-plane architecture:

**Control Plane (Port 9000 - Private)**
1. **exe.dev perimeter** - Only invited users can access
2. **Conductor API key** - Required for all management API requests

**Public API (Port 9001 - Optional, Public)**
1. **Per-workflow secrets** - Each webhook/trigger has its own credential
2. **Signature verification** - GitHub/Slack signatures validated
3. **No management APIs** - Only workflow triggers exposed

Both layers are enabled by default. Never disable API key authentication on the control plane.
