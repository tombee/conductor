# Runbooks

This guide provides procedures for operators to diagnose and resolve common integration issues.

## Runbook: Integration Health Check

Test connectivity and auth for all configured integrations.

```bash
# Test a specific integration
conductor integration test github

# Expected output:
# ✓ github: Connected (api.github.com, 45ms)
# ✓ Rate limit: 4,832/5,000 remaining
# ✓ Auth: Valid (expires: never)
```

**Troubleshooting failed health checks:**

1. **Connection timeout**: Check network connectivity, DNS resolution, and firewall rules
2. **401/403**: Verify token is set and hasn't expired
3. **404**: Confirm base_url is correct for the service instance

## Runbook: Rate Limit Recovery

**Symptoms**: Workflows failing with `rate_limited` errors

**Diagnosis:**

```bash
# Check current rate limit status
conductor integration status --rate-limits

# Output shows:
# github: 12/100 requests remaining this minute
# github: Last wait: 2.3s
# github: Total waits: 47
```

**Recovery Steps:**

1. **Identify high-frequency callers**
   ```bash
   # Check logs for patterns
   conductor logs --filter "connector=github" --last 1h
   ```

2. **Temporary mitigation**
   - Wait for rate limit window to reset (1 minute for per-minute limits)
   - Reduce workflow execution frequency
   - Add delays between workflow runs

3. **Long-term fix**
   Update integration config to reduce request rate:
   ```yaml
   integrations:
     github:
       from: integrations/github
       rate_limit:
         requests_per_second: 5   # Reduced from 10
         requests_per_minute: 50  # Reduced from 100
   ```

4. **For batch operations**
   Consider request coalescing or pagination to reduce total requests.

## Runbook: Credential Rotation

**When**: Rotating API tokens for security compliance or after suspected compromise

**Steps:**

1. **Generate new token** in external service (GitHub, Slack, etc.)

2. **Update environment variable** or secret store
   ```bash
   # Update environment variable
   export GITHUB_TOKEN=ghp_new_token_here

   # Or update in secret manager
   aws secretsmanager update-secret --secret-id conductor/github-token --secret-string ghp_new_token
   ```

3. **Reload configuration**
   ```bash
   # For controller mode
   conductor reload

   # Or restart daemon
   conductor daemon restart
   ```

4. **Verify connectivity**
   ```bash
   conductor integration test github
   ```

5. **Revoke old token** in external service (after confirming new token works)

**Rollback**: If new token fails, revert environment variable to old value and reload.

## Runbook: Debugging Failed Integration Steps

**Symptoms**: Integration step returns error in workflow execution

**Diagnosis Steps:**

1. **Check error type** in workflow output
   ```
   Error: integration step failed
   Type: auth_error | rate_limited | not_found | server_error
   Message: Detailed error message
   ```

2. **Enable debug logging**
   ```bash
   conductor run --log-level debug workflow.yaml
   ```

   Look for:
   - Request URL and headers (auth masked)
   - Response status code
   - Response body
   - Rate limit wait times

3. **Validate request payload**
   ```bash
   conductor run --dry-run workflow.yaml
   ```

   Verify:
   - Path parameters are correct
   - Required inputs are provided
   - Values match expected types

4. **Test operation in isolation**
   ```bash
   conductor integration invoke github.create_issue \
     --inputs '{"owner":"my-org","repo":"my-repo","title":"test"}'
   ```

**Resolution by Error Type:**

| Error Type | Likely Cause | Fix |
|------------|--------------|-----|
| `auth_error` | Invalid or expired token | Rotate credentials (see above) |
| `not_found` | Wrong resource ID or URL | Verify inputs, check base_url |
| `validation_error` | Invalid inputs | Check request_schema requirements |
| `rate_limited` | Too many requests | See rate limit recovery above |
| `server_error` | External service issue | Check service status page, retry later |
| `timeout` | Slow response or network issue | Increase timeout, check network |
| `connection_error` | Cannot reach service | Check DNS, firewall, base_url |

## Runbook: SSRF Protection Override

**Scenario**: Need to access internal API but SSRF protection blocks it

**Warning**: Only override SSRF protection if you understand the security implications.

**Steps:**

1. **Verify the request is legitimate**
   - Confirm internal API is intentional
   - Review security policy

2. **Add to allowed hosts**
   ```yaml
   security:
     allowed_hosts:
       - api.internal.corp
       - "*.internal.corp"  # Wildcard for subdomains
   ```

3. **Test connectivity**
   ```bash
   conductor integration test internal_api
   ```

4. **Document the override** in your security runbook

**Security Checklist:**
- [ ] Internal API requires authentication
- [ ] API is not accessible from outside network
- [ ] Workflow inputs are validated to prevent injection
- [ ] Override documented in security policy

## Runbook: Performance Degradation

**Symptoms**: Integration steps taking longer than usual

**Diagnosis:**

1. **Check metrics**
   ```bash
   # View integration metrics
   curl http://localhost:9090/metrics | grep conductor_connector

   # Look for:
   # conductor_connector_request_duration_seconds_sum
   # conductor_connector_rate_limit_waits_total
   ```

2. **Identify slow operations**
   ```bash
   conductor logs --filter "connector" --format json | \
     jq '.duration' | \
     sort -n | \
     tail -20
   ```

3. **Check external service status**
   - GitHub: [githubstatus.com](https://www.githubstatus.com/)
   - Slack: [status.slack.com](https://status.slack.com/)
   - Check your service's status page

**Resolution:**

1. **For rate limit waits**: Reduce request frequency
2. **For slow API responses**: 
   - Increase timeout if responses are legitimately slow
   - Check network path (proxy, VPN)
   - Contact API provider if persistent
3. **For large responses**: Add response_transform to reduce data size

## Runbook: Integration Configuration Validation

**Before deploying** new integration configurations:

1. **Validate workflow syntax**
   ```bash
   conductor validate workflow.yaml
   ```

2. **Test in dry-run mode**
   ```bash
   conductor run --dry-run workflow.yaml
   ```

3. **Test with minimal inputs**
   ```bash
   conductor integration invoke my_api.health_check
   ```

4. **Monitor first production run**
   ```bash
   conductor run workflow.yaml --log-level debug
   ```

5. **Check metrics after rollout**
   ```bash
   # Monitor error rates
   curl http://localhost:9090/metrics | grep conductor_connector_requests_total
   ```

## Monitoring Dashboards

### Key Metrics to Monitor

**Request Rate:**
- `conductor_connector_requests_total{connector="github",status="200"}`
- Alert if error rate (status 4xx/5xx) > 5%

**Duration:**
- `conductor_connector_request_duration_seconds{connector="github"}`
- Alert if p95 > 5 seconds

**Rate Limit Waits:**
- `conductor_connector_rate_limit_waits_total{connector="github"}`
- Alert if wait count increasing rapidly

**Sample Prometheus Queries:**

```promql
# Error rate by integration
rate(conductor_connector_requests_total{status=~"4..|5.."}[5m]) /
rate(conductor_connector_requests_total[5m])

# P95 request duration
histogram_quantile(0.95, conductor_connector_request_duration_seconds)

# Rate limit wait rate
rate(conductor_connector_rate_limit_waits_total[5m])
```

## Emergency Procedures

### Circuit Breaker: Disable Failing Integration

If a integration is causing cascading failures:

1. **Identify affected workflows**
   ```bash
   grep -r "connector: failing_connector" workflows/
   ```

2. **Temporary disable** (if supported by your deployment)
   ```yaml
   # In workflow, add condition to skip
   - id: use_connector
     type: integration
     integration: failing_connector.operation
     condition: "false"  # Temporarily disabled
   ```

3. **Monitor impact** on dependent workflows

4. **Root cause analysis** using debug logs and metrics

5. **Re-enable gradually** after fix confirmed

## See Also

- [Connector Configuration](./custom.md)
- [Monitoring](../operations/monitoring.md)
- [Security](../operations/security.md)
