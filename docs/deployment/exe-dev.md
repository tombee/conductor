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

## Full Documentation

See the complete deployment guide at [deploy/exe.dev/](https://github.com/tombee/conductor/tree/main/deploy/exe.dev) which covers:

- Detailed setup instructions
- Team access management
- Backup and restore procedures
- Upgrading
- Troubleshooting
- Security best practices

## Security Model

exe.dev deployment uses defense-in-depth:

1. **exe.dev perimeter** - Only invited users can access your VM's shared ports
2. **Conductor API key** - Required for all API requests even with network access

Both layers are enabled by default. Never disable API key authentication.
