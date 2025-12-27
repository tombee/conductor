# Deployment

Deploy Conductor to production using the method that best fits your infrastructure.

## Choose Your Platform

| Platform | Best For | Complexity |
|----------|----------|------------|
| [exe.dev](exe-dev.md) | Individuals, small teams | Simple |
| [Docker](docker.md) | Single-node, containerized environments | Simple |
| [Kubernetes](kubernetes.md) | Multi-node, scalable production | Moderate |
| [Bare Metal](bare-metal.md) | Maximum control, existing servers | Moderate |

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

- Set up [monitoring](../operations/monitoring.md)
- Review [security hardening](../operations/security.md)
- Configure [webhook integrations](../reference/connectors/github.md)
