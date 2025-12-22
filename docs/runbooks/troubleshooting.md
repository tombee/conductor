# Backend Troubleshooting Runbook

## Quick Diagnostics

Run these commands first to gather information:

```bash
# 1. Check if backend is running
ps aux | grep conduct

# 2. Test health endpoint
curl http://localhost:9876/health

# 3. Check port availability
lsof -i :9876

# 4. View recent logs (packaged app)
tail -n 100 ~/Library/Logs/foreman/backend.log

# 5. Check metrics endpoint
curl http://localhost:9876/metrics
```

## Common Issues

### Backend Fails to Start

**Symptoms:**
- Electron shows "Go backend unavailable"
- No process found when running `ps aux | grep conduct`
- Electron logs show repeated restart attempts

**Diagnosis:**

1. **Check binary exists:**
   ```bash
   ls -lh /path/to/conduct
   # Should show executable permissions (-rwxr-xr-x)
   ```

2. **Try manual startup:**
   ```bash
   ./conductor --port 9876
   # Watch for errors on stderr
   ```

**Common causes:**

**Binary not found or not executable:**
```bash
# Fix permissions
chmod +x ./conduct

# Verify it runs
./conductor --version
```

**Port range 9876-9899 all in use:**
```bash
# Find processes using ports
for port in {9876..9899}; do
  lsof -i :$port
done

# Kill conflicting processes or restart system
```

**Missing dependencies (rare on macOS):**
```bash
# Check for missing libraries
otool -L ./conduct
# All libraries should be found
```

**Incompatible architecture:**
```bash
# Check binary architecture
file ./conduct
# Should match system (arm64 for Apple Silicon, x86_64 for Intel)
```

### Backend Starts but Health Check Fails

**Symptoms:**
- Process is running (`ps aux | grep conduct`)
- `curl http://localhost:9876/health` times out or connection refused

**Diagnosis:**

1. **Check if process is listening:**
   ```bash
   lsof -i :9876
   # Should show conductor process
   ```

2. **Check backend logs:**
   ```bash
   # Look for port binding errors
   LOG_LEVEL=debug ./conduct
   ```

**Common causes:**

**Backend bound to wrong interface:**
- Backend may be listening on 127.0.0.1 only
- Frontend trying to connect to 0.0.0.0

**Resolution:**
Backend should bind to `127.0.0.1:<port>` for security.
Frontend should connect to `http://127.0.0.1:<port>`.

**Firewall blocking localhost:**
```bash
# Check firewall rules (rare)
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate
```

**Backend crashed during initialization:**
Check logs for panic or fatal error during startup.

### Backend Crashes Repeatedly

**Symptoms:**
- Electron falls back to Node.js backend
- Logs show "Backend exited unexpectedly" repeated 3+ times
- Auto-restart attempts exhausted

**Diagnosis:**

1. **Check exit code:**
   ```bash
   ./conductor &
   PID=$!
   wait $PID
   echo "Exit code: $?"
   # 0 = clean exit
   # 1 = error
   # 2 = panic
   # 139 = SIGSEGV (segfault)
   ```

2. **Look for panic in logs:**
   ```
   panic: runtime error: invalid memory address
   ```

3. **Check resource limits:**
   ```bash
   ulimit -a
   # Ensure open files and memory limits are sufficient
   ```

**Common causes:**

**Configuration error:**
- Invalid config file
- Missing required environment variable
- Incompatible version

**Resource exhaustion:**
- Out of memory (backend needs ~200MB)
- File descriptor limit reached
- Disk full (for SQLite database)

**Bug in initialization code:**
Check logs for specific error message, then report issue with:
- Backend version (`./conductor --version`)
- Full error message
- Steps to reproduce

### WebSocket Connection Fails

**Symptoms:**
- Health check works
- RPC connection fails with "WebSocket connection error"
- Frontend shows "Backend connected" then "Disconnected"

**Diagnosis:**

1. **Test WebSocket connection:**
   ```bash
   # Install wscat if needed: npm install -g wscat
   wscat -c ws://localhost:9876/rpc
   ```

2. **Check authentication:**
   WebSocket requires `Authorization: Bearer <token>` header.
   Token is generated on backend startup.

3. **Verify RPC endpoint:**
   ```bash
   curl -v http://localhost:9876/rpc
   # Should upgrade to WebSocket
   ```

**Common causes:**

**Missing authentication token:**
Frontend must obtain token from environment or backend stdout.

**WebSocket upgrade failed:**
- Check nginx/proxy configuration if using reverse proxy
- Backend must support WebSocket upgrade headers

**Connection rate limiting:**
Backend may reject connections if too many failed attempts.

**Resolution:**
```bash
# Check backend logs for auth failures
# Look for "E_AUTH_FAILED" events
```

### LLM Provider Errors

**Symptoms:**
- LLM requests fail with "Provider error"
- Logs show "Provider failover" events
- All providers failing

**Diagnosis:**

1. **Check provider credentials:**
   ```bash
   # Verify API key is set
   echo $ANTHROPIC_API_KEY

   # Or check keychain (macOS)
   security find-generic-password -s "foreman-anthropic-key" -w
   ```

2. **Test provider directly:**
   ```bash
   curl https://api.anthropic.com/v1/messages \
     -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -H "content-type: application/json" \
     -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
   ```

3. **Check provider status:**
   Visit status pages:
   - Anthropic: https://status.anthropic.com
   - OpenAI: https://status.openai.com

**Common causes:**

**Invalid API key:**
- Expired or revoked key
- Typo in key
- Wrong key for provider

**Rate limiting:**
- Too many requests (HTTP 429)
- Exceeds API quota

**Provider outage:**
- Service degradation (HTTP 503)
- Regional outage

**Resolution:**

For invalid credentials:
```bash
# Update keychain
security add-generic-password \
  -s "foreman-anthropic-key" \
  -a "foreman" \
  -w "your-api-key" \
  -U
```

For rate limiting:
- Backend should auto-retry with exponential backoff
- Check metrics for request rate: `curl http://localhost:9876/metrics`

### High Latency or Slow Responses

**Symptoms:**
- Requests taking much longer than expected
- UI feels sluggish
- Timeouts occurring

**Diagnosis:**

1. **Check metrics endpoint:**
   ```bash
   curl http://localhost:9876/metrics
   ```

   Look for:
   - `llm_request_duration_p99`: Should be < 5s
   - `rpc_request_duration_p99`: Should be < 10ms
   - `workflow_transition_duration_p99`: Should be < 50ms

2. **Check connection pool:**
   ```json
   {
     "connection_pool": {
       "active": 8,
       "idle": 2,
       "max": 10
     }
   }
   ```

   If `active` = `max`, pool is saturated.

3. **Check concurrent workflows:**
   ```json
   {
     "workflows": {
       "active": 15,
       "total": 100
     }
   }
   ```

   If `active` > 10, performance may degrade.

**Common causes:**

**Connection pool exhaustion:**
- Too many concurrent LLM requests
- Connections not being released

**Resolution:**
- Reduce concurrent workflow limit
- Increase connection pool size in config

**Network issues:**
- High latency to LLM provider
- DNS resolution slow

**Resolution:**
```bash
# Test latency to provider
ping api.anthropic.com

# Test DNS resolution
time nslookup api.anthropic.com
```

**Resource contention:**
- High CPU usage
- Memory pressure

**Resolution:**
```bash
# Check backend CPU/memory
ps aux | grep conduct

# Check system resources
top
```

### Database Errors

**Symptoms:**
- "Database locked" errors
- "Unable to open database" errors
- Workflow state not persisting

**Diagnosis:**

1. **Check database file:**
   ```bash
   ls -lh ~/.config/foreman/conduct.db
   # Should be readable/writable
   ```

2. **Check for locks:**
   ```bash
   lsof ~/.config/foreman/conduct.db
   # Should only show conductor process
   ```

3. **Test database integrity:**
   ```bash
   sqlite3 ~/.config/foreman/conduct.db "PRAGMA integrity_check;"
   # Should return "ok"
   ```

**Common causes:**

**Database locked:**
- Multiple backend instances running
- Unclean shutdown left lock file

**Resolution:**
```bash
# Kill all conductor processes
pkill -9 conduct

# Remove lock files
rm -f ~/.config/foreman/conduct.db-wal
rm -f ~/.config/foreman/conduct.db-shm

# Restart backend
```

**Database corruption:**
- Disk full during write
- Power loss during transaction
- Filesystem error

**Resolution:**
```bash
# Backup database
cp ~/.config/foreman/conduct.db ~/.config/foreman/conduct.db.backup

# Try to repair
sqlite3 ~/.config/foreman/conduct.db "VACUUM;"

# If corruption is severe, delete and restart
# (workflow history will be lost)
mv ~/.config/foreman/conduct.db ~/.config/foreman/conduct.db.corrupt
# Backend will create new database on next start
```

## Diagnostic Tools

### Enable Debug Logging

```bash
# Temporary (current session)
LOG_LEVEL=debug ./conduct

# Permanent (add to Electron environment)
export LOG_LEVEL=debug
```

Debug logs include:
- Detailed startup sequence
- RPC message payloads
- LLM request/response bodies
- State machine transitions

### Collect Diagnostic Bundle

When reporting issues, collect:

```bash
#!/bin/bash
# collect-diagnostics.sh

echo "=== Backend Version ==="
./conductor --version

echo "=== Process Status ==="
ps aux | grep conduct

echo "=== Port Status ==="
lsof -i :9876

echo "=== Health Check ==="
curl -s http://localhost:9876/health | jq .

echo "=== Metrics ==="
curl -s http://localhost:9876/metrics | jq .

echo "=== Recent Logs ==="
tail -n 100 ~/Library/Logs/foreman/backend.log

echo "=== Database Status ==="
ls -lh ~/.config/foreman/conduct.db
sqlite3 ~/.config/foreman/conduct.db "PRAGMA integrity_check;"

echo "=== System Info ==="
uname -a
sw_vers
```

### Performance Profiling

If backend is slow:

```bash
# Enable pprof (requires CONDUCT_DEBUG=1)
CONDUCT_DEBUG=1 ./conduct

# Collect CPU profile (30s)
curl http://localhost:9876/debug/pprof/profile?seconds=30 > cpu.prof

# Collect heap profile
curl http://localhost:9876/debug/pprof/heap > heap.prof

# Analyze with pprof
go tool pprof cpu.prof
# Then type 'top' or 'web' in pprof shell
```

## Escalation

If issue persists after trying troubleshooting steps:

1. **Collect diagnostic bundle** (see above)
2. **Check for known issues**: https://github.com/tombee/foreman/issues
3. **File new issue** with:
   - Backend version
   - Operating system and version
   - Full error message
   - Diagnostic bundle output
   - Steps to reproduce

## Recovery Procedures

### Force Fallback to Node.js Backend

If Go backend is completely broken:

```bash
# Disable Go backend in Electron
unset FOREMAN_GO_BACKEND

# Or start Electron with flag
FOREMAN_GO_BACKEND=0 npm start
```

### Reset Backend State

To start fresh:

```bash
# Stop backend
pkill conduct

# Backup data
cp -r ~/.config/foreman ~/.config/foreman.backup

# Remove database and logs
rm -f ~/.config/foreman/conduct.db*
rm -f ~/Library/Logs/foreman/backend.log

# Restart Electron
# Backend will reinitialize with clean state
```

### Roll Back Backend Version

If new version has issues:

```bash
# Replace conductor binary with previous version
cp /path/to/old/conductor /path/to/app/resources/conduct

# Restart Electron
```
