// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/daemon/api"
	"github.com/tombee/conductor/internal/daemon/auth"
	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/internal/daemon/backend/memory"
	"github.com/tombee/conductor/internal/daemon/backend/postgres"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/daemon/endpoint"
	"github.com/tombee/conductor/internal/daemon/github"
	"github.com/tombee/conductor/internal/daemon/leader"
	"github.com/tombee/conductor/internal/daemon/listener"
	"github.com/tombee/conductor/internal/daemon/publicapi"
	daemonremote "github.com/tombee/conductor/internal/daemon/remote"
	"github.com/tombee/conductor/internal/daemon/runner"
	"github.com/tombee/conductor/internal/daemon/scheduler"
	"github.com/tombee/conductor/internal/daemon/webhook"
	internalllm "github.com/tombee/conductor/internal/llm"
	internallog "github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/workflow"
)

// Options contains daemon options set at build time.
type Options struct {
	Version   string
	Commit    string
	BuildDate string
}

// Daemon is the main conductord daemon.
type Daemon struct {
	cfg          *config.Config
	opts         Options
	logger       *slog.Logger
	server       *http.Server
	publicServer *publicapi.Server // Optional public API server for webhooks
	ln           net.Listener
	pidFile      string
	runner          *runner.Runner
	backend         backend.Backend
	checkpoints     *checkpoint.Manager
	scheduler       *scheduler.Scheduler
	endpointHandler *endpoint.Handler
	authMw          *auth.Middleware
	leader          *leader.Elector
	mcpRegistry     *mcp.Registry
	otelProvider    *tracing.OTelProvider

	mu      sync.Mutex
	started bool
}

// New creates a new daemon instance.
func New(cfg *config.Config, opts Options) (*Daemon, error) {
	// Create logger with daemon component context
	logger := internallog.WithComponent(internallog.New(internallog.FromEnv()), "daemon")

	// Create backend based on configuration
	var be backend.Backend
	var db *sql.DB

	switch cfg.Daemon.Backend.Type {
	case "postgres":
		pgCfg := postgres.Config{
			ConnectionString: cfg.Daemon.Backend.Postgres.ConnectionString,
			MaxOpenConns:     cfg.Daemon.Backend.Postgres.MaxOpenConns,
			MaxIdleConns:     cfg.Daemon.Backend.Postgres.MaxIdleConns,
			ConnMaxLifetime:  time.Duration(cfg.Daemon.Backend.Postgres.ConnMaxLifetimeSeconds) * time.Second,
		}
		pgBackend, err := postgres.New(pgCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres backend: %w", err)
		}
		be = pgBackend
		db = pgBackend.DB()
	default:
		// Default to in-memory backend
		be = memory.New()
	}

	// Create checkpoint manager
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{
		Dir: cfg.Daemon.CheckpointDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint manager: %w", err)
	}

	// Create runner with configured concurrency
	r := runner.New(runner.Config{
		MaxParallel:    cfg.Daemon.MaxConcurrentRuns,
		DefaultTimeout: cfg.Daemon.DefaultTimeout,
	}, be, cm)

	// Create remote workflow fetcher
	// This enables remote workflow support (github:user/repo)
	fetcher, err := daemonremote.NewFetcher(daemonremote.Config{
		GitHubToken: github.ResolveToken(),
		// Use default cache path and GitHub host
	})
	if err != nil {
		logger.Warn("failed to initialize remote workflow fetcher",
			internallog.Error(err))
		logger.Warn("remote workflows (github:user/repo) will not be available")
	} else {
		r.SetFetcher(fetcher)
	}

	// Create LLM provider for workflow execution
	// Use the default provider from config
	if cfg.DefaultProvider != "" {
		llmProvider, err := internalllm.CreateProvider(cfg, cfg.DefaultProvider)
		if err != nil {
			logger.Warn("failed to create LLM provider for workflow execution",
				internallog.Error(err),
				slog.String("provider", cfg.DefaultProvider))
			logger.Warn("workflows requiring LLM steps may fail without a configured provider")
		} else {
			// Create the workflow executor adapter
			providerAdapter := internalllm.NewProviderAdapter(llmProvider)
			// TODO(SPEC-36): Wire up tool registry once tool types are unified
			// For now, pass nil as tool registry (like CLI does)
			executor := workflow.NewExecutor(nil, providerAdapter)
			executionAdapter := runner.NewExecutorAdapter(executor)
			r.SetAdapter(executionAdapter)

			logger.Info("workflow execution adapter initialized",
				slog.String("provider", cfg.DefaultProvider))
		}
	} else {
		logger.Warn("no default LLM provider configured",
			slog.String("hint", "set default_provider in config to enable LLM workflow steps"))
	}

	// Create scheduler if enabled
	var sched *scheduler.Scheduler
	if cfg.Daemon.Schedules.Enabled && len(cfg.Daemon.Schedules.Schedules) > 0 {
		schedules := make([]scheduler.Schedule, len(cfg.Daemon.Schedules.Schedules))
		for i, s := range cfg.Daemon.Schedules.Schedules {
			schedules[i] = scheduler.Schedule{
				Name:     s.Name,
				Cron:     s.Cron,
				Workflow: s.Workflow,
				Inputs:   s.Inputs,
				Enabled:  s.Enabled,
				Timezone: s.Timezone,
			}
		}
		sched, err = scheduler.New(scheduler.Config{
			Schedules:    schedules,
			WorkflowsDir: cfg.Daemon.WorkflowsDir,
		}, r)
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler: %w", err)
		}
	}

	// Create endpoint handler if enabled
	var endpointHandler *endpoint.Handler
	if cfg.Daemon.Endpoints.Enabled {
		// Create rate limiter for endpoints
		rateLimiter := auth.NewNamedRateLimiter()

		// Load endpoint configuration with rate limiter
		registry, err := endpoint.LoadConfig(cfg.Daemon.Endpoints, cfg.Daemon.WorkflowsDir, rateLimiter)
		if err != nil {
			return nil, fmt.Errorf("failed to load endpoints: %w", err)
		}

		// Create handler and wire in rate limiter
		endpointHandler = endpoint.NewHandler(registry, r, cfg.Daemon.WorkflowsDir)
		endpointHandler.SetRateLimiter(rateLimiter)

		logger.Info("endpoints loaded",
			slog.Int("count", registry.Count()))
	}

	// Create auth middleware
	apiKeys := make([]auth.APIKey, len(cfg.Daemon.DaemonAuth.APIKeys))
	for i, key := range cfg.Daemon.DaemonAuth.APIKeys {
		apiKeys[i] = auth.APIKey{
			Key:       key,
			Name:      fmt.Sprintf("key-%d", i+1),
			CreatedAt: time.Now(),
		}
	}
	authMw := auth.NewMiddleware(auth.Config{
		Enabled:         cfg.Daemon.DaemonAuth.Enabled,
		APIKeys:         apiKeys,
		AllowUnixSocket: cfg.Daemon.DaemonAuth.AllowUnixSocket,
	})

	// Create leader elector if distributed mode is enabled
	var elector *leader.Elector
	if cfg.Daemon.Distributed.Enabled && db != nil {
		instanceID := cfg.Daemon.Distributed.InstanceID
		if instanceID == "" {
			instanceID = uuid.New().String()
		}
		elector = leader.NewElector(leader.Config{
			DB:            db,
			InstanceID:    instanceID,
			RetryInterval: 5 * time.Second,
		})
		logger.Info("distributed mode enabled",
			slog.String("instance_id", instanceID))
	}

	// Create MCP server registry
	mcpRegistry, err := mcp.NewRegistry(mcp.RegistryConfig{
		Logger: logger,
	})
	if err != nil {
		// MCP registry is optional - log warning but continue
		logger.Warn("failed to initialize MCP registry",
			internallog.Error(err))
		logger.Warn("MCP server management will not be available")
	}

	// Initialize OpenTelemetry provider for metrics and tracing
	var otelProvider *tracing.OTelProvider
	if cfg.Daemon.Observability.Enabled {
		tracingCfg := observabilityToTracingConfig(cfg.Daemon.Observability, opts.Version)
		otelProvider, err = tracing.NewOTelProviderWithConfig(tracingCfg)
		if err != nil {
			logger.Warn("failed to initialize OpenTelemetry provider",
				internallog.Error(err))
			logger.Warn("metrics and tracing will not be available")
		} else {
			logger.Info("OpenTelemetry provider initialized",
				slog.String("service_name", tracingCfg.ServiceName),
				slog.String("service_version", tracingCfg.ServiceVersion))
			// Wire metrics collector to runner
			r.SetMetrics(otelProvider.MetricsCollector())
		}
	}

	return &Daemon{
		cfg:             cfg,
		opts:            opts,
		logger:          logger,
		runner:          r,
		backend:         be,
		checkpoints:     cm,
		scheduler:       sched,
		endpointHandler: endpointHandler,
		authMw:          authMw,
		leader:          elector,
		mcpRegistry:     mcpRegistry,
		otelProvider:    otelProvider,
	}, nil
}

// Start starts the daemon and blocks until the context is cancelled.
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return fmt.Errorf("daemon already started")
	}
	d.started = true
	d.mu.Unlock()

	// Check permissions on critical directories and files at startup
	d.checkPermissionsAtStartup()

	// Write PID file if configured
	if d.cfg.Daemon.PIDFile != "" {
		if err := d.writePIDFile(); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
		d.pidFile = d.cfg.Daemon.PIDFile
	}

	// Resume any interrupted runs from checkpoints
	if err := d.runner.ResumeInterrupted(ctx); err != nil {
		d.logger.Warn("failed to resume interrupted runs",
			internallog.Error(err))
	}

	// Create listener
	ln, err := listener.New(d.cfg.Daemon.Listen)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	d.ln = ln

	// Create HTTP router
	router := api.NewRouter(api.RouterConfig{
		Version:   d.opts.Version,
		Commit:    d.opts.Commit,
		BuildDate: d.opts.BuildDate,
	})

	// Register runs API
	runsHandler := api.NewRunsHandler(d.runner)
	runsHandler.RegisterRoutes(router.Mux())

	// Register trigger API
	triggerHandler := api.NewTriggerHandler(d.runner, d.cfg.Daemon.WorkflowsDir)
	triggerHandler.RegisterRoutes(router.Mux())

	// Register webhook routes
	webhookRoutes := make([]webhook.Route, len(d.cfg.Daemon.Webhooks.Routes))
	for i, r := range d.cfg.Daemon.Webhooks.Routes {
		webhookRoutes[i] = webhook.Route{
			Path:         r.Path,
			Source:       r.Source,
			Workflow:     r.Workflow,
			Events:       r.Events,
			Secret:       r.Secret,
			InputMapping: r.InputMapping,
		}
	}
	webhookRouter := webhook.NewRouter(webhook.Config{
		Routes:       webhookRoutes,
		WorkflowsDir: d.cfg.Daemon.WorkflowsDir,
	}, d.runner)
	webhookRouter.RegisterRoutes(router.Mux())

	// Register schedules API
	schedulesHandler := api.NewSchedulesHandler(d.scheduler)
	schedulesHandler.RegisterRoutes(router.Mux())

	// Register endpoint routes if enabled
	if d.endpointHandler != nil {
		d.endpointHandler.RegisterRoutes(router.Mux())
	}

	// Register MCP API if registry is available
	if d.mcpRegistry != nil {
		mcpHandler := api.NewMCPHandler(d.mcpRegistry)
		mcpHandler.RegisterRoutes(router.Mux())
	}

	// Wire up scheduler to router for health endpoint
	if d.scheduler != nil {
		router.SetScheduleProvider(d.scheduler)
	}

	// Wire up MCP registry to router for health endpoint
	if d.mcpRegistry != nil {
		router.SetMCPProvider(&mcpStatusAdapter{registry: d.mcpRegistry})
	}

	// Wire up metrics handler if observability is enabled
	if d.otelProvider != nil {
		router.SetMetricsHandler(d.otelProvider.MetricsHandler())
	}

	// Create HTTP server with auth middleware
	var handler http.Handler = router
	if d.authMw != nil {
		handler = d.authMw.Wrap(handler)
	}

	d.server = &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Log startup
	d.logger.Info("conductord starting",
		slog.String("version", d.opts.Version),
		slog.String("listen_addr", ln.Addr().String()))

	// Start leader election if in distributed mode
	if d.leader != nil {
		d.leader.Start(ctx)

		// Register callback to start/stop scheduler based on leadership
		if d.scheduler != nil && d.cfg.Daemon.Distributed.LeaderElection {
			d.leader.OnLeadershipChange(func(isLeader bool) {
				if isLeader {
					d.scheduler.Start(ctx)
					d.logger.Info("became leader - scheduler started",
						slog.Int("schedule_count", len(d.cfg.Daemon.Schedules.Schedules)))
				} else {
					d.scheduler.Stop()
					d.logger.Info("lost leadership - scheduler stopped")
				}
			})
		}
	}

	// Start scheduler if configured (and not using leader election)
	if d.scheduler != nil && (d.leader == nil || !d.cfg.Daemon.Distributed.LeaderElection) {
		d.scheduler.Start(ctx)
		d.logger.Info("scheduler started",
			slog.Int("schedule_count", len(d.cfg.Daemon.Schedules.Schedules)))
	}

	// Start MCP registry (auto-starts configured servers)
	if d.mcpRegistry != nil {
		if err := d.mcpRegistry.Start(ctx); err != nil {
			d.logger.Warn("MCP registry start error",
				internallog.Error(err))
		} else {
			summary := d.mcpRegistry.GetSummary()
			d.logger.Info("MCP registry started",
				slog.Int("configured", summary.Total),
				slog.Int("running", summary.Running))
		}
	}

	// Create and start public API server if enabled
	var publicErrCh chan error
	if d.cfg.Daemon.Listen.PublicAPI.Enabled {
		publicRouter := api.NewPublicRouter()
		d.publicServer = publicapi.New(
			d.cfg.Daemon.Listen.PublicAPI,
			publicRouter.Handler(),
			internallog.WithComponent(d.logger, "public-api"),
		)

		publicErrCh = make(chan error, 1)
		go func() {
			if err := d.publicServer.Start(ctx); err != nil {
				publicErrCh <- fmt.Errorf("public API server error: %w", err)
			}
			close(publicErrCh)
		}()
	}

	// Start control plane server
	errCh := make(chan error, 1)
	go func() {
		if err := d.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or error from either server
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	case err := <-publicErrCh:
		// Public API error - also an error
		return err
	}
}

// Shutdown gracefully shuts down the daemon.
func (d *Daemon) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return nil
	}

	// Start draining: log the drain start with active workflow count
	activeCount := d.runner.ActiveRunCount()
	d.logger.Info("graceful shutdown initiated",
		slog.Int("active_workflows", activeCount))

	// Put runner into draining mode to stop accepting new workflows
	d.runner.StartDraining()

	// Stop accepting new connections (disable keep-alive)
	if d.server != nil {
		d.server.SetKeepAlivesEnabled(false)
	}

	// Wait for active workflows to complete (with drain timeout)
	drainCtx, drainCancel := context.WithTimeout(ctx, d.cfg.Daemon.DrainTimeout)
	defer drainCancel()

	if err := d.runner.WaitForDrain(drainCtx, d.cfg.Daemon.DrainTimeout); err != nil {
		remainingCount := d.runner.ActiveRunCount()
		d.logger.Warn("drain timeout exceeded",
			slog.Int("remaining_workflows", remainingCount),
			slog.Duration("drain_timeout", d.cfg.Daemon.DrainTimeout))
	} else {
		d.logger.Info("all workflows completed during drain")
	}

	// Stop leader election
	if d.leader != nil {
		d.leader.Stop()
	}

	// Stop scheduler
	if d.scheduler != nil {
		d.scheduler.Stop()
	}

	// Stop MCP registry
	if d.mcpRegistry != nil {
		if err := d.mcpRegistry.Stop(); err != nil {
			d.logger.Error("MCP registry shutdown error",
				internallog.Error(err))
		}
	}

	// Shutdown public API server first (if enabled)
	if d.publicServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, d.cfg.Daemon.ShutdownTimeout)
		defer cancel()

		if err := d.publicServer.Shutdown(shutdownCtx); err != nil {
			d.logger.Error("public API server shutdown error",
				internallog.Error(err))
		}
	}

	// Shutdown control plane HTTP server
	if d.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, d.cfg.Daemon.ShutdownTimeout)
		defer cancel()

		if err := d.server.Shutdown(shutdownCtx); err != nil {
			d.logger.Error("HTTP server shutdown error",
				internallog.Error(err))
		}
	}

	// Clean up PID file
	if d.pidFile != "" {
		if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
			d.logger.Error("failed to remove PID file",
				internallog.Error(err),
				slog.String("path", d.pidFile))
		}
	}

	// Clean up Unix socket file if it exists
	if d.cfg.Daemon.Listen.SocketPath != "" {
		if err := os.Remove(d.cfg.Daemon.Listen.SocketPath); err != nil && !os.IsNotExist(err) {
			d.logger.Error("failed to remove socket file",
				internallog.Error(err),
				slog.String("path", d.cfg.Daemon.Listen.SocketPath))
		}
	}

	// Shutdown OpenTelemetry provider
	if d.otelProvider != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := d.otelProvider.Shutdown(shutdownCtx); err != nil {
			d.logger.Error("OpenTelemetry provider shutdown error",
				internallog.Error(err))
		}
	}

	// Close backend
	if d.backend != nil {
		if err := d.backend.Close(); err != nil {
			d.logger.Error("failed to close backend",
				internallog.Error(err))
		}
	}

	d.started = false
	d.logger.Info("daemon stopped")
	return nil
}

// checkPermissionsAtStartup checks critical paths for insecure permissions and logs warnings.
func (d *Daemon) checkPermissionsAtStartup() {
	pathsToCheck := []string{}

	// Check data directory (contains checkpoints, state files)
	if d.cfg.Daemon.DataDir != "" {
		pathsToCheck = append(pathsToCheck, d.cfg.Daemon.DataDir)
	}

	// Check PID file directory
	if d.cfg.Daemon.PIDFile != "" {
		pidDir := filepath.Dir(d.cfg.Daemon.PIDFile)
		pathsToCheck = append(pathsToCheck, pidDir)
	}

	// Check workflows directory (may contain sensitive configurations)
	if d.cfg.Daemon.WorkflowsDir != "" {
		pathsToCheck = append(pathsToCheck, d.cfg.Daemon.WorkflowsDir)
	}

	// Check each path and log warnings
	for _, path := range pathsToCheck {
		warnings := security.CheckConfigPermissions(path)
		for _, warning := range warnings {
			d.logger.Warn("security warning",
				slog.String("warning", warning))
		}
	}
}

// writePIDFile writes the current process ID to the PID file.
func (d *Daemon) writePIDFile() error {
	// Create parent directory with restrictive permissions (0700)
	dir := filepath.Dir(d.cfg.Daemon.PIDFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write PID with 0600 permissions (owner-only access)
	pid := os.Getpid()
	return os.WriteFile(d.cfg.Daemon.PIDFile, []byte(fmt.Sprintf("%d\n", pid)), 0600)
}

// mcpStatusAdapter adapts mcp.Registry to api.MCPStatusProvider.
type mcpStatusAdapter struct {
	registry *mcp.Registry
}

// GetSummary returns the MCP server summary.
func (a *mcpStatusAdapter) GetSummary() api.MCPServerSummary {
	summary := a.registry.GetSummary()
	return api.MCPServerSummary{
		Total:   summary.Total,
		Running: summary.Running,
		Stopped: summary.Stopped,
		Error:   summary.Error,
	}
}

// observabilityToTracingConfig converts config.ObservabilityConfig to tracing.Config.
func observabilityToTracingConfig(obs config.ObservabilityConfig, version string) tracing.Config {
	cfg := tracing.Config{
		Enabled:        obs.Enabled,
		ServiceName:    obs.ServiceName,
		ServiceVersion: obs.ServiceVersion,
		Sampling: tracing.SamplingConfig{
			Enabled:            obs.Sampling.Enabled,
			Type:               obs.Sampling.Type,
			Rate:               obs.Sampling.Rate,
			AlwaysSampleErrors: obs.Sampling.AlwaysSampleErrors,
		},
		Storage: tracing.StorageConfig{
			Backend: obs.Storage.Backend,
			Path:    obs.Storage.Path,
			Retention: tracing.RetentionConfig{
				Traces:     time.Duration(obs.Storage.Retention.TraceDays) * 24 * time.Hour,
				Events:     time.Duration(obs.Storage.Retention.EventDays) * 24 * time.Hour,
				Aggregates: time.Duration(obs.Storage.Retention.AggregateDays) * 24 * time.Hour,
			},
		},
		Redaction: tracing.RedactionConfig{
			Level:    obs.Redaction.Level,
			Patterns: convertRedactionPatterns(obs.Redaction.Patterns),
		},
	}

	// Convert exporters
	cfg.Exporters = make([]tracing.ExporterConfig, len(obs.Exporters))
	for i, exp := range obs.Exporters {
		cfg.Exporters[i] = tracing.ExporterConfig{
			Type:     exp.Type,
			Endpoint: exp.Endpoint,
			Headers:  exp.Headers,
			TLS: tracing.TLSConfig{
				Enabled:           exp.TLS.Enabled,
				VerifyCertificate: exp.TLS.VerifyCertificate,
				CACertPath:        exp.TLS.CACertPath,
			},
			Timeout: time.Duration(exp.TimeoutSeconds) * time.Second,
		}
	}

	// Use build version if service version not set
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = version
	}

	return cfg
}

// convertRedactionPatterns converts config redaction patterns to tracing patterns.
func convertRedactionPatterns(patterns []config.RedactionPattern) []tracing.RedactionPattern {
	result := make([]tracing.RedactionPattern, len(patterns))
	for i, p := range patterns {
		result[i] = tracing.RedactionPattern{
			Name:        p.Name,
			Regex:       p.Regex,
			Replacement: p.Replacement,
		}
	}
	return result
}
