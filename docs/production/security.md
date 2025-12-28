# Security Guide

This guide covers security best practices for deploying and operating Conductor in production environments.

## Security Overview

Conductor's security model addresses several threat vectors:

- **Credential exposure** - API keys and secrets
- **Code execution** - Shell commands and file operations
- **Network access** - Outbound connections to APIs
- **Data leakage** - Sensitive data in logs and outputs

## Credential Management

### Environment Variables

The simplest method for local development:

```bash
export ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}"
export OPENAI_API_KEY="${OPENAI_API_KEY}"

conductor run workflow.yaml
```

:::caution[Security Risk]
Environment variables are visible in process listings and may be logged. Not recommended for production.
:::


### System Credential Store

Use your operating system's secure credential storage:

#### macOS Keychain

Store credentials in macOS Keychain:

```bash
# Store API key
security add-generic-password \
  -s "conductor-anthropic" \
  -a "conductor" \
  -w "${ANTHROPIC_API_KEY}"

# Retrieve in shell profile
export ANTHROPIC_API_KEY=$(security find-generic-password \
  -s "conductor-anthropic" \
  -a "conductor" \
  -w)
```

Add to `~/.zshrc` or `~/.bashrc`:

```bash
# Automatically load API keys from Keychain
if [[ "$(uname)" == "Darwin" ]]; then
  export ANTHROPIC_API_KEY=$(security find-generic-password \
    -s "conductor-anthropic" -a "conductor" -w 2>/dev/null)
fi
```

#### Linux Secret Service

Use GNOME Keyring or KWallet:

```bash
# Store API key (GNOME Keyring)
secret-tool store \
  --label="conductor-anthropic" \
  service conductor \
  key anthropic

# Retrieve in shell profile
export ANTHROPIC_API_KEY=$(secret-tool lookup \
  service conductor \
  key anthropic)
```

#### Windows Credential Manager

Use PowerShell to store credentials:

```powershell
# Store API key
cmdkey /generic:"conductor-anthropic" /user:"conductor" /pass:"${ANTHROPIC_API_KEY}"

# Retrieve (requires additional scripting)
# Not recommended - use environment variables or vaults instead
```

### Secrets Management Platforms

For production deployments, use dedicated secrets management:

#### HashiCorp Vault

Store and retrieve secrets from Vault:

```bash
# Store secret in Vault
vault kv put secret/conductor/api-keys \
  anthropic="${ANTHROPIC_API_KEY}" \
  openai="${OPENAI_API_KEY}"

# Retrieve in application startup script
export ANTHROPIC_API_KEY=$(vault kv get -field=anthropic secret/conductor/api-keys)
export OPENAI_API_KEY=$(vault kv get -field=openai secret/conductor/api-keys)

# Start Conductor daemon
conductor daemon start
```

Configure Vault authentication:

```bash
# AppRole authentication
vault write auth/approle/role/conductor \
  token_ttl=1h \
  token_max_ttl=4h \
  secret_id_ttl=24h

# Get role ID and secret ID
ROLE_ID=$(vault read -field=role_id auth/approle/role/conductor/role-id)
SECRET_ID=$(vault write -f -field=secret_id auth/approle/role/conductor/secret-id)

# Login and get token
VAULT_TOKEN=$(vault write -field=token auth/approle/login \
  role_id="$ROLE_ID" \
  secret_id="$SECRET_ID")

export VAULT_TOKEN
```

#### AWS Secrets Manager

Retrieve secrets from AWS:

```bash
# Store secret
aws secretsmanager create-secret \
  --name conductor/api-keys \
  --secret-string '{"anthropic":"${ANTHROPIC_API_KEY}","openai":"${OPENAI_API_KEY}"}'

# Retrieve in startup script
SECRET_JSON=$(aws secretsmanager get-secret-value \
  --secret-id conductor/api-keys \
  --query SecretString \
  --output text)

export ANTHROPIC_API_KEY=$(echo $SECRET_JSON | jq -r '.anthropic')
export OPENAI_API_KEY=$(echo $SECRET_JSON | jq -r '.openai')

conductor daemon start
```

#### Kubernetes Secrets

Store secrets in Kubernetes:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: conductor-secrets
  namespace: conductor
type: Opaque
stringData:
  anthropic-api-key: "${ANTHROPIC_API_KEY}"
  openai-api-key: "${OPENAI_API_KEY}"
```

Mount as environment variables:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: conductor
spec:
  template:
    spec:
      containers:
      - name: conductor
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: anthropic-api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: conductor-secrets
              key: openai-api-key
```

Or use external secrets operator:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: conductor-secrets
  namespace: conductor
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: conductor-secrets
    creationPolicy: Owner
  data:
  - secretKey: anthropic-api-key
    remoteRef:
      key: secret/conductor/api-keys
      property: anthropic
  - secretKey: openai-api-key
    remoteRef:
      key: secret/conductor/api-keys
      property: openai
```

### Credential Rotation

Rotate API keys regularly:

1. **Generate new API key** at provider console
2. **Update secret store** with new key
3. **Restart Conductor** to load new key
4. **Revoke old API key** after verification

Automate rotation with scripts:

```bash
#!/bin/bash
# rotate-api-key.sh

set -e

# Generate new key (example - provider-specific)
NEW_KEY=$(generate-new-api-key)

# Update Vault
vault kv put secret/conductor/api-keys \
  anthropic="$NEW_KEY" \
  openai="$(vault kv get -field=openai secret/conductor/api-keys)"

# Restart Conductor
kubectl rollout restart deployment/conductor -n conductor

# Wait for rollout
kubectl rollout status deployment/conductor -n conductor

# Revoke old key (after verification)
# revoke-old-api-key
```

### Workflow Secrets

Never hardcode secrets in workflow files:

```yaml
# ❌ INSECURE - API key in workflow
steps:
  - id: call_api
    type: action
    action: http
    inputs:
      url: "https://api.example.com/data"
      headers:
        Authorization: "Bearer sk-1234567890abcdef"  # Don't do this!
```

Use input variables instead:

```yaml
# ✅ SECURE - API key from input
inputs:
  - name: api_key
    type: string
    required: true

steps:
  - id: call_api
    type: action
    action: http
    inputs:
      url: "https://api.example.com/data"
      headers:
        Authorization: "Bearer {{.api_key}}"
```

Provide secrets at runtime:

```bash
# From environment variable
conductor run workflow.yaml --input api_key="${API_KEY}"

# From stdin (most secure)
echo "${API_KEY}" | conductor run workflow.yaml --input-stdin api_key
```

## Security Controls

Conductor provides security controls through allowlists, command validation, and security profiles.

**Important:** Conductor does not provide process-level isolation. Users who need container isolation should run Conductor itself in a containerized environment. See [Docker Deployment Guide](./deployment/docker.md) for details.

### Security Profiles

Conductor supports multiple security profiles:

#### Unrestricted Profile

Default profile with no restrictions:

```yaml
# config.yaml
security:
  profile: unrestricted
```

**Use cases:**
- Local development
- Trusted environments
- Full control required

**Risks:**
- Full filesystem access
- Unrestricted network access
- No command restrictions

#### Standard Profile

Balanced security for production:

```yaml
# config.yaml
security:
  profile: standard
  allowed_tools:
    - file.read
    - file.write
    - shell.exec
    - http.request
  filesystem:
    allowed_paths:
      - /opt/conductor/workflows
      - /tmp
    denied_paths:
      - /etc
      - /root
      - ~/.ssh
  network:
    allowed_domains:
      - api.anthropic.com
      - api.openai.com
      - "*.github.com"
    denied_ips:
      - 169.254.169.254  # AWS metadata
      - 10.0.0.0/8       # Private networks
```

**Use cases:**
- Production deployments
- Multi-tenant environments
- Standard workflows

For additional isolation, run Conductor in a container. See [Docker Deployment Guide](./deployment/docker.md).

### Filesystem Restrictions

Limit file access:

```yaml
# config.yaml
security:
  filesystem:
    # Read-only mode
    read_only: false

    # Allowed paths
    allowed_paths:
      - /opt/conductor/workflows
      - /opt/conductor/data
      - /tmp

    # Denied paths (takes precedence)
    denied_paths:
      - /etc/passwd
      - /etc/shadow
      - ~/.ssh
      - /root

    # Maximum file size for reads/writes
    max_file_size: 100MB
```

### Network Restrictions

Control network access:

```yaml
# config.yaml
security:
  network:
    # Allowed domains (supports wildcards)
    allowed_domains:
      - api.anthropic.com
      - api.openai.com
      - "*.github.com"
      - "*.slack.com"

    # Denied IPs/ranges
    denied_ips:
      - 169.254.169.254      # AWS metadata
      - fd00:ec2::254        # AWS metadata IPv6
      - 127.0.0.0/8          # Loopback
      - 10.0.0.0/8           # Private
      - 172.16.0.0/12        # Private
      - 192.168.0.0/16       # Private

    # Deny access to cloud metadata services
    deny_cloud_metadata: true

    # Deny all private IPs
    deny_private_ips: true
```

### Command Restrictions

Limit shell commands:

```yaml
# config.yaml
security:
  shell:
    # Allowed commands
    allowed_commands:
      - /bin/ls
      - /bin/cat
      - /usr/bin/python3
      - /usr/bin/node

    # Denied patterns (regex)
    denied_patterns:
      - "rm -rf"
      - "dd if="
      - ":(){ :|:& };:"  # Fork bomb

    # Maximum execution time
    timeout: 300s
```

## Webhook Security

Secure webhook endpoints against unauthorized access.

### Webhook Secrets

Verify webhook signatures:

```yaml
# config.yaml
webhooks:
  - path: /webhooks/github
    workflow: workflows/pr-review.yaml
    secret: "${GITHUB_WEBHOOK_SECRET}"
    signature_header: X-Hub-Signature-256
    signature_algorithm: sha256
```

Configure in GitHub:

1. Go to repository **Settings** > **Webhooks**
2. Add webhook URL: `https://conductor.example.com/webhooks/github`
3. Set **Secret**: Use the same value as in config
4. Select events: Pull requests, Issues, etc.

Conductor validates signatures automatically:

```
X-Hub-Signature-256: sha256=<hmac-signature>
```

### IP Allowlisting

Restrict webhook sources:

```yaml
# config.yaml
webhooks:
  - path: /webhooks/github
    workflow: workflows/pr-review.yaml
    secret: "${GITHUB_WEBHOOK_SECRET}"
    allowed_ips:
      - 140.82.112.0/20    # GitHub hooks
      - 143.55.64.0/20     # GitHub hooks
      - 192.30.252.0/22    # GitHub hooks
```

Or use reverse proxy (Nginx):

```nginx
location /webhooks/github {
    # GitHub webhook IPs
    allow 140.82.112.0/20;
    allow 143.55.64.0/20;
    allow 192.30.252.0/22;
    deny all;

    proxy_pass http://localhost:8080;
}
```

### Rate Limiting

Prevent abuse:

```yaml
# config.yaml
webhooks:
  - path: /webhooks/github
    workflow: workflows/pr-review.yaml
    rate_limit:
      requests_per_minute: 60
      burst: 10
```

Or configure in Nginx:

```nginx
# Define rate limit zone
limit_req_zone $binary_remote_addr zone=webhook_limit:10m rate=60r/m;

location /webhooks/ {
    limit_req zone=webhook_limit burst=10 nodelay;
    proxy_pass http://localhost:8080;
}
```

### TLS/SSL

Always use HTTPS for webhooks:

```nginx
server {
    listen 443 ssl http2;
    server_name conductor.example.com;

    # TLS configuration
    ssl_certificate /etc/ssl/certs/conductor.crt;
    ssl_certificate_key /etc/ssl/private/conductor.key;

    # Modern TLS only
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # HSTS
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    location /webhooks/ {
        proxy_pass http://localhost:8080;
    }
}
```

### Request Validation

Validate webhook payloads:

```yaml
# config.yaml
webhooks:
  - path: /webhooks/github
    workflow: workflows/pr-review.yaml
    validation:
      # Maximum payload size
      max_body_size: 10MB

      # Required headers
      required_headers:
        - X-GitHub-Event
        - X-Hub-Signature-256

      # Content-Type validation
      allowed_content_types:
        - application/json
```

### Audit Logging

Log all webhook requests:

```yaml
# config.yaml
webhooks:
  audit_log:
    enabled: true
    destination: /var/log/conductor/webhooks.log
    format: json
    fields:
      - timestamp
      - source_ip
      - path
      - headers
      - signature_valid
      - workflow_triggered
```

## Network Security

### Firewall Configuration

Restrict network access:

```bash
# Allow only necessary ports
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 8080/tcp comment 'Conductor HTTP'
sudo ufw allow 443/tcp comment 'HTTPS'
sudo ufw enable
```

For production, block Conductor port and use reverse proxy:

```bash
# Conductor only accessible from localhost
sudo ufw deny 8080/tcp
sudo ufw allow from 127.0.0.1 to any port 8080

# HTTPS through Nginx
sudo ufw allow 443/tcp
```

### TLS for LLM APIs

Verify TLS certificates:

```yaml
# config.yaml
llm:
  providers:
    - name: anthropic
      verify_tls: true
      ca_cert_path: /etc/ssl/certs/ca-certificates.crt
```

### Proxy Configuration

Route traffic through proxy:

```bash
export HTTP_PROXY="http://proxy.company.com:8080"
export HTTPS_PROXY="http://proxy.company.com:8080"
export NO_PROXY="localhost,127.0.0.1"

conductor daemon start
```

## Audit Logging

Track security-relevant events:

```yaml
# config.yaml
audit:
  enabled: true
  destinations:
    - type: file
      path: /var/log/conductor/audit.log
      format: json
    - type: syslog
      address: syslog.company.com:514
      protocol: udp
    - type: webhook
      url: https://siem.company.com/events
      headers:
        Authorization: "Bearer ${SIEM_TOKEN}"

  events:
    - workflow_started
    - workflow_completed
    - workflow_failed
    - credential_access
    - file_access
    - shell_command
    - network_request
    - security_violation
```

Audit log format:

```json
{
  "timestamp": "2025-12-24T10:00:00Z",
  "event": "workflow_started",
  "workflow": "pr-review",
  "user": "github-webhook",
  "source_ip": "140.82.112.10",
  "metadata": {
    "repository": "tombee/conductor",
    "pr_number": 42
  }
}
```

## Data Security

### Sensitive Data in Logs

Prevent credential leakage:

```yaml
# config.yaml
logging:
  redact_patterns:
    - "sk-ant-[a-zA-Z0-9]+"      # Anthropic keys
    - "sk-[a-zA-Z0-9]{48}"        # OpenAI keys
    - "ghp_[a-zA-Z0-9]{36}"       # GitHub tokens
    - "xoxb-[0-9]+-[a-zA-Z0-9]+"  # Slack tokens

  redact_fields:
    - password
    - api_key
    - secret
    - token
    - authorization
```

### Workflow Outputs

Sanitize sensitive outputs:

```yaml
steps:
  - id: get_secret
    type: action
    action: file.read
    inputs:
      path: /secrets/api-key.txt

  - id: use_secret
    type: llm
    inputs:
      model: fast
      prompt: "Do something with the API key"

outputs:
  # ❌ Don't expose secrets
  - name: secret
    value: "{{$.get_secret.content}}"

  # ✅ Only expose non-sensitive data
  - name: result
    value: "{{$.use_secret.content}}"
```

## Production Hardening Checklist

Use this checklist before deploying to production:

### Credentials

- [ ] API keys stored in secrets manager (Vault, AWS Secrets Manager, etc.)
- [ ] No hardcoded secrets in workflows or configs
- [ ] Credential rotation schedule established
- [ ] Secrets access audited and logged

### Security Configuration

- [ ] Security profile configured (unrestricted or standard)
- [ ] Filesystem access restricted to necessary paths
- [ ] Network access limited to required domains
- [ ] For isolation needs, Conductor running in container (see [Docker Guide](./deployment/docker.md))

### Webhooks

- [ ] Webhook secrets configured and validated
- [ ] IP allowlisting enabled for known sources
- [ ] Rate limiting configured
- [ ] TLS/HTTPS enforced
- [ ] Request validation enabled

### Network

- [ ] Firewall rules restrict access to Conductor port
- [ ] Reverse proxy (Nginx/Apache) configured with TLS
- [ ] TLS certificate valid and renewed automatically
- [ ] Cloud metadata endpoints blocked

### Logging & Monitoring

- [ ] Audit logging enabled
- [ ] Logs forwarded to SIEM or log aggregation
- [ ] Sensitive data redacted from logs
- [ ] Security alerts configured
- [ ] Log retention policy defined

### Access Control

- [ ] Conductor runs as non-root user
- [ ] File permissions restrict access to configs and data
- [ ] Systemd security hardening enabled
- [ ] Admin access requires authentication

### Updates & Maintenance

- [ ] Update process documented and tested
- [ ] Security patches applied promptly
- [ ] Backup and recovery procedures tested
- [ ] Incident response plan defined

### Compliance

- [ ] Data residency requirements met
- [ ] Encryption at rest enabled (if required)
- [ ] Audit trail meets compliance standards
- [ ] Security controls documented

## Security Incidents

### Responding to Compromise

If you suspect a security incident:

1. **Isolate** - Stop Conductor service immediately
2. **Assess** - Check audit logs for unauthorized access
3. **Contain** - Revoke compromised API keys
4. **Investigate** - Review logs, file access, network connections
5. **Remediate** - Update secrets, patch vulnerabilities
6. **Monitor** - Watch for suspicious activity post-recovery

### Revoking Compromised Keys

```bash
# 1. Generate new API keys at provider console

# 2. Update secrets manager
vault kv put secret/conductor/api-keys \
  anthropic="<NEW_KEY>" \
  openai="<NEW_KEY>"

# 3. Restart Conductor to load new keys
systemctl restart conductor

# 4. Revoke old keys at provider console

# 5. Audit logs for unauthorized usage
journalctl -u conductor --since "1 hour ago" | grep ERROR
```

## Additional Resources

- [Agent Security Model](../design/agent-security-model.md) - Architecture details
- [Deployment Guide](deployment.md) - Production deployment
- [Monitoring Guide](monitoring.md) - Security monitoring
- [Troubleshooting](troubleshooting.md) - Common issues

## Security Contact

Report security vulnerabilities:

- Email: security@example.com
- PGP Key: https://example.com/pgp-key.asc
- Bug Bounty: https://example.com/security/bounty
