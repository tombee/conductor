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
	"github.com/tombee/conductor/internal/controller/api"
	"github.com/tombee/conductor/internal/controller/auth"
	"github.com/tombee/conductor/internal/controller/backend"
	"github.com/tombee/conductor/internal/controller/backend/memory"
	"github.com/tombee/conductor/internal/controller/backend/postgres"
	"github.com/tombee/conductor/internal/controller/checkpoint"
	"github.com/tombee/conductor/internal/controller/endpoint"
	"github.com/tombee/conductor/internal/controller/github"
	"github.com/tombee/conductor/internal/controller/leader"
	"github.com/tombee/conductor/internal/controller/listener"
	"github.com/tombee/conductor/internal/controller/publicapi"
	controllerremote "github.com/tombee/conductor/internal/controller/remote"
	"github.com/tombee/conductor/internal/controller/runner"
	"github.com/tombee/conductor/internal/controller/scheduler"
	"github.com/tombee/conductor/internal/controller/webhook"
	internalllm "github.com/tombee/conductor/internal/llm"
	internallog "github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/internal/tracing/audit"
	"github.com/tombee/conductor/pkg/llm/cost"
	"github.com/tombee/conductor/pkg/security"
	securityaudit "github.com/tombee/conductor/pkg/security/audit"
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
	mcpLogCapture   *mcp.LogCapture
	otelProvider    *tracing.OTelProvider
	retentionMgr    *tracing.RetentionManager
	auditLogger     *audit.Logger

	// Security components
	dnsMonitor          *security.DNSQueryMonitor
	overrideManager     *security.OverrideManager
	metricsCollector    *security.MetricsCollector
	rotatingDestination interface{} // Will be *securityaudit.RotatingFileDestination when enabled
	securityManager     security.Manager
	overrideStopChan    chan struct{}

	mu           sync.Mutex
	started      bool
	lastActivity time.Time
	autoStarted  bool
}

// New creates a new daemon instance.
func New(cfg *config.Config, opts Options) (*Daemon, error) {
	// Validate that workflows requiring public API have it enabled
	if err := config.ValidatePublicAPIRequirements(cfg); err != nil {
		return nil, err
	}

	// Create logger with daemon component context
	// Use daemon-specific log configuration if available, otherwise fall back to global log config
	level := cfg.Daemon.DaemonLog.Level
	if level == "" {
		level = cfg.Log.Level
	}
	format := cfg.Daemon.DaemonLog.Format
	if format == "" {
		format = cfg.Log.Format
	}
	logCfg := &internallog.Config{
		Level:  level,
		Format: internallog.Format(format),
		Output: os.Stderr,
	}
	logger := internallog.WithComponent(internallog.New(logCfg), "daemon")

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

	// Create cost store for LLM cost tracking
	costStore := cost.NewMemoryStore()

	// Create runner with configured concurrency and cost tracking
	r := runner.New(runner.Config{
		MaxParallel:    cfg.Daemon.MaxConcurrentRuns,
		DefaultTimeout: cfg.Daemon.DefaultTimeout,
	}, be, cm, runner.WithCostStore(costStore))

	// Create remote workflow fetcher
	// This enables remote workflow support (github:user/repo)
	fetcher, err := controllerremote.NewFetcher(controllerremote.Config{
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
			// Create the workflow executor adapter with cost tracking
			providerAdapter := internalllm.NewProviderAdapter(llmProvider)
			providerAdapter.SetCostStore(costStore)
			// TODO: Wire up tool registry once tool types are unified
			// For now, pass nil as tool registry (like CLI does)
			executor := workflow.NewExecutor(nil, providerAdapter)
			executionAdapter := runner.NewExecutorAdapter(executor)
			executionAdapter.SetCostStore(costStore)
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

	// Prepare API keys for auth middleware (will be initialized later)
	apiKeys := make([]auth.APIKey, len(cfg.Daemon.DaemonAuth.APIKeys))
	for i, key := range cfg.Daemon.DaemonAuth.APIKeys {
		apiKeys[i] = auth.APIKey{
			Key:       key,
			Name:      fmt.Sprintf("key-%d", i+1),
			CreatedAt: time.Now(),
		}
	}

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

	// Create shared MCP log capture
	mcpLogCapture := mcp.NewLogCapture()

	// Create MCP server registry
	mcpRegistry, err := mcp.NewRegistry(mcp.RegistryConfig{
		Logger:     logger,
		LogCapture: mcpLogCapture,
	})
	if err != nil {
		// MCP registry is optional - log warning but continue
		logger.Warn("failed to initialize MCP registry",
			internallog.Error(err))
		logger.Warn("MCP server management will not be available")
	}

	// Initialize OpenTelemetry provider for metrics and tracing
	var otelProvider *tracing.OTelProvider
	var retentionMgr *tracing.RetentionManager
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

			// Create retention manager if trace storage is configured and retention is non-zero
			if otelProvider.GetStore() != nil && tracingCfg.Storage.Retention.Traces > 0 {
				cleanupInterval := 1 * time.Hour // Default cleanup interval
				if cfg.Daemon.Observability.Storage.Retention.CleanupInterval > 0 {
					cleanupInterval = time.Duration(cfg.Daemon.Observability.Storage.Retention.CleanupInterval) * time.Hour
				}

				retentionMgr = tracing.NewRetentionManager(
					otelProvider.GetStore(),
					tracingCfg.Storage.Retention.Traces,
					cleanupInterval,
					logger,
				)
				logger.Info("trace retention manager initialized",
					slog.Duration("max_age", tracingCfg.Storage.Retention.Traces),
					slog.Duration("cleanup_interval", cleanupInterval))
			}
		}
	}

	// Check if this daemon was auto-started
	autoStarted := os.Getenv("CONDUCTOR_AUTO_STARTED") == "1"
	if autoStarted {
		logger.Info("daemon auto-started by CLI",
			slog.Duration("idle_timeout", cfg.Daemon.IdleTimeout))
	}

	// Create audit logger if enabled
	var auditLogger *audit.Logger
	if cfg.Daemon.Observability.Enabled && cfg.Daemon.Observability.Audit.Enabled {
		auditCfg := cfg.Daemon.Observability.Audit
		var err error
		auditLogger, err = audit.NewLoggerFromDestination(auditCfg.Destination, auditCfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit logger: %w", err)
		}
		logger.Info("audit logging enabled",
			slog.String("destination", auditCfg.Destination))
	}

	// Initialize security components
	// Initialize security Manager first
	var securityMgr security.Manager
	if cfg.Security.DefaultProfile != "" || len(cfg.Security.Profiles) > 0 {
		var err error
		securityMgr, err = security.NewManager(&cfg.Security)
		if err != nil {
			logger.Warn("failed to initialize security manager",
				internallog.Error(err))
			logger.Warn("security enforcement will not be available")
		} else {
			logger.Info("security manager initialized",
				slog.String("default_profile", cfg.Security.DefaultProfile))
		}
	}

	// Initialize DNSQueryMonitor if configured
	var dnsMonitor *security.DNSQueryMonitor
	if cfg.Security.DNS.RebindingPrevention || cfg.Security.DNS.BlockDynamicDNS ||
		cfg.Security.DNS.ExfiltrationLimits.MaxQueriesPerMinute > 0 {
		dnsMonitor = security.NewDNSQueryMonitor(cfg.Security.DNS)
		logger.Info("DNS query monitor initialized")
	}

	// Initialize security MetricsCollector and wire to Manager
	var metricsCollector *security.MetricsCollector
	if cfg.Security.Metrics.Enabled {
		metricsCollector = security.NewMetricsCollector()
		logger.Info("security metrics collector initialized")

		// Wire MetricsCollector to security Manager
		if securityMgr != nil {
			securityMgr.SetMetricsCollector(metricsCollector)
			logger.Info("security metrics collector wired to security manager")
		}
	}

	// Initialize OverrideManager with event logger
	var overrideManager *security.OverrideManager
	if cfg.Security.Override.Enabled {
		// Create event logger for override auditing
		eventLogger := security.NewEventLogger(cfg.Security.Audit)
		overrideManager = security.NewOverrideManager(eventLogger)
		logger.Info("security override manager initialized with event logger")
	}

	// Audit rotation is handled by EventLogger
	// The rotation destination is created automatically when Rotation.Enabled is true
	var rotatingDest interface{}
	if cfg.Security.Audit.Rotation.Enabled && cfg.Security.Audit.Enabled {
		logger.Info("audit rotation configured",
			slog.Int64("max_size_mb", cfg.Security.Audit.Rotation.MaxSizeMB),
			slog.Int("max_age_days", cfg.Security.Audit.Rotation.MaxAgeDays),
			slog.Bool("compress", cfg.Security.Audit.Rotation.Compress))
	}

	// Create auth middleware (after security components are initialized)
	authMw := auth.NewMiddleware(auth.Config{
		Enabled:         cfg.Daemon.DaemonAuth.Enabled,
		APIKeys:         apiKeys,
		AllowUnixSocket: cfg.Daemon.DaemonAuth.AllowUnixSocket,
		OverrideManager: overrideManager,
		Logger:          logger,
	})


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
		mcpLogCapture:   mcpLogCapture,
		otelProvider:    otelProvider,
		retentionMgr:    retentionMgr,
		auditLogger:     auditLogger,
		lastActivity:    time.Now(),
		autoStarted:     autoStarted,

		// Security components
		dnsMonitor:          dnsMonitor,
		metricsCollector:    metricsCollector,
		overrideManager:     overrideManager,
		rotatingDestination: rotatingDest,
		securityManager:     securityMgr,
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

	// Create cancellable context for idle timeout monitoring
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start idle timeout monitor for auto-started daemons
	d.startIdleTimeoutMonitor(ctx, cancel)

	// Check permissions on critical directories and files at startup
	d.checkPermissionsAtStartup()

	// Log security warnings for risky configurations
	d.logSecurityWarnings()

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

	// Wire up activity recorder for idle timeout tracking
	router.SetActivityRecorder(d)

	// Register runs API
	runsHandler := api.NewRunsHandler(d.runner)
	runsHandler.RegisterRoutes(router.Mux())

	// Register trigger API
	triggerHandler := api.NewTriggerHandler(d.runner, d.cfg.Daemon.WorkflowsDir)
	triggerHandler.RegisterRoutes(router.Mux())

	// Register webhook routes on control plane only if public API is disabled
	// When public API is enabled, webhooks are only available on the public API port
	// When public API is disabled, webhooks from config are available on control plane
	if !d.cfg.Daemon.Listen.PublicAPI.Enabled {
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
	}

	// Register schedules API
	schedulesHandler := api.NewSchedulesHandler(d.scheduler)
	schedulesHandler.RegisterRoutes(router.Mux())

	// Register endpoint routes if enabled
	if d.endpointHandler != nil {
		d.endpointHandler.RegisterRoutes(router.Mux())
	}

	// Register MCP API if registry is available
	if d.mcpRegistry != nil {
		mcpHandler := api.NewMCPHandler(d.mcpRegistry, d.mcpLogCapture)
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

	// Wire up audit status provider to router for health endpoint
	router.SetAuditProvider(&auditStatusAdapter{cfg: d.cfg})

	// Wire up metrics handler if observability is enabled
	// Combine OTel metrics with security metrics if both are available
	if d.otelProvider != nil {
		var metricsHandler http.Handler
		if d.metricsCollector != nil {
			// Create combined handler with both OTel and security metrics
			metricsHandler = NewCombinedMetricsHandler(d.otelProvider.MetricsHandler(), d.metricsCollector)
		} else {
			// Use OTel metrics only
			metricsHandler = d.otelProvider.MetricsHandler()
		}
		router.SetMetricsHandler(metricsHandler)
	} else if d.metricsCollector != nil {
		// If only security metrics are available (no OTel), create a simple handler
		metricsHandler := NewCombinedMetricsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No OTel metrics, empty base
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		}), d.metricsCollector)
		router.SetMetricsHandler(metricsHandler)
	}

	// Wire up override management handler if enabled
	if d.overrideManager != nil {
		overrideHandler := api.NewOverrideHandler(d.overrideManager)
		router.SetOverrideHandler(overrideHandler)
	}

	// Create HTTP server with middleware chain
	// Middleware order (from outer to inner):
	// 1. Auth middleware (validates credentials, sets user context)
	// 2. Audit middleware (logs API access using user from context)
	// 3. Router (handles requests)
	var handler http.Handler = router

	// Apply audit middleware if enabled (before auth so it can log auth failures too)
	// Actually, apply after auth so we have user context
	if d.auditLogger != nil {
		trustedProxies := d.cfg.Daemon.Observability.Audit.TrustedProxies
		handler = audit.Middleware(d.auditLogger, trustedProxies)(handler)
	}

	// Apply auth middleware (outer layer)
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

	// Start trace retention manager if configured
	if d.retentionMgr != nil {
		d.retentionMgr.Start()
		d.logger.Info("trace retention manager started")
	}

	// Start OverrideManager cleanup goroutine
	if d.overrideManager != nil {
		d.overrideStopChan = make(chan struct{})
		go d.overrideManager.StartAutoCleanup(d.overrideStopChan)
		d.logger.Info("security override cleanup goroutine started")
	}

	// Create and start public API server if enabled
	var publicErrCh chan error
	if d.cfg.Daemon.Listen.PublicAPI.Enabled {
		publicRouter := api.NewPublicRouter(api.PublicRouterConfig{
			Runner:       d.runner,
			WorkflowsDir: d.cfg.Daemon.WorkflowsDir,
		})
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

	// Stop runner and wait for all goroutines to exit
	// Use remaining shutdown context time for runner stop
	if err := d.runner.Stop(ctx); err != nil {
		d.logger.Warn("runner stop timeout",
			internallog.Error(err))
	} else {
		d.logger.Info("runner stopped cleanly")
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

	// T7: Stop OverrideManager cleanup goroutine
	if d.overrideStopChan != nil {
		close(d.overrideStopChan)
		d.logger.Info("security override cleanup goroutine stopped")
	}

	// T7: Close audit logger via security manager
	if d.securityManager != nil {
		if err := d.securityManager.Close(); err != nil {
			d.logger.Error("security manager shutdown error",
				internallog.Error(err))
		} else {
			d.logger.Info("security manager shutdown complete")
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

	// Stop retention manager before shutting down trace storage
	if d.retentionMgr != nil {
		d.retentionMgr.Stop()
		d.logger.Info("trace retention manager stopped")
	}

	// Flush pending spans before shutdown
	if d.otelProvider != nil {
		flushCtx, flushCancel := context.WithTimeout(ctx, 10*time.Second)
		defer flushCancel()
		if err := d.otelProvider.ForceFlush(flushCtx); err != nil {
			d.logger.Warn("failed to flush pending spans",
				internallog.Error(err))
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

	// Close audit logger
	if d.auditLogger != nil {
		if err := d.auditLogger.Close(); err != nil {
			d.logger.Error("failed to close audit logger",
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

// logSecurityWarnings logs warnings for risky security configurations.
func (d *Daemon) logSecurityWarnings() {
	// If force-insecure is set, log a single warning and skip detailed checks
	if d.cfg.Daemon.ForceInsecure {
		d.logger.Warn("security: running with --force-insecure flag",
			slog.String("warning", "security warnings suppressed - not recommended for production"))
		return
	}

	// Warn if authentication is disabled
	if !d.cfg.Daemon.DaemonAuth.Enabled {
		d.logger.Warn("security: authentication is disabled",
			slog.String("recommendation", "enable daemon_auth.enabled for production use"))

		// Extra warning if listening on non-localhost TCP
		if d.cfg.Daemon.Listen.TCPAddr != "" && !isLocalhostAddr(d.cfg.Daemon.Listen.TCPAddr) {
			d.logger.Warn("security: authentication disabled on network-accessible address",
				slog.String("tcp_addr", d.cfg.Daemon.Listen.TCPAddr),
				slog.String("risk", "unauthenticated API access from network"))
		}

		// Warning if public API is enabled without auth
		if d.cfg.Daemon.Listen.PublicAPI.Enabled {
			d.logger.Warn("security: public API enabled without authentication",
				slog.String("public_api_tcp", d.cfg.Daemon.Listen.PublicAPI.TCP),
				slog.String("risk", "publicly accessible unauthenticated webhooks"))
		}
	}

	// Warn if TLS is not enabled for TCP listener
	if d.cfg.Daemon.Listen.TCPAddr != "" && d.cfg.Daemon.Listen.TLSCert == "" {
		if !isLocalhostAddr(d.cfg.Daemon.Listen.TCPAddr) {
			d.logger.Warn("security: TLS not configured for network listener",
				slog.String("tcp_addr", d.cfg.Daemon.Listen.TCPAddr),
				slog.String("recommendation", "configure tls_cert and tls_key for encrypted connections"))
		}
	}
}

// isLocalhostAddr returns true if the address is localhost-only.
func isLocalhostAddr(addr string) bool {
	// Parse host from address (host:port format)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Try without port
		host = addr
	}

	// Empty host or 127.x.x.x is localhost
	if host == "" || host == "localhost" {
		return true
	}

	// Parse as IP to check for loopback
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback()
	}

	return false
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
		Running:summary.Running,
		Stopped: summary.Stopped,
		Error:   summary.Error,
	}
}

// auditStatusAdapter provides audit rotation status for health checks.
type auditStatusAdapter struct {
	cfg *config.Config
}

// GetAuditRotationStatus returns the audit rotation status.
func (a *auditStatusAdapter) GetAuditRotationStatus() api.AuditRotationStatus {
	if !a.cfg.Security.Audit.Enabled {
		return api.AuditRotationStatus{
			Enabled: false,
			Status:  "audit disabled",
		}
	}

	if !a.cfg.Security.Audit.Rotation.Enabled {
		return api.AuditRotationStatus{
			Enabled: false,
			Status:  "rotation disabled",
		}
	}

	// Find the file path for rotation
	basePath := ""
	for _, dest := range a.cfg.Security.Audit.Destinations {
		if dest.Type == "file" {
			basePath = dest.Path
			break
		}
	}

	if basePath == "" {
		basePath = "~/.conductor/logs/audit.log"
	}

	// List rotated logs to get status
	logs, err := securityaudit.ListRotatedLogs(basePath)
	if err != nil {
		return api.AuditRotationStatus{
			Enabled: true,
			Status:  "error reading logs",
		}
	}

	var totalSize int64
	for _, log := range logs {
		totalSize += log.Size
	}

	return api.AuditRotationStatus{
		Enabled:      true,
		CurrentFiles: len(logs),
		TotalSize:    totalSize,
		Status:       "ok",
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
		BatchSize:     obs.BatchSize,
		BatchInterval: time.Duration(obs.BatchInterval) * time.Second,
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

// RecordActivity updates the last activity timestamp.
// This should be called on every incoming API request.
func (d *Daemon) RecordActivity() {
	d.mu.Lock()
	d.lastActivity = time.Now()
	d.mu.Unlock()
}

// timeSinceLastActivity returns duration since last activity.
func (d *Daemon) timeSinceLastActivity() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return time.Since(d.lastActivity)
}

// startIdleTimeoutMonitor starts a goroutine that monitors idle timeout.
// Only applies to auto-started daemons.
func (d *Daemon) startIdleTimeoutMonitor(ctx context.Context, cancel context.CancelFunc) {
	if !d.autoStarted || d.cfg.Daemon.IdleTimeout == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				idle := d.timeSinceLastActivity()
				if idle >= d.cfg.Daemon.IdleTimeout {
					d.logger.Info("shutting down due to idle timeout",
						slog.Duration("idle_duration", idle),
						slog.Duration("idle_timeout", d.cfg.Daemon.IdleTimeout))
					cancel()
					return
				}
			}
		}
	}()
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
