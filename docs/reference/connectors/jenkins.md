# Jenkins

The Jenkins connector provides integration with Jenkins CI/CD.

## Quick Start

```conductor
connectors:
  jenkins:
    from: connectors/jenkins
    base_url: https://jenkins.company.com
    auth:
      type: basic
      username: ${JENKINS_USER}
      password: ${JENKINS_API_TOKEN}
```

## Authentication

Generate API token in Jenkins: User menu → Configure → API Token

```bash
export JENKINS_USER=your-username
export JENKINS_API_TOKEN=your-token
```

## Operations

### Build Management

#### trigger_build

Trigger a new build for a job.

```conductor
- id: start_build
  type: connector
  connector: jenkins.trigger_build
  inputs:
    job: my-project/main
    parameters:
      BRANCH: main
      ENVIRONMENT: production
```

**Inputs:**
- `job` (required): Job name or path (e.g., "my-job" or "folder/my-job")
- `parameters`: Map of build parameters (key-value pairs)
- `token`: Build token for authentication (if configured on job)

**Output:** `{queue_item: {id, url}}`

Note: Returns immediately with queue item. Use `get_queue_item` to track build start.

#### get_build

Get details about a specific build.

```conductor
- id: fetch_build
  type: connector
  connector: jenkins.get_build
  inputs:
    job: my-project/main
    build_number: 42
```

**Inputs:**
- `job` (required): Job name or path
- `build_number` (required): Build number (integer)

**Output:** `{number, url, result, building, duration, timestamp, description, changeSet}`

Build result values:
- `SUCCESS` - Build completed successfully
- `FAILURE` - Build failed
- `UNSTABLE` - Build succeeded but unstable (e.g., test failures)
- `ABORTED` - Build was cancelled
- `null` - Build is still running

#### get_build_log

Get the console output for a build.

```conductor
- id: fetch_logs
  type: connector
  connector: jenkins.get_build_log
  inputs:
    job: my-project/main
    build_number: 42
    start: 0
```

**Inputs:**
- `job` (required): Job name or path
- `build_number` (required): Build number
- `start`: Starting byte offset for partial log (default: 0)

**Output:** `{text, hasMore, size}` - Console output text and metadata

Use `start` parameter to fetch logs incrementally for running builds.

#### cancel_build

Cancel a running build.

```conductor
- id: stop_build
  type: connector
  connector: jenkins.cancel_build
  inputs:
    job: my-project/main
    build_number: 42
```

**Inputs:**
- `job` (required): Job name or path
- `build_number` (required): Build number

**Output:** `{ok: true}`

#### get_test_report

Get test results for a build.

```conductor
- id: fetch_tests
  type: connector
  connector: jenkins.get_test_report
  inputs:
    job: my-project/main
    build_number: 42
```

**Inputs:**
- `job` (required): Job name or path
- `build_number` (required): Build number

**Output:** `{totalCount, failCount, skipCount, passCount, suites: [{name, cases}]}`

Note: Only available if job publishes test results (e.g., JUnit plugin).

### Job Management

#### get_job

Get information about a job.

```conductor
- id: job_info
  type: connector
  connector: jenkins.get_job
  inputs:
    job: my-project/main
```

**Inputs:**
- `job` (required): Job name or path

**Output:** `{name, url, description, buildable, builds: [{number, url}], lastBuild, lastSuccessfulBuild, lastFailedBuild}`

#### list_jobs

List all jobs on the Jenkins instance.

```conductor
- id: get_all_jobs
  type: connector
  connector: jenkins.list_jobs
  inputs:
    folder: "production"
```

**Inputs:**
- `folder`: Optional folder path to list jobs from (default: root)
- `depth`: API depth for nested data (default: 1)

**Output:** `[{name, url, color, buildable}]`

Job color indicates status:
- `blue` - Last build successful
- `red` - Last build failed
- `yellow` - Last build unstable
- `aborted` - Last build aborted
- `notbuilt` - Job never built
- Suffix `_anime` indicates currently building

### Queue Management

#### get_queue_item

Get information about a queued build item.

```conductor
- id: check_queue
  type: connector
  connector: jenkins.get_queue_item
  inputs:
    queue_id: 12345
```

**Inputs:**
- `queue_id` (required): Queue item ID from trigger_build

**Output:** `{id, url, why, blocked, buildable, stuck, executable: {number, url}}`

The `executable` field appears once the build starts and contains the build number.

### Node Management

#### list_nodes

List all Jenkins nodes (agents).

```conductor
- id: get_agents
  type: connector
  connector: jenkins.list_nodes
```

**Inputs:** None

**Output:** `[{name, offline, temporarilyOffline, idle, offlineCauseReason}]`

#### get_node

Get details about a specific node.

```conductor
- id: node_info
  type: connector
  connector: jenkins.get_node
  inputs:
    node: "build-agent-01"
```

**Inputs:**
- `node` (required): Node name (use "master" for controller)

**Output:** `{name, description, numExecutors, offline, temporarilyOffline, offlineCauseReason, monitorData}`

## Examples

### Trigger Build and Wait for Completion

```conductor
steps:
  - id: start_build
    type: connector
    connector: jenkins.trigger_build
    inputs:
      job: "my-project/main"
      parameters:
        BRANCH: "{{.inputs.branch}}"

  - id: wait_for_start
    type: connector
    connector: jenkins.get_queue_item
    inputs:
      queue_id: "{{.steps.start_build.queue_item.id}}"
    retry:
      max_attempts: 30
      delay: 2s
      condition: "{{.steps.wait_for_start.executable == nil}}"

  - id: wait_for_completion
    type: connector
    connector: jenkins.get_build
    inputs:
      job: "my-project/main"
      build_number: "{{.steps.wait_for_start.executable.number}}"
    retry:
      max_attempts: 300
      delay: 10s
      condition: "{{.steps.wait_for_completion.building}}"

  - id: get_logs
    type: connector
    connector: jenkins.get_build_log
    inputs:
      job: "my-project/main"
      build_number: "{{.steps.wait_for_start.executable.number}}"
```

### Monitor Build Health

```conductor
steps:
  - id: get_all_jobs
    type: connector
    connector: jenkins.list_jobs

  - id: analyze_failures
    type: llm
    model: fast
    prompt: |
      Analyze these Jenkins job statuses and identify trends:
      {{.steps.get_all_jobs | json}}

      Focus on recurring failures.

  - id: get_failed_details
    type: parallel
    for_each: "{{.steps.get_all_jobs | filter 'color' 'red'}}"
    steps:
      - id: get_job
        type: connector
        connector: jenkins.get_job
        inputs:
          job: "{{.item.name}}"

      - id: get_last_build
        type: connector
        connector: jenkins.get_build
        inputs:
          job: "{{.item.name}}"
          build_number: "{{.steps.get_job.lastBuild.number}}"
```

### Automated Build Retry

```conductor
steps:
  - id: get_build
    type: connector
    connector: jenkins.get_build
    inputs:
      job: "{{.inputs.job}}"
      build_number: "{{.inputs.build_number}}"

  - id: analyze_failure
    type: llm
    model: balanced
    prompt: |
      Analyze this build failure:
      Result: {{.steps.get_build.result}}
      {{.steps.get_build_log.text}}

      Is this a transient/flaky failure that should be retried?
      Return: yes or no

  - id: retry_build
    type: connector
    connector: jenkins.trigger_build
    when: "{{.steps.analyze_failure.should_retry == 'yes'}}"
    inputs:
      job: "{{.inputs.job}}"
      parameters: "{{.steps.get_build.parameters}}"
```

## Troubleshooting

### 401 Unauthorized

**Problem**: Authentication fails

**Solutions**:
1. Generate new API token: User menu → Configure → API Token
2. Verify username is correct (not email)
3. Check `base_url` includes protocol (https://)

### 403 Forbidden

**Problem**: User lacks permissions

**Solutions**:
1. Check user has "Build" permission for the job
2. Verify user has "Read" permission
3. For job creation/deletion, need "Configure" permission

### 404 Job Not Found

**Problem**: Cannot find job

**Solutions**:
1. Verify job name exactly (case-sensitive)
2. For jobs in folders, use path: "folder/subfolder/job"
3. Check job exists and user has read access

### 500 Build Parameters Invalid

**Problem**: Build fails to start

**Solutions**:
1. Verify parameter names match job configuration exactly
2. Check parameter types (string, boolean, choice)
3. Ensure required parameters are provided

### Connection Timeout

**Problem**: Requests timeout

**Solutions**:
1. Check Jenkins instance is accessible
2. Verify firewall/network allows access
3. Increase timeout in connector config
4. For long builds, use polling instead of waiting

## See Also

- [Jenkins REST API](https://www.jenkins.io/doc/book/using/remote-access-api/)
- [Remote Access API](https://www.jenkins.io/doc/book/using/remote-access-api/)
- [Jenkins Job DSL](https://plugins.jenkins.io/job-dsl/)
