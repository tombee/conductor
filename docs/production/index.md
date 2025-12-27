# Production

Deploy and operate Conductor in production environments.

---

## Overview

This section covers everything needed to run Conductor reliably in production:

- **Deployment** strategies for different platforms
- **Security** best practices and configurations
- **Monitoring** and observability
- **Troubleshooting** common issues
- **Operations** guides for day-to-day management

---

## Deployment

Choose the deployment option that fits your infrastructure:

### [Deployment Overview](deployment/)
Compare deployment modes and platforms.

### Platform-Specific Guides

- **[Docker](deployment/docker.md)** — Container-based deployment
- **[Kubernetes](deployment/kubernetes.md)** — Production-grade orchestration
- **[Bare Metal](deployment/bare-metal.md)** — Direct server installation
- **[exe.dev](deployment/exe-dev.md)** — Quick cloud deployment

---

## Operations

### [Security](security.md)
Secure your Conductor deployment with proper authentication, secrets management, and access controls.

### [Monitoring](monitoring.md)
Track workflow execution, performance metrics, and system health.

### [Startup Configuration](startup.md)
Configure Conductor for production environments with proper logging, resource limits, and fault tolerance.

### [Troubleshooting](troubleshooting.md)
Diagnose and resolve common production issues.

---

## Best Practices

### Security Checklist

- Use secrets management (not hardcoded API keys)
- Enable authentication for daemon mode
- Restrict network access
- Regular security updates
- Audit workflow execution logs

### Reliability Checklist

- Implement health checks
- Configure resource limits
- Set up monitoring and alerting
- Plan for graceful degradation
- Test disaster recovery procedures

### Performance Checklist

- Use appropriate model tiers
- Leverage parallel execution
- Implement caching where appropriate
- Monitor API rate limits
- Optimize workflow design

---

## Quick Start

1. **Choose your platform:** Review [deployment options](deployment/)
2. **Deploy Conductor:** Follow the platform-specific guide
3. **Configure security:** Set up authentication and secrets
4. **Enable monitoring:** Configure metrics and logging
5. **Test workflows:** Validate in production-like environment
6. **Go live:** Deploy with confidence

---

## Additional Resources

- **[Configuration Reference](../reference/configuration.md)** — All configuration options
- **[Daemon Mode Guide](../building-workflows/daemon-mode.md)** — Run as a service
- **[Error Codes Reference](../reference/error-codes.md)** — Troubleshooting guide
