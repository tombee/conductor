# Monitoring

Monitor Conductor workflows and controller health in production.

## Health Endpoint

Check controller health:

```bash
curl http://localhost:9000/health
```

**Response:**
```json
{
  "status": "healthy",
  "uptime": "2h15m30s",
  "workflows": 5,
  "active_runs": 2
}
```

## Metrics

Conductor exposes Prometheus-compatible metrics:

```bash
curl http://localhost:9000/metrics
```

**Key metrics:**
- `conductor_workflows_total` — Workflow execution count by status
- `conductor_workflow_duration_seconds` — Execution duration histogram
- `conductor_llm_requests_total` — LLM API calls by provider/model
- `conductor_llm_tokens_total` — Token usage by provider

**Memory metrics:**
- `conductor_sse_subscribers` — Active SSE subscribers across all runs
- `conductor_log_aggregator_runs` — Number of runID keys in subscriber map
- `conductor_goroutines` — Active goroutine count
- `conductor_runs_in_memory` — Runs cached in memory
- `conductor_heap_bytes` — Current heap allocation in bytes

## Logs

### Structured Logging

Enable JSON logging for log aggregation:

```conductor
# config.yaml
logging:
  level: info
  format: json
  output: /var/log/conductor/conductor.log
```

### Log Levels

- `debug` — Detailed execution information
- `info` — Standard operation events
- `warn` — Non-critical issues
- `error` — Errors requiring attention

### Viewing Logs

**Systemd:**
```bash
sudo journalctl -u conductor -f
```

**Docker:**
```bash
docker logs -f conductor
```

**File:**
```bash
tail -f /var/log/conductor/conductor.log
```

## Correlation IDs

Conductor assigns a unique correlation ID to each workflow run for distributed tracing.

### Format

Correlation IDs are UUID v4 format:

```
550e8400-e29b-41d4-a716-446655440000
```

Generated at workflow start, the same ID is used across all steps, LLM calls, and integration requests.

### Propagation

Correlation IDs flow through:

| Location | How |
|----------|-----|
| **HTTP headers** | `X-Correlation-ID` header on outbound requests |
| **Log entries** | `correlation_id` field in structured logs |
| **LLM provider calls** | Passed to provider for request tracking |
| **Run records** | Stored with workflow run data |

### Finding Logs by Correlation ID

**grep:**
```bash
grep "550e8400-e29b-41d4-a716-446655440000" /var/log/conductor/conductor.log
```

**jq (JSON logs):**
```bash
cat conductor.log | jq 'select(.correlation_id == "550e8400-e29b-41d4-a716-446655440000")'
```

**Elasticsearch:**
```json
{
  "query": {
    "term": {
      "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

**CloudWatch Insights:**
```
fields @timestamp, @message
| filter correlation_id = "550e8400-e29b-41d4-a716-446655440000"
| sort @timestamp asc
```

### Accessing in Workflows

Access the current correlation ID in templates:

```conductor
steps:
  - id: log_context
    shell.run:
      command: ["echo", "Processing with ID: {{.run.correlation_id}}"]
```

### Cross-System Debugging Runbook

When debugging issues across systems:

**Step 1: Get the workflow run ID**

```bash
conductor history list --status failed
```

**Step 2: Get the correlation ID**

```bash
conductor history show <run-id> | grep correlation_id
```

**Step 3: Search Conductor logs**

```bash
grep "<correlation-id>" /var/log/conductor/conductor.log
```

**Step 4: Search LLM provider logs**

Use the correlation ID to find matching requests in your LLM provider's dashboard or logs. The `X-Correlation-ID` header is sent with each request.

### Privacy Considerations

!!! note "Data linkability"
    Correlation IDs enable linking logs across systems, which may have privacy implications:

    - **GDPR/HIPAA:** Correlation IDs are not PII themselves, but they link to data that may be PII. Include them in data retention policies.
    - **Third parties:** If logs are shared with external services, consider whether correlation IDs should be redacted.
    - **Retention alignment:** Ensure correlation ID retention matches your data retention policies.

## Common Monitoring Patterns

### Prometheus Integration

Scrape metrics for time-series analysis:

```conductor
# prometheus.yml
scrape_configs:
  - job_name: 'conductor'
    static_configs:
      - targets: ['localhost:9000']
```

### Grafana Dashboard

Create dashboards tracking:
- Workflow success/failure rates
- Execution durations (p50, p95, p99)
- LLM token consumption
- Error rates by type

### Alerting

Set up alerts for:
- High failure rates (>5% of workflows)
- Long execution times (p95 > threshold)
- Controller health check failures
- High error rates

### Memory Alerts

Monitor memory metrics to detect leaks and resource exhaustion:

**Recommended alert thresholds:**

| Metric | Warning | Critical | Notes |
|--------|---------|----------|-------|
| `conductor_goroutines` | >baseline+100 | >baseline+500 | Baseline varies by load; measure during normal operation |
| `conductor_heap_bytes` | >15% growth/hour | >50% growth/hour | Indicates potential memory leak |
| `conductor_runs_in_memory` | >1000 | >5000 | May need to reduce retention period |
| `conductor_sse_subscribers` | >100 | >500 | May indicate clients not disconnecting properly |
| `conductor_log_aggregator_runs` | >1000 | >5000 | Should match runs_in_memory; higher value indicates subscriber leak |

**Example Prometheus alerting rules:**

```yaml
# prometheus-alerts.yml
groups:
  - name: conductor_memory
    interval: 1m
    rules:
      - alert: ConductorGoroutineLeakWarning
        expr: conductor_goroutines > (conductor_goroutines offset 1h) + 100
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Conductor goroutine count increasing"
          description: "Goroutines have increased by >100 in the last hour (current: {{ $value }})"

      - alert: ConductorHeapGrowthWarning
        expr: |
          (conductor_heap_bytes - (conductor_heap_bytes offset 1h))
          / (conductor_heap_bytes offset 1h) > 0.15
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Conductor heap growing rapidly"
          description: "Heap size increased by >15% in the last hour (current: {{ $value | humanize1024 }})"

      - alert: ConductorSubscriberLeak
        expr: conductor_log_aggregator_runs > conductor_runs_in_memory * 1.2
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Log aggregator subscriber leak detected"
          description: "Subscriber map has more entries ({{ $value }}) than active runs, indicating a memory leak"
```

**Troubleshooting high memory usage:**

1. Check goroutine count:
   ```bash
   curl http://localhost:9000/metrics | grep conductor_goroutines
   ```

2. Get heap profile:
   ```bash
   curl http://localhost:9000/debug/pprof/heap > heap.prof
   go tool pprof heap.prof
   ```

3. Check run retention:
   ```yaml
   # config.yaml
   controller:
     run_retention: 12h  # Reduce from default 24h if needed
   ```

4. Verify subscriber cleanup:
   ```bash
   curl http://localhost:9000/metrics | grep conductor_log_aggregator_runs
   # Should be close to conductor_runs_in_memory
   ```

## Best Practices

**1. Enable structured logging:**
```conductor
logging:
  format: json
```

**2. Monitor success rates:**
Track `conductor_workflows_total{status="failed"}` vs `status="success"`

**3. Track token costs:**
Monitor `conductor_llm_tokens_total` to control spending. See [Cost Tracking](cost-tracking.md) for budgets and alerts.

**4. Set up health checks:**
Ping `/health` endpoint regularly

**5. Alert on anomalies:**
Use metrics to detect unusual patterns

## Further Reading

For advanced monitoring setups:
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Dashboards](https://grafana.com/docs/)
- [OpenTelemetry (future support)](https://opentelemetry.io/)

Monitoring capabilities will expand in future releases.
