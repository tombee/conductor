# Monitoring

Monitor Conductor workflows and daemon health in production.

## Health Endpoint

Check daemon health:

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
- Daemon health check failures
- High error rates

## Best Practices

**1. Enable structured logging:**
```conductor
logging:
  format: json
```

**2. Monitor success rates:**
Track `conductor_workflows_total{status="failed"}` vs `status="success"`

**3. Track token costs:**
Monitor `conductor_llm_tokens_total` to control spending

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
