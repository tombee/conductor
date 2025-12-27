# Monitoring and Observability

This guide covers monitoring Conductor in production, including metrics collection, dashboards, alerting, and log aggregation.

## Overview

Effective monitoring requires:

- **Metrics** - Quantitative measurements (request rate, latency, errors)
- **Logs** - Event records with context
- **Traces** - Request flow through the system (future)
- **Alerts** - Notifications when thresholds are exceeded

## Metrics Collection

### Built-in Metrics Endpoint

Conductor exposes Prometheus-compatible metrics:

```bash
curl http://localhost:8080/metrics
```

Example output:

```
# HELP conductor_workflows_total Total number of workflow executions
# TYPE conductor_workflows_total counter
conductor_workflows_total{status="success"} 142
conductor_workflows_total{status="failed"} 8

# HELP conductor_workflow_duration_seconds Workflow execution duration
# TYPE conductor_workflow_duration_seconds histogram
conductor_workflow_duration_seconds_bucket{le="1"} 45
conductor_workflow_duration_seconds_bucket{le="5"} 98
conductor_workflow_duration_seconds_bucket{le="10"} 132
conductor_workflow_duration_seconds_bucket{le="+Inf"} 150
conductor_workflow_duration_seconds_sum 487.3
conductor_workflow_duration_seconds_count 150

# HELP conductor_llm_requests_total Total LLM API requests
# TYPE conductor_llm_requests_total counter
conductor_llm_requests_total{provider="anthropic",model="claude-3-5-sonnet"} 324
conductor_llm_requests_total{provider="openai",model="gpt-4"} 56

# HELP conductor_llm_tokens_total Total tokens consumed
# TYPE conductor_llm_tokens_total counter
conductor_llm_tokens_total{provider="anthropic",type="input"} 145920
conductor_llm_tokens_total{provider="anthropic",type="output"} 32104

# HELP conductor_llm_cost_total Total cost in USD
# TYPE conductor_llm_cost_total counter
conductor_llm_cost_total{provider="anthropic"} 12.34
conductor_llm_cost_total{provider="openai"} 8.76

# HELP conductor_http_requests_total Total HTTP requests
# TYPE conductor_http_requests_total counter
conductor_http_requests_total{method="GET",path="/health",status="200"} 1543
conductor_http_requests_total{method="POST",path="/webhooks/github",status="200"} 42

# HELP conductor_http_request_duration_seconds HTTP request duration
# TYPE conductor_http_request_duration_seconds histogram
conductor_http_request_duration_seconds_bucket{le="0.005"} 1200
conductor_http_request_duration_seconds_bucket{le="0.01"} 1450
conductor_http_request_duration_seconds_bucket{le="0.025"} 1520
conductor_http_request_duration_seconds_bucket{le="+Inf"} 1585
conductor_http_request_duration_seconds_sum 8.42
conductor_http_request_duration_seconds_count 1585
```

### Key Metrics

#### Workflow Metrics

- `conductor_workflows_total` - Total workflows executed (counter)
- `conductor_workflows_active` - Currently running workflows (gauge)
- `conductor_workflow_duration_seconds` - Execution time (histogram)
- `conductor_workflow_steps_total` - Total steps executed (counter)
- `conductor_workflow_errors_total` - Workflow failures (counter)

#### LLM Metrics

- `conductor_llm_requests_total` - API requests by provider/model (counter)
- `conductor_llm_tokens_total` - Token usage by type (input/output) (counter)
- `conductor_llm_cost_total` - Estimated cost in USD (counter)
- `conductor_llm_latency_seconds` - API response time (histogram)
- `conductor_llm_errors_total` - API errors (counter)

#### System Metrics

- `conductor_http_requests_total` - HTTP requests (counter)
- `conductor_http_request_duration_seconds` - Request latency (histogram)
- `conductor_memory_bytes` - Memory usage (gauge)
- `conductor_goroutines` - Active goroutines (gauge)

## Prometheus Setup

### Installation

Install Prometheus:

```bash
# Docker
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v /path/to/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus

# Or download binary
wget https://github.com/prometheus/prometheus/releases/download/v2.45.0/prometheus-2.45.0.linux-amd64.tar.gz
tar xvfz prometheus-2.45.0.linux-amd64.tar.gz
cd prometheus-2.45.0.linux-amd64
./prometheus --config.file=prometheus.yml
```

### Configuration

Configure Prometheus to scrape Conductor:

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'conductor'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

For Kubernetes:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'conductor'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - conductor
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: conductor
      - source_labels: [__meta_kubernetes_pod_ip]
        target_label: __address__
        replacement: '${1}:8080'
```

### Verify Scraping

Check Prometheus is collecting metrics:

```bash
# Access Prometheus UI
open http://localhost:9090

# Query metrics
curl 'http://localhost:9090/api/v1/query?query=conductor_workflows_total'
```

## Grafana Dashboards

### Installation

Install Grafana:

```bash
# Docker
docker run -d \
  --name grafana \
  -p 3000:3000 \
  -e "GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}" \
  grafana/grafana

# Or download binary
wget https://dl.grafana.com/oss/release/grafana-10.0.0.linux-amd64.tar.gz
tar -zxvf grafana-10.0.0.linux-amd64.tar.gz
cd grafana-10.0.0
./bin/grafana-server
```

Access Grafana at http://localhost:3000 (default: admin/admin)

### Add Prometheus Data Source

1. Navigate to **Configuration** > **Data Sources**
2. Click **Add data source**
3. Select **Prometheus**
4. Set URL: `http://localhost:9090`
5. Click **Save & Test**

### Sample Dashboard

Create a dashboard with these panels:

#### Workflow Success Rate

```promql
# Success rate (last 5 minutes)
sum(rate(conductor_workflows_total{status="success"}[5m]))
/
sum(rate(conductor_workflows_total[5m])) * 100
```

#### Active Workflows

```promql
# Currently running workflows
conductor_workflows_active
```

#### Workflow Duration (p50, p95, p99)

```promql
# p50
histogram_quantile(0.50,
  rate(conductor_workflow_duration_seconds_bucket[5m])
)

# p95
histogram_quantile(0.95,
  rate(conductor_workflow_duration_seconds_bucket[5m])
)

# p99
histogram_quantile(0.99,
  rate(conductor_workflow_duration_seconds_bucket[5m])
)
```

#### LLM Request Rate

```promql
# Requests per second by provider
sum by (provider) (
  rate(conductor_llm_requests_total[5m])
)
```

#### Token Consumption

```promql
# Total tokens per minute
sum by (type) (
  rate(conductor_llm_tokens_total[1m]) * 60
)
```

#### LLM Cost per Hour

```promql
# Cost rate (dollars per hour)
sum by (provider) (
  rate(conductor_llm_cost_total[1h]) * 3600
)
```

#### Error Rate

```promql
# Errors per minute
sum(rate(conductor_workflow_errors_total[1m])) * 60
```

#### HTTP Request Latency

```promql
# p95 latency
histogram_quantile(0.95,
  rate(conductor_http_request_duration_seconds_bucket[5m])
)
```

### Import Pre-built Dashboard

Import the official Conductor dashboard:

1. Navigate to **Dashboards** > **Import**
2. Upload `conductor-dashboard.json` (from repository)
3. Select Prometheus data source
4. Click **Import**

## Alerting

### Prometheus Alerting Rules

Define alert rules:

```yaml
# alerts.yml
groups:
  - name: conductor
    interval: 30s
    rules:
      # High error rate
      - alert: HighWorkflowErrorRate
        expr: |
          sum(rate(conductor_workflow_errors_total[5m]))
          /
          sum(rate(conductor_workflows_total[5m])) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High workflow error rate"
          description: "Error rate is {{ $value | humanizePercentage }}"

      # No workflows executing (possible downtime)
      - alert: NoWorkflowActivity
        expr: |
          rate(conductor_workflows_total[10m]) == 0
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "No workflow activity detected"
          description: "No workflows executed in last 10 minutes"

      # High LLM latency
      - alert: HighLLMLatency
        expr: |
          histogram_quantile(0.95,
            rate(conductor_llm_latency_seconds_bucket[5m])
          ) > 30
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High LLM API latency"
          description: "p95 latency is {{ $value }}s"

      # High cost rate
      - alert: HighLLMCost
        expr: |
          sum(rate(conductor_llm_cost_total[1h]) * 3600) > 50
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "High LLM cost rate"
          description: "Spending ${{ $value | humanize }}/hour"

      # Service down
      - alert: ConductorDown
        expr: up{job="conductor"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Conductor service is down"
          description: "Conductor has been down for 1 minute"

      # Memory usage high
      - alert: HighMemoryUsage
        expr: |
          conductor_memory_bytes / (1024*1024*1024) > 4
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value | humanize }}GB"
```

### Alertmanager Configuration

Configure alert routing and notifications:

```yaml
# alertmanager.yml
global:
  resolve_timeout: 5m
  slack_api_url: '${SLACK_WEBHOOK_URL}'

route:
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'slack'

  routes:
    # Critical alerts go to PagerDuty
    - match:
        severity: critical
      receiver: 'pagerduty'
      continue: true

    # All alerts go to Slack
    - match_re:
        severity: warning|critical
      receiver: 'slack'

receivers:
  - name: 'slack'
    slack_configs:
      - channel: '#conductor-alerts'
        title: 'Conductor Alert: {{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: '${PAGERDUTY_SERVICE_KEY}'
```

Start Alertmanager:

```bash
./alertmanager --config.file=alertmanager.yml
```

Update Prometheus config:

```yaml
# prometheus.yml
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - localhost:9093

rule_files:
  - "alerts.yml"
```

## Log Aggregation

### Structured Logging

Conductor outputs JSON logs for easy parsing:

```json
{
  "timestamp": "2025-12-24T10:00:00Z",
  "level": "info",
  "message": "Workflow started",
  "workflow": "pr-review",
  "workflow_id": "wf-123456",
  "trigger": "webhook"
}
```

### Loki Setup

Collect logs with Grafana Loki:

```bash
# Install Loki
docker run -d \
  --name loki \
  -p 3100:3100 \
  -v /path/to/loki-config.yml:/etc/loki/local-config.yaml \
  grafana/loki
```

Loki configuration:

```yaml
# loki-config.yml
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

schema_config:
  configs:
    - from: 2024-01-01
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  boltdb_shipper:
    active_index_directory: /loki/boltdb-shipper-active
    cache_location: /loki/boltdb-shipper-cache
  filesystem:
    directory: /loki/chunks
```

### Promtail for Log Collection

Install Promtail to ship logs to Loki:

```bash
docker run -d \
  --name promtail \
  -v /var/log/conductor:/var/log/conductor \
  -v /path/to/promtail-config.yml:/etc/promtail/config.yml \
  grafana/promtail
```

Promtail configuration:

```yaml
# promtail-config.yml
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://localhost:3100/loki/api/v1/push

scrape_configs:
  - job_name: conductor
    static_configs:
      - targets:
          - localhost
        labels:
          job: conductor
          __path__: /var/log/conductor/*.log
    pipeline_stages:
      - json:
          expressions:
            timestamp: timestamp
            level: level
            message: message
            workflow: workflow
      - labels:
          level:
          workflow:
```

### Query Logs in Grafana

1. Add Loki data source in Grafana
2. Set URL: `http://localhost:3100`
3. Query logs:

```logql
# All Conductor logs
{job="conductor"}

# Errors only
{job="conductor"} |= "error"

# Specific workflow
{job="conductor", workflow="pr-review"}

# Failed workflows
{job="conductor"} | json | level="error" | message=~"workflow failed"

# Rate of errors
rate({job="conductor"} | json | level="error" [5m])
```

### ELK Stack (Alternative)

Use Elasticsearch, Logstash, Kibana:

#### Filebeat configuration

```yaml
# filebeat.yml
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/log/conductor/*.log
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["localhost:9200"]
  index: "conductor-%{+yyyy.MM.dd}"

setup.kibana:
  host: "localhost:5601"
```

Start Filebeat:

```bash
./filebeat -e -c filebeat.yml
```

### Fluentd (Alternative)

Collect and forward logs:

```xml
# fluentd.conf
<source>
  @type tail
  path /var/log/conductor/*.log
  pos_file /var/log/td-agent/conductor.pos
  tag conductor.*
  <parse>
    @type json
  </parse>
</source>

<match conductor.**>
  @type elasticsearch
  host localhost
  port 9200
  logstash_format true
  logstash_prefix conductor
</match>
```

## Health Checks

### Liveness Probe

Check if Conductor is running:

```bash
curl http://localhost:8080/health
```

Response:

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h30m15s"
}
```

Kubernetes liveness probe:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  failureThreshold: 3
```

### Readiness Probe

Check if Conductor is ready to serve requests:

```bash
curl http://localhost:8080/ready
```

Response when ready:

```json
{
  "status": "ready",
  "checks": {
    "llm_providers": "ok",
    "storage": "ok"
  }
}
```

Kubernetes readiness probe:

```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

## Distributed Tracing (Future)

Future support for OpenTelemetry tracing:

```yaml
# config.yaml (future)
tracing:
  enabled: true
  exporter: jaeger
  endpoint: http://localhost:14268/api/traces
  sample_rate: 0.1
```

Trace workflow execution across:
- HTTP requests
- LLM API calls
- Tool executions
- Step transitions

## Performance Monitoring

### Request Rate

Monitor workflow execution rate:

```promql
# Requests per second
rate(conductor_workflows_total[1m])

# By workflow name
sum by (workflow) (
  rate(conductor_workflows_total[1m])
)
```

### Latency Percentiles

Track response time distribution:

```promql
# Latency percentiles
histogram_quantile(0.50, rate(conductor_workflow_duration_seconds_bucket[5m]))  # p50
histogram_quantile(0.90, rate(conductor_workflow_duration_seconds_bucket[5m]))  # p90
histogram_quantile(0.99, rate(conductor_workflow_duration_seconds_bucket[5m]))  # p99
```

### Error Budget

Calculate error budget (SLO: 99.9% success):

```promql
# Error budget remaining (%)
(
  1 - (
    sum(rate(conductor_workflow_errors_total[30d]))
    /
    sum(rate(conductor_workflows_total[30d]))
  )
) * 100

# Should be >= 99.9
```

## Cost Monitoring

### Token Usage

Track token consumption:

```promql
# Tokens per hour
sum by (provider, type) (
  rate(conductor_llm_tokens_total[1h]) * 3600
)

# Cost per workflow
(
  sum(rate(conductor_llm_cost_total[5m]))
  /
  sum(rate(conductor_workflows_total[5m]))
)
```

### Cost Alerts

Alert on unexpected cost increases:

```yaml
# Alert if spending exceeds budget
- alert: CostBudgetExceeded
  expr: |
    sum(rate(conductor_llm_cost_total[24h]) * 86400) > 100
  labels:
    severity: warning
  annotations:
    summary: "Daily cost budget exceeded"
    description: "Spending ${{ $value }} per day"
```

## Monitoring Best Practices

### What to Monitor

**Golden Signals:**
- **Latency** - How long workflows take
- **Traffic** - Workflows per second
- **Errors** - Failure rate
- **Saturation** - Resource utilization

**Business Metrics:**
- Workflows by type/purpose
- Token usage and cost
- User activity patterns

### Alert Fatigue

Avoid alert fatigue:
- Set appropriate thresholds
- Use `for` clause to avoid flapping
- Group related alerts
- Route by severity

### Dashboard Organization

Organize dashboards by audience:
- **Operations** - Service health, errors, latency
- **Business** - Workflow counts, usage trends
- **Cost** - Token usage, provider costs
- **Debugging** - Detailed metrics for troubleshooting

## Troubleshooting

For detailed troubleshooting procedures, see:
- [Troubleshooting Guide](troubleshooting.md)
- [Startup Runbook](startup.md)

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/)
