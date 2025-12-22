# Sandbox Package

The `sandbox` package provides process isolation for Conductor tool execution under `strict` and `air-gapped` security profiles.

## Overview

Sandboxes isolate tool execution to prevent:
- Unauthorized filesystem access
- Network exfiltration
- Resource exhaustion
- Lateral movement to other processes

## Implementations

### Docker/Podman (Primary)

Container-based isolation using Docker or Podman:

**Features:**
- Full filesystem isolation with bind mounts
- Network isolation (none, filtered, full)
- Resource limits (memory, CPU, processes)
- Read-only root filesystem
- Security options (no-new-privileges)

**Requirements:**
- Docker or Podman installed and running
- Image available (defaults to `alpine:latest`)

**Example:**
```go
factory := sandbox.NewDockerFactory()
if !factory.Available(ctx) {
    // Docker/Podman not available
}

cfg := sandbox.Config{
    WorkflowID:  "my-workflow",
    WorkDir:     "/path/to/workspace",
    NetworkMode: sandbox.NetworkNone,
    ResourceLimits: sandbox.ResourceLimits{
        MaxMemory: 512 * 1024 * 1024, // 512MB
        MaxCPU:    50,                 // 50% = 0.5 cores
    },
}

sb, err := factory.Create(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer sb.Cleanup()

output, err := sb.Execute(ctx, "ls", []string{"-la"})
```

### Fallback (Degraded Mode)

Process-level isolation when containers are unavailable:

**Features:**
- Restricted environment variables (credentials filtered)
- Separate process group for cleanup
- File operations via host filesystem

**Limitations:**
- No memory/CPU limits
- No network isolation
- No filesystem isolation (relies on SecurityInterceptor)
- No seccomp filtering

**Example:**
```go
factory := sandbox.NewFallbackFactory()
// Always available

cfg := sandbox.Config{
    WorkflowID: "my-workflow",
    Env: map[string]string{
        "SAFE_VAR": "value",
        "AWS_SECRET_ACCESS_KEY": "filtered", // Will be removed
    },
}

sb, err := factory.Create(ctx, cfg)
defer sb.Cleanup()
```

## Credential Protection

Per spec FR7.4, credentials must be handled securely:

**DO:**
- Use tmpfs-mounted secrets files for credentials in containers
- Filter environment variables before passing to sandbox
- Remove `AWS_*`, `API_KEY*`, `*_TOKEN`, etc.

**DON'T:**
- Pass credentials via command-line arguments (visible in `ps`)
- Pass credentials via environment variables to sandbox
- Store credentials in sandbox filesystem

## Degraded Mode

When Docker/Podman is unavailable, the sandbox falls back to process-level isolation:

**Detection:**
```go
selector := sandbox.NewFactorySelector()
factory, degraded, err := selector.SelectFactory(ctx)
if degraded {
    // Running in degraded mode
    warning := sandbox.GetDegradedModeWarning(profileName)
    fmt.Fprintln(os.Stderr, warning)
}
```

**Behavior:**
- Filesystem allowlists: Still enforced (via SecurityInterceptor)
- Network allowlists: Still enforced (via SecurityInterceptor)
- Command allowlists: Still enforced (via SecurityInterceptor)
- Resource limits: Not enforced
- Process isolation: Not enforced
- Network isolation: Not enforced

## Seccomp Profile

The embedded `seccomp.json` profile for Linux containers blocks dangerous syscalls:

**Allowed:**
- Basic I/O (read, write, close, fstat)
- File operations (openat with O_NOFOLLOW)
- Memory allocation (mmap without PROT_EXEC)
- Process termination
- Time operations

**Blocked:**
- Memory protection changes (mprotect - prevents W^X bypass)
- Process spawning (fork, execve - no subprocesses)
- Network access (socket, connect - all network syscalls)
- Debugging (ptrace - no process inspection)
- Privilege escalation (setuid, setgid)

## Sandbox-exec Profile (macOS)

The `sandbox-exec-profile.sb` is provided for reference only:

**Status:** Deprecated by Apple
**Recommendation:** Use Docker Desktop for Mac instead

## Testing

### Unit Tests (Fallback)
```bash
go test ./pkg/security/sandbox -v -run TestFallback
```

### Integration Tests (Docker)
```bash
# Requires Docker/Podman running
go test ./pkg/security/sandbox -v -run TestDocker
```

### Skip Docker Tests
```bash
# If Docker unavailable, tests are automatically skipped
go test ./pkg/security/sandbox -v
# Output: "Docker/Podman not available, skipping integration tests"
```

## Architecture

```
┌─────────────────────────────────────┐
│   WorkflowSecurityContext           │
│   (pkg/security/context.go)         │
└────────────┬────────────────────────┘
             │ GetSandbox()
             ▼
┌─────────────────────────────────────┐
│   FactorySelector                   │
│   - Try Docker/Podman first         │
│   - Fallback to process isolation   │
└────────────┬────────────────────────┘
             │
       ┌─────┴─────┐
       ▼           ▼
┌──────────┐  ┌──────────┐
│  Docker  │  │ Fallback │
│  Sandbox │  │ Sandbox  │
└──────────┘  └──────────┘
```

## Performance

Container startup overhead:
- First tool call: ~300ms (p50), ~500ms (p99)
- Subsequent calls: ~50ms (container reuse)

**Optimization:**
```go
// Pre-warm sandbox during workflow initialization
secCtx := manager.CreateContext(workflowID, true) // prewarm=true
secCtx.PrewarmSandbox(ctx)
```

## Security Considerations

1. **Container Images:** Use minimal, verified base images (alpine, distroless)
2. **Network Isolation:** Default to `NetworkNone` for air-gapped profiles
3. **Resource Limits:** Always set memory/CPU limits to prevent DoS
4. **Read-only Root:** Containers use `--read-only` by default
5. **No New Privileges:** Containers use `--security-opt no-new-privileges`

## Future Enhancements

- [ ] Native Linux seccomp support (without containers)
- [ ] macOS App Sandbox (replacement for deprecated sandbox-exec)
- [ ] Windows job objects for isolation
- [ ] Network filtering via iptables for `NetworkFiltered` mode
- [ ] Container image signing and verification
- [ ] Rootless container support

## References

- Spec: SPEC-14 (Agent Security Model)
- Docker security: https://docs.docker.com/engine/security/
- Seccomp: https://man7.org/linux/man-pages/man2/seccomp.2.html
- Linux namespaces: https://man7.org/linux/man-pages/man7/namespaces.7.html
