# Backend Startup Runbook

## Overview

This runbook covers the Conductor backend startup process, common issues, and resolution steps.

## Startup Sequence

### 1. Process Launch

When the Electron app starts, it spawns the Go backend:

```typescript
// Process manager spawns backend
const backend = spawn('./conduct', ['--port', '9876'])
```

**Expected behavior:**
- Backend prints `CONDUCTOR_BACKEND_PORT=<PORT>` to stdout within 2s
- Process stays running without exiting
- Health check endpoint becomes available within 5s

**Common issues:**
- **Binary not found**: Verify `conduct` binary exists in app resources
- **Port in use**: Backend will try ports 9876-9899 sequentially
- **Permission denied**: Check binary has execute permissions

### 2. Port Discovery

The process manager reads the backend's stdout to discover the assigned port.

**Expected output:**
```
CONDUCTOR_BACKEND_PORT=9876
```

**Common issues:**
- **No port output**: Backend may have crashed immediately - check stderr
- **Invalid port format**: Corrupted output or version mismatch
- **Timeout (2s)**: Backend initialization too slow - investigate startup delays

**Resolution:**
```bash
# Check if backend can start standalone
./conductor --port 9876

# Should print port to stdout
# Press Ctrl+C to stop
```

### 3. Health Check Polling

Once port is discovered, Electron polls the health endpoint every 500ms.

**Expected response:**
```json
{
  "status": "ready",
  "version": "0.1.0",
  "message": "Backend ready"
}
```

**Response codes:**
- `200 OK` + status="ready": Backend is healthy
- `503 Service Unavailable` + status="starting": Still initializing, keep polling
- `503 Service Unavailable` + status="error": Fatal error, trigger restart

**Common issues:**
- **Connection refused**: Backend not listening yet (normal during first 500ms)
- **Timeout after 2s**: Backend hung during initialization
- **Status stuck on "starting"**: Initialization step blocked (check logs)

**Resolution:**
```bash
# Test health endpoint manually
curl http://localhost:9876/health

# Check backend logs for initialization errors
# Logs are written to stderr by default
```

### 4. Ready State

Backend is considered ready when:
- Health check returns HTTP 200
- Response has `status: "ready"`
- Response time is < 2s

**Startup timeline:**
- **0-100ms**: Process launch
- **100-500ms**: Port binding and initialization
- **500-1000ms**: First health check succeeds
- **1000ms**: Backend considered ready

**Performance targets:**
- Health check p99 latency: < 10ms
- Time to ready state: < 1s
- Memory usage: < 200MB

## Auto-Restart Behavior

The backend auto-restarts on unexpected exit with exponential backoff.

**Restart policy:**
- **Max attempts**: 3 restarts within 60s window
- **Backoff**: 1s, 2s, 4s between attempts
- **Fallback**: After 3 failures, Electron falls back to Node.js backend

**Restart triggers:**
- Process exits unexpectedly (non-zero exit code)
- Process crashes (SIGSEGV, SIGABRT)
- Health check fails for 3 consecutive polls (1.5s)

**Auto-restart will NOT trigger for:**
- Graceful shutdown (SIGTERM with exit code 0)
- Electron app quit
- Manual backend stop

**Common restart scenarios:**

### Scenario: Panic during initialization
```
Attempt 1: Backend panics at startup
  → Wait 1s
  → Retry
Attempt 2: Backend panics again
  → Wait 2s
  → Retry
Attempt 3: Backend panics again
  → Wait 4s
  → Fallback to Node.js backend
```

### Scenario: Intermittent crash
```
Backend runs for 30s → crashes → restart
Backend runs for 120s → crashes → restart (new 60s window)
```

## Graceful Shutdown

On Electron app quit, the backend receives SIGTERM.

**Expected behavior:**
- Backend receives SIGTERM
- Ongoing requests complete (up to 5s grace period)
- WebSocket connections close cleanly
- Process exits with code 0

**Shutdown timeline:**
- **0ms**: SIGTERM received
- **0-5000ms**: Drain ongoing requests
- **5000ms**: Force shutdown if not complete
- Exit code 0: Clean shutdown
- Exit code 1: Forced shutdown

**Common issues:**
- **Backend hangs on shutdown**: Ongoing requests stuck
- **Forced shutdown (exit code 1)**: Grace period exceeded

**Resolution:**
```bash
# Test graceful shutdown
./conductor &
PID=$!
kill -TERM $PID
# Should exit within 5s with code 0
```

## Environment Variables

### Required
None - backend uses sensible defaults

### Optional
- `CONDUCTOR_BACKEND_PORT`: Force specific port (default: auto-assign from 9876-9899)
- `LOG_LEVEL`: Set logging verbosity (debug, info, warn, error)
- `ANTHROPIC_API_KEY`: Anthropic API key (if not using keychain)

### Development
- `FOREMAN_GO_BACKEND=1`: Enable Go backend in Electron app
- `CONDUCT_DEBUG=1`: Enable debug logging and pprof endpoints

## Diagnostic Commands

### Check backend version
```bash
./conductor --version
```

### Test health endpoint
```bash
curl http://localhost:9876/health
```

### Test WebSocket connection
```bash
# Using wscat
wscat -c ws://localhost:9876/rpc -H "Authorization: Bearer <token>"
```

### Monitor backend metrics
```bash
curl http://localhost:9876/metrics
```

### Check process status
```bash
# Find backend process
ps aux | grep conduct

# Check if port is in use
lsof -i :9876
```

## Logs and Debugging

**Log locations:**
- **Development**: Backend stderr redirected to console
- **Production**: `~/Library/Logs/conductor/conductord.log`
- **Standalone**: stderr

**Log format:**
```json
{
  "timestamp": "2025-12-22T10:00:00Z",
  "level": "info",
  "message": "Backend started",
  "port": 9876,
  "version": "0.1.0"
}
```

**Enable debug logging:**
```bash
LOG_LEVEL=debug ./conduct
```

**Useful debug information:**
- Startup sequence timing
- Health check requests/responses
- Port selection logic
- Initialization step completion

## Troubleshooting Checklist

Before investigating deeper:

- [ ] Backend binary exists and is executable
- [ ] No other process using ports 9876-9899
- [ ] Sufficient memory available (backend needs ~200MB)
- [ ] Electron and backend versions are compatible
- [ ] No firewall blocking localhost connections

If backend still fails to start, see [troubleshooting.md](./troubleshooting.md) for detailed diagnostics.
