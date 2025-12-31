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

package controller

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/tombee/conductor/internal/controller/debug"
	"github.com/tombee/conductor/internal/controller/endpoint"
	"github.com/tombee/conductor/internal/controller/filewatcher"
	"github.com/tombee/conductor/internal/controller/github"
	"github.com/tombee/conductor/internal/controller/leader"
	"github.com/tombee/conductor/internal/controller/listener"
	"github.com/tombee/conductor/internal/controller/polltrigger"
	"github.com/tombee/conductor/internal/controller/publicapi"
	controllerremote "github.com/tombee/conductor/internal/controller/remote"
	"github.com/tombee/conductor/internal/controller/runner"
	"github.com/tombee/conductor/internal/controller/scheduler"
	"github.com/tombee/conductor/internal/controller/trigger"
	"github.com/tombee/conductor/internal/controller/webhook"
	internalllm "github.com/tombee/conductor/internal/llm"
	internallog "github.com/tombee/conductor/internal/log"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/internal/tracing/audit"
	"github.com/tombee/conductor/internal/triggers"
	"github.com/tombee/conductor/pkg/security"
	securityaudit "github.com/tombee/conductor/pkg/security/audit"
	"github.com/tombee/conductor/pkg/workflow"
)

// Options contains controller options set at build time.
type Options struct {
	Version   string
	Commit    string
	BuildDate string
}

// Controller is the main conductor controller service.
type Controller struct {
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
	fileWatcher     *filewatcher.Service
	endpointHandler *endpoint.Handler
	authMw          *auth.Middleware
	leader          *leader.Elector
	mcpRegistry     *mcp.Registry
	mcpLogCapture   *mcp.LogCapture
	otelProvider    *tracing.OTelProvider
	retentionMgr    *tracing.RetentionManager
	auditLogger        *audit.Logger
	pollTriggerService *polltrigger.Service
	debugSessionMgr    *debug.SessionManager

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

// New creates a new controller instance.
func New(cfg *config.Config, opts Options) (*Controller, error) {
	// Validate that workflows requiring public API have it enabled
	if err := config.ValidatePublicAPIRequirements(cfg); err != nil {
		return nil, err
	}

	// Create logger with controller component context
	// Use controller-specific log configuration if available, otherwise fall back to global log config
	level := cfg.Controller.ControllerLog.Level
	if level == "" {
		level = cfg.Log.Level
	}
	format := cfg.Controller.ControllerLog.Format
	if format == "" {
		format = cfg.Log.Format
	}
	logCfg := &internallog.Config{
		Level:  level,
		Format: internallog.Format(format),
		Output: os.Stderr,
	}
	logger := internallog.WithComponent(internallog.New(logCfg), "controller")

	// Create backend based on configuration
	var be backend.Backend
	var db *sql.DB

	switch cfg.Controller.Backend.Type {
	case "postgres":
		pgCfg := postgres.Config{
			ConnectionString: cfg.Controller.Backend.Postgres.ConnectionString,
			MaxOpenConns:     cfg.Controller.Backend.Postgres.MaxOpenConns,
			MaxIdleConns:     cfg.Controller.Backend.Postgres.MaxIdleConns,
			ConnMaxLifetime:  time.Duration(cfg.Controller.Backend.Postgres.ConnMaxLifetimeSeconds) * time.Second,
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
		Dir: cfg.Controller.CheckpointDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create checkpoint manager: %w", err)
	}

	// Create runner with configured concurrency
	r := runner.New(runner.Config{
		MaxParallel:    cfg.Controller.MaxConcurrentRuns,
		DefaultTimeout: cfg.Controller.DefaultTimeout,
	}, be, cm)

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
			// Create the workflow executor adapter
			providerAdapter := internalllm.NewProviderAdapter(llmProvider)
			// Tool registry is nil; tools are resolved dynamically per-workflow.
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
	if cfg.Controller.Schedules.Enabled && len(cfg.Controller.Schedules.Schedules) > 0 {
		schedules := make([]scheduler.Schedule, len(cfg.Controller.Schedules.Schedules))
		for i, s := range cfg.Controller.Schedules.Schedules {
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
			WorkflowsDir: cfg.Controller.WorkflowsDir,
		}, r)
		if err != nil {
			return nil, fmt.Errorf("failed to create scheduler: %w", err)
		}
	}

	// Create file watcher service if enabled
	var fileWatcherSvc *filewatcher.Service
	if cfg.Controller.FileWatchers.Enabled {
		fileWatcherSvc = filewatcher.NewService(cfg.Controller.WorkflowsDir, r)
		logger.Info("file watcher service created",
			slog.Int("watcher_count", len(cfg.Controller.FileWatchers.Watchers)))
	}

	// Create endpoint handler if enabled
	var endpointHandler *endpoint.Handler
	if cfg.Controller.Endpoints.Enabled {
		// Create rate limiter for endpoints
		rateLimiter := auth.NewNamedRateLimiter()

		// Load endpoint configuration with rate limiter
		registry, err := endpoint.LoadConfig(cfg.Controller.Endpoints, cfg.Controller.WorkflowsDir, rateLimiter)
		if err != nil {
			return nil, fmt.Errorf("failed to load endpoints: %w", err)
		}

		// Create handler and wire in rate limiter
		endpointHandler = endpoint.NewHandler(registry, r, cfg.Controller.WorkflowsDir)
		endpointHandler.SetRateLimiter(rateLimiter)

		logger.Info("endpoints loaded",
			slog.Int("count", registry.Count()))
	}

	// Prepare API keys for auth middleware (will be initialized later)
	apiKeys := make([]auth.APIKey, len(cfg.Controller.ControllerAuth.APIKeys))
	for i, key := range cfg.Controller.ControllerAuth.APIKeys {
		apiKeys[i] = auth.APIKey{
			Key:       key,
			Name:      fmt.Sprintf("key-%d", i+1),
			CreatedAt: time.Now(),
		}
	}

	// Create leader elector if distributed mode is enabled
	var elector *leader.Elector
	if cfg.Controller.Distributed.Enabled && db != nil {
		instanceID := cfg.Controller.Distributed.InstanceID
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
	if cfg.Controller.Observability.Enabled {
		tracingCfg := observabilityToTracingConfig(cfg.Controller.Observability, opts.Version)
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
			mc := otelProvider.MetricsCollector()
			r.SetMetrics(mc)

			// Create retention manager if trace storage is configured and retention is non-zero
			if otelProvider.GetStore() != nil && tracingCfg.Storage.Retention.Traces > 0 {
				cleanupInterval := 1 * time.Hour // Default cleanup interval
				if cfg.Controller.Observability.Storage.Retention.CleanupInterval > 0 {
					cleanupInterval = time.Duration(cfg.Controller.Observability.Storage.Retention.CleanupInterval) * time.Hour
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

	// Initialize debug session manager if observability storage is available
	var debugSessionMgr *debug.SessionManager
	if otelProvider != nil && otelProvider.GetStore() != nil {
		debugSessionMgr = debug.NewSessionManager(debug.SessionManagerConfig{
			Store: otelProvider.GetStore(),
		})
		logger.Info("debug session manager initialized")
	}

	// Check if this controller was auto-started
	autoStarted := os.Getenv("CONDUCTOR_AUTO_STARTED") == "1"
	if autoStarted {
		logger.Info("controller auto-started by CLI",
			slog.Duration("idle_timeout", cfg.Controller.IdleTimeout))
	}

	// Create audit logger if enabled
	var auditLogger *audit.Logger
	if cfg.Controller.Observability.Enabled && cfg.Controller.Observability.Audit.Enabled {
		auditCfg := cfg.Controller.Observability.Audit
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
		Enabled:         cfg.Controller.ControllerAuth.Enabled,
		APIKeys:         apiKeys,
		AllowUnixSocket: cfg.Controller.ControllerAuth.AllowUnixSocket,
		OverrideManager: overrideManager,
		Logger:          logger,
	})

	// Create poll trigger service
	var pollTriggerSvc *polltrigger.Service
	pollTriggerSvc, err = polltrigger.NewService(polltrigger.ServiceConfig{
		Logger: logger,
		WorkflowFirer: func(ctx context.Context, workflowPath string, triggerContext *polltrigger.PollTriggerContext) error {
			// Fire workflow via runner
			// Read workflow YAML
			workflowYAML, err := os.ReadFile(workflowPath)
			if err != nil {
				return fmt.Errorf("failed to read workflow: %w", err)
			}

			// Convert trigger context to workflow inputs
			inputs := make(map[string]interface{})
			inputs["trigger"] = triggerContext

			// Execute workflow
			_, err = r.Submit(ctx, runner.SubmitRequest{
				WorkflowYAML: workflowYAML,
				Inputs:       inputs,
			})
			return err
		},
	})
	if err != nil {
		logger.Warn("failed to create poll trigger service",
			internallog.Error(err))
		logger.Warn("poll triggers will not be available")
	} else {
		// Register PagerDuty poller if token is configured
		pagerdutyToken := os.Getenv("PAGERDUTY_TOKEN")
		if pagerdutyToken != "" {
			pdPoller := polltrigger.NewPagerDutyPoller(pagerdutyToken)
			if err := pollTriggerSvc.RegisterPoller(pdPoller); err != nil {
				logger.Warn("failed to register PagerDuty poller",
					internallog.Error(err))
			}
		}

		// Register Datadog poller if API key and app key are configured
		datadogAPIKey := os.Getenv("DATADOG_API_KEY")
		datadogAppKey := os.Getenv("DATADOG_APP_KEY")
		if datadogAPIKey != "" && datadogAppKey != "" {
			datadogSite := os.Getenv("DATADOG_SITE") // Optional, defaults to datadoghq.com
			ddPoller := polltrigger.NewDatadogPoller(datadogAPIKey, datadogAppKey, datadogSite)
			if err := pollTriggerSvc.RegisterPoller(ddPoller); err != nil {
				logger.Warn("failed to register Datadog poller",
					internallog.Error(err))
			}
		}

		// Register Jira poller if credentials are configured
		jiraEmail := os.Getenv("JIRA_EMAIL")
		jiraAPIToken := os.Getenv("JIRA_API_TOKEN")
		jiraBaseURL := os.Getenv("JIRA_BASE_URL")
		if jiraEmail != "" && jiraAPIToken != "" && jiraBaseURL != "" {
			jiraPoller := polltrigger.NewJiraPoller(jiraEmail, jiraAPIToken, jiraBaseURL)
			if err := pollTriggerSvc.RegisterPoller(jiraPoller); err != nil {
				logger.Warn("failed to register Jira poller",
					internallog.Error(err))
			}
		}

		// Register Slack poller if bot token is configured
		slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
		if slackBotToken != "" {
			slackPoller := polltrigger.NewSlackPoller(slackBotToken)
			if err := pollTriggerSvc.RegisterPoller(slackPoller); err != nil {
				logger.Warn("failed to register Slack poller",
					internallog.Error(err))
			}
		}
	}

	return &Controller{
		cfg:                cfg,
		opts:               opts,
		logger:             logger,
		runner:             r,
		backend:            be,
		checkpoints:        cm,
		scheduler:          sched,
		fileWatcher:        fileWatcherSvc,
		endpointHandler:    endpointHandler,
		authMw:             authMw,
		leader:             elector,
		mcpRegistry:        mcpRegistry,
		mcpLogCapture:      mcpLogCapture,
		otelProvider:       otelProvider,
		retentionMgr:       retentionMgr,
		auditLogger:        auditLogger,
		pollTriggerService: pollTriggerSvc,
		debugSessionMgr:    debugSessionMgr,
		lastActivity:       time.Now(),
		autoStarted:        autoStarted,

		// Security components
		dnsMonitor:          dnsMonitor,
		metricsCollector:    metricsCollector,
		overrideManager:     overrideManager,
		rotatingDestination: rotatingDest,
		securityManager:     securityMgr,
	}, nil
}

// Start starts the controller and blocks until the context is cancelled.
func (c *Controller) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("controller already started")
	}
	c.started = true
	c.mu.Unlock()

	// Create cancellable context for idle timeout monitoring
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start idle timeout monitor for auto-started daemons
	c.startIdleTimeoutMonitor(ctx, cancel)

	// Check permissions on critical directories and files at startup
	c.checkPermissionsAtStartup()

	// Log security warnings for risky configurations
	c.logSecurityWarnings()

	// Write PID file if configured
	if c.cfg.Controller.PIDFile != "" {
		if err := c.writePIDFile(); err != nil {
			return fmt.Errorf("failed to write PID file: %w", err)
		}
		c.pidFile = c.cfg.Controller.PIDFile
	}

	// Resume any interrupted runs from checkpoints
	if err := c.runner.ResumeInterrupted(ctx); err != nil {
		c.logger.Warn("failed to resume interrupted runs",
			internallog.Error(err))
	}

	// Create listener
	ln, err := listener.New(c.cfg.Controller.Listen)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	c.ln = ln

	// Create HTTP router
	router := api.NewRouter(api.RouterConfig{
		Version:   c.opts.Version,
		Commit:    c.opts.Commit,
		BuildDate: c.opts.BuildDate,
	})

	// Wire up activity recorder for idle timeout tracking
	router.SetActivityRecorder(c)

	// Register runs API
	runsHandler := api.NewRunsHandler(c.runner)
	runsHandler.RegisterRoutes(router.Mux())

	// Register trigger API
	triggerHandler := api.NewTriggerHandler(c.runner, c.cfg.Controller.WorkflowsDir)
	triggerHandler.RegisterRoutes(router.Mux())

	// Register webhook routes on control plane only if public API is disabled
	// When public API is enabled, webhooks are only available on the public API port
	// When public API is disabled, webhooks from config are available on control plane
	if !c.cfg.Controller.Listen.PublicAPI.Enabled {
		webhookRoutes := make([]webhook.Route, len(c.cfg.Controller.Webhooks.Routes))
		for i, r := range c.cfg.Controller.Webhooks.Routes {
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
			WorkflowsDir: c.cfg.Controller.WorkflowsDir,
		}, c.runner)
		webhookRouter.RegisterRoutes(router.Mux())
	}

	// Register schedules API
	schedulesHandler := api.NewSchedulesHandler(c.scheduler)
	schedulesHandler.RegisterRoutes(router.Mux())

	// Register endpoint routes if enabled
	if c.endpointHandler != nil {
		c.endpointHandler.RegisterRoutes(router.Mux())
	}

	// Register MCP API if registry is available
	if c.mcpRegistry != nil {
		mcpHandler := api.NewMCPHandler(c.mcpRegistry, c.mcpLogCapture)
		mcpHandler.RegisterRoutes(router.Mux())
	}

	// Register traces, events, and debug API if observability storage is available
	if c.otelProvider != nil && c.otelProvider.GetStore() != nil {
		store := c.otelProvider.GetStore()
		tracesHandler := api.NewTracesHandler(store)
		tracesHandler.RegisterRoutes(router.Mux())
		eventsHandler := api.NewEventsHandler(store)
		eventsHandler.RegisterRoutes(router.Mux())

		// Register debug API if session manager is available
		if c.debugSessionMgr != nil {
			debugHandler := api.NewDebugHandler(c.debugSessionMgr)
			debugHandler.RegisterRoutes(router.Mux())
		}
	}

	// Wire up scheduler to router for health endpoint
	if c.scheduler != nil {
		router.SetScheduleProvider(c.scheduler)
	}

	// Wire up MCP registry to router for health endpoint
	if c.mcpRegistry != nil {
		router.SetMCPProvider(&mcpStatusAdapter{registry: c.mcpRegistry})
	}

	// Wire up audit status provider to router for health endpoint
	router.SetAuditProvider(&auditStatusAdapter{cfg: c.cfg})

	// Wire up metrics handler if observability is enabled
	// Combine OTel metrics with security metrics if both are available
	if c.otelProvider != nil {
		var metricsHandler http.Handler
		if c.metricsCollector != nil {
			// Create combined handler with both OTel and security metrics
			metricsHandler = NewCombinedMetricsHandler(c.otelProvider.MetricsHandler(), c.metricsCollector)
		} else {
			// Use OTel metrics only
			metricsHandler = c.otelProvider.MetricsHandler()
		}
		router.SetMetricsHandler(metricsHandler)
	} else if c.metricsCollector != nil {
		// If only security metrics are available (no OTel), create a simple handler
		metricsHandler := NewCombinedMetricsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No OTel metrics, empty base
			w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		}), c.metricsCollector)
		router.SetMetricsHandler(metricsHandler)
	}

	// Wire up override management handler if enabled
	if c.overrideManager != nil {
		overrideHandler := api.NewOverrideHandler(c.overrideManager)
		router.SetOverrideHandler(overrideHandler)
	}

	// Wire up trigger management handler
	{
		cfgPath, err := config.ConfigPath()
		if err != nil {
			// Fall back to default if config path cannot be determined
			cfgPath = "~/.config/conductor/config.yaml"
		}
		triggerHandler := api.NewTriggerManagementHandler(triggers.NewManager(
			cfgPath,
			c.cfg.Controller.WorkflowsDir,
		))
		router.SetTriggerManagementHandler(triggerHandler)
	}

	// Create HTTP server with middleware chain
	// Middleware order (from outer to inner):
	// 1. Auth middleware (validates credentials, sets user context)
	// 2. Audit middleware (logs API access using user from context)
	// 3. Router (handles requests)
	var handler http.Handler = router

	// Apply audit middleware if enabled (before auth so it can log auth failures too)
	// Actually, apply after auth so we have user context
	if c.auditLogger != nil {
		trustedProxies := c.cfg.Controller.Observability.Audit.TrustedProxies
		handler = audit.Middleware(c.auditLogger, trustedProxies)(handler)
	}

	// Apply auth middleware (outer layer)
	if c.authMw != nil {
		handler = c.authMw.Wrap(handler)
	}

	c.server = &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Log startup
	c.logger.Info("conductord starting",
		slog.String("version", c.opts.Version),
		slog.String("listen_addr", ln.Addr().String()))

	// Start leader election if in distributed mode
	if c.leader != nil {
		c.leader.Start(ctx)

		// Register callback to start/stop scheduler based on leadership
		if c.scheduler != nil && c.cfg.Controller.Distributed.LeaderElection {
			c.leader.OnLeadershipChange(func(isLeader bool) {
				if isLeader {
					c.scheduler.Start(ctx)
					c.logger.Info("became leader - scheduler started",
						slog.Int("schedule_count", len(c.cfg.Controller.Schedules.Schedules)))
				} else {
					c.scheduler.Stop()
					c.logger.Info("lost leadership - scheduler stopped")
				}
			})
		}
	}

	// Start scheduler if configured (and not using leader election)
	if c.scheduler != nil && (c.leader == nil || !c.cfg.Controller.Distributed.LeaderElection) {
		c.scheduler.Start(ctx)
		c.logger.Info("scheduler started",
			slog.Int("schedule_count", len(c.cfg.Controller.Schedules.Schedules)))
	}

	// Start file watcher service if configured
	if c.fileWatcher != nil {
		if err := c.fileWatcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start file watcher service: %w", err)
		}

		// Add configured watchers
		for _, w := range c.cfg.Controller.FileWatchers.Watchers {
			if !w.Enabled {
				continue
			}
			config := filewatcher.WatchConfig{
				Name:     w.Name,
				Workflow: w.Workflow,
				Paths:    w.Paths,
				Events:   w.Events,
				Inputs:   w.Inputs,
			}
			if err := c.fileWatcher.AddWatcher(config); err != nil {
				c.logger.Error("failed to add file watcher",
					slog.String("name", w.Name),
					internallog.Error(err))
			}
		}

		// Scan workflow files for file triggers
		scanner := trigger.NewScanner(c.cfg.Controller.WorkflowsDir)
		scanResult, err := scanner.Scan()
		if err != nil {
			c.logger.Warn("failed to scan workflows for file triggers",
				internallog.Error(err))
		} else {
			// Add file triggers from workflow definitions
			for _, t := range scanResult.FileTriggers {
				if t.File == nil {
					continue
				}

				// Parse debounce duration if specified
				var debounceWindow time.Duration
				if t.File.Debounce != "" {
					debounceWindow, err = time.ParseDuration(t.File.Debounce)
					if err != nil {
						c.logger.Error("invalid debounce duration in workflow file trigger",
							slog.String("workflow", t.WorkflowName),
							slog.String("debounce", t.File.Debounce),
							internallog.Error(err))
						continue
					}
				}

				config := filewatcher.WatchConfig{
					Name:                 fmt.Sprintf("workflow:%s", t.WorkflowName),
					Workflow:             t.WorkflowPath,
					Paths:                t.File.Paths,
					Events:               t.File.Events,
					IncludePatterns:      t.File.IncludePatterns,
					ExcludePatterns:      t.File.ExcludePatterns,
					DebounceWindow:       debounceWindow,
					BatchMode:            t.File.BatchMode,
					MaxTriggersPerMinute: t.File.MaxTriggersPerMinute,
					Recursive:            t.File.Recursive,
					MaxDepth:             t.File.MaxDepth,
					Inputs:               t.File.Inputs,
				}
				if err := c.fileWatcher.AddWatcher(config); err != nil {
					c.logger.Error("failed to add workflow file trigger",
						slog.String("workflow", t.WorkflowName),
						internallog.Error(err))
				} else {
					c.logger.Info("registered file trigger from workflow",
						slog.String("workflow", t.WorkflowName),
						slog.Int("path_count", len(t.File.Paths)))
				}
			}

			// Log any errors from scanning
			for _, scanErr := range scanResult.Errors {
				c.logger.Warn("error scanning workflow for triggers",
					internallog.Error(scanErr))
			}
		}
	}

	// Start MCP registry (auto-starts configured servers)
	if c.mcpRegistry != nil {
		if err := c.mcpRegistry.Start(ctx); err != nil {
			c.logger.Warn("MCP registry start error",
				internallog.Error(err))
		} else {
			summary := c.mcpRegistry.GetSummary()
			c.logger.Info("MCP registry started",
				slog.Int("configured", summary.Total),
				slog.Int("running", summary.Running))
		}
	}

	// Start poll trigger service and scan workflows
	if c.pollTriggerService != nil {
		if err := c.pollTriggerService.Start(ctx); err != nil {
			c.logger.Warn("poll trigger service start error",
				internallog.Error(err))
		} else {
			c.logger.Info("poll trigger service started")
			// Scan workflows directory for poll triggers
			go c.scanAndRegisterPollTriggers(ctx)
		}
	}

	// Start trace retention manager if configured
	if c.retentionMgr != nil {
		c.retentionMgr.Start()
		c.logger.Info("trace retention manager started")
	}

	// Start run cleanup loop for memory management
	runRetention := c.cfg.Controller.RunRetention
	if runRetention == 0 {
		runRetention = 24 * time.Hour // Default to 24 hours
	}
	go c.runner.GetStateManager().StartCleanupLoop(ctx, runRetention, c.logger)
	c.logger.Info("run cleanup loop started",
		slog.Duration("retention", runRetention))

	// Start OverrideManager cleanup goroutine
	if c.overrideManager != nil {
		c.overrideStopChan = make(chan struct{})
		go c.overrideManager.StartAutoCleanup(c.overrideStopChan)
		c.logger.Info("security override cleanup goroutine started")
	}

	// Create and start public API server if enabled
	var publicErrCh chan error
	if c.cfg.Controller.Listen.PublicAPI.Enabled {
		publicRouter := api.NewPublicRouter(api.PublicRouterConfig{
			Runner:       c.runner,
			WorkflowsDir: c.cfg.Controller.WorkflowsDir,
		})
		c.publicServer = publicapi.New(
			c.cfg.Controller.Listen.PublicAPI,
			publicRouter.Handler(),
			internallog.WithComponent(c.logger, "public-api"),
		)

		publicErrCh = make(chan error, 1)
		go func() {
			if err := c.publicServer.Start(ctx); err != nil {
				publicErrCh <- fmt.Errorf("public API server error: %w", err)
			}
			close(publicErrCh)
		}()
	}

	// Start control plane server
	errCh := make(chan error, 1)
	go func() {
		if err := c.server.Serve(ln); err != nil && err != http.ErrServerClosed {
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

// Shutdown gracefully shuts down the controller.
func (c *Controller) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}

	// Start draining: log the drain start with active workflow count
	activeCount := c.runner.ActiveRunCount()
	c.logger.Info("graceful shutdown initiated",
		slog.Int("active_workflows", activeCount))

	// Put runner into draining mode to stop accepting new workflows
	c.runner.StartDraining()

	// Stop accepting new connections (disable keep-alive)
	if c.server != nil {
		c.server.SetKeepAlivesEnabled(false)
	}

	// Wait for active workflows to complete (with drain timeout)
	drainCtx, drainCancel := context.WithTimeout(ctx, c.cfg.Controller.DrainTimeout)
	defer drainCancel()

	if err := c.runner.WaitForDrain(drainCtx, c.cfg.Controller.DrainTimeout); err != nil {
		remainingCount := c.runner.ActiveRunCount()
		c.logger.Warn("drain timeout exceeded",
			slog.Int("remaining_workflows", remainingCount),
			slog.Duration("drain_timeout", c.cfg.Controller.DrainTimeout))
	} else {
		c.logger.Info("all workflows completed during drain")
	}

	// Stop runner and wait for all goroutines to exit
	// Use remaining shutdown context time for runner stop
	if err := c.runner.Stop(ctx); err != nil {
		c.logger.Warn("runner stop timeout",
			internallog.Error(err))
	} else {
		c.logger.Info("runner stopped cleanly")
	}

	// Stop leader election
	if c.leader != nil {
		c.leader.Stop()
	}

	// Stop scheduler
	if c.scheduler != nil {
		c.scheduler.Stop()
	}

	// Stop file watcher service
	if c.fileWatcher != nil {
		if err := c.fileWatcher.Stop(); err != nil {
			c.logger.Error("failed to stop file watcher service", internallog.Error(err))
		}
	}

	// Stop MCP registry
	if c.mcpRegistry != nil {
		if err := c.mcpRegistry.Stop(); err != nil {
			c.logger.Error("MCP registry shutdown error",
				internallog.Error(err))
		}
	}

	// Stop poll trigger service
	if c.pollTriggerService != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 5*time.Second)
		defer shutdownCancel()
		if err := c.pollTriggerService.Stop(shutdownCtx); err != nil {
			c.logger.Error("poll trigger service shutdown error",
				internallog.Error(err))
		} else {
			c.logger.Info("poll trigger service stopped")
		}
	}

	// T7: Stop OverrideManager cleanup goroutine
	if c.overrideStopChan != nil {
		close(c.overrideStopChan)
		c.logger.Info("security override cleanup goroutine stopped")
	}

	// T7: Close audit logger via security manager
	if c.securityManager != nil {
		if err := c.securityManager.Close(); err != nil {
			c.logger.Error("security manager shutdown error",
				internallog.Error(err))
		} else {
			c.logger.Info("security manager shutdown complete")
		}
	}

	// Shutdown public API server first (if enabled)
	if c.publicServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, c.cfg.Controller.ShutdownTimeout)
		defer cancel()

		if err := c.publicServer.Shutdown(shutdownCtx); err != nil {
			c.logger.Error("public API server shutdown error",
				internallog.Error(err))
		}
	}

	// Shutdown control plane HTTP server
	if c.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, c.cfg.Controller.ShutdownTimeout)
		defer cancel()

		if err := c.server.Shutdown(shutdownCtx); err != nil {
			c.logger.Error("HTTP server shutdown error",
				internallog.Error(err))
		}
	}

	// Clean up PID file
	if c.pidFile != "" {
		if err := os.Remove(c.pidFile); err != nil && !os.IsNotExist(err) {
			c.logger.Error("failed to remove PID file",
				internallog.Error(err),
				slog.String("path", c.pidFile))
		}
	}

	// Clean up Unix socket file if it exists
	if c.cfg.Controller.Listen.SocketPath != "" {
		if err := os.Remove(c.cfg.Controller.Listen.SocketPath); err != nil && !os.IsNotExist(err) {
			c.logger.Error("failed to remove socket file",
				internallog.Error(err),
				slog.String("path", c.cfg.Controller.Listen.SocketPath))
		}
	}

	// Stop retention manager before shutting down trace storage
	if c.retentionMgr != nil {
		c.retentionMgr.Stop()
		c.logger.Info("trace retention manager stopped")
	}

	// Flush pending spans before shutdown
	if c.otelProvider != nil {
		flushCtx, flushCancel := context.WithTimeout(ctx, 10*time.Second)
		defer flushCancel()
		if err := c.otelProvider.ForceFlush(flushCtx); err != nil {
			c.logger.Warn("failed to flush pending spans",
				internallog.Error(err))
		}
	}

	// Shutdown OpenTelemetry provider
	if c.otelProvider != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := c.otelProvider.Shutdown(shutdownCtx); err != nil {
			c.logger.Error("OpenTelemetry provider shutdown error",
				internallog.Error(err))
		}
	}

	// Close audit logger
	if c.auditLogger != nil {
		if err := c.auditLogger.Close(); err != nil {
			c.logger.Error("failed to close audit logger",
				internallog.Error(err))
		}
	}

	// Close backend
	if c.backend != nil {
		if err := c.backend.Close(); err != nil {
			c.logger.Error("failed to close backend",
				internallog.Error(err))
		}
	}

	c.started = false
	c.logger.Info("controller stopped")
	return nil
}

// checkPermissionsAtStartup checks critical paths for insecure permissions and logs warnings.
func (c *Controller) checkPermissionsAtStartup() {
	pathsToCheck := []string{}

	// Check data directory (contains checkpoints, state files)
	if c.cfg.Controller.DataDir != "" {
		pathsToCheck = append(pathsToCheck, c.cfg.Controller.DataDir)
	}

	// Check PID file directory
	if c.cfg.Controller.PIDFile != "" {
		pidDir := filepath.Dir(c.cfg.Controller.PIDFile)
		pathsToCheck = append(pathsToCheck, pidDir)
	}

	// Check workflows directory (may contain sensitive configurations)
	if c.cfg.Controller.WorkflowsDir != "" {
		pathsToCheck = append(pathsToCheck, c.cfg.Controller.WorkflowsDir)
	}

	// Check each path and log warnings
	for _, path := range pathsToCheck {
		warnings := security.CheckConfigPermissions(path)
		for _, warning := range warnings {
			c.logger.Warn("security warning",
				slog.String("warning", warning))
		}
	}
}

// logSecurityWarnings logs warnings for risky security configurations.
func (c *Controller) logSecurityWarnings() {
	// If force-insecure is set, log a single warning and skip detailed checks
	if c.cfg.Controller.ForceInsecure {
		c.logger.Warn("security: running with --force-insecure flag",
			slog.String("warning", "security warnings suppressed - not recommended for production"))
		return
	}

	// Warn if authentication is disabled
	if !c.cfg.Controller.ControllerAuth.Enabled {
		c.logger.Warn("security: authentication is disabled",
			slog.String("recommendation", "enable daemon_auth.enabled for production use"))

		// Extra warning if listening on non-localhost TCP
		if c.cfg.Controller.Listen.TCPAddr != "" && !isLocalhostAddr(c.cfg.Controller.Listen.TCPAddr) {
			c.logger.Warn("security: authentication disabled on network-accessible address",
				slog.String("tcp_addr", c.cfg.Controller.Listen.TCPAddr),
				slog.String("risk", "unauthenticated API access from network"))
		}

		// Warning if public API is enabled without auth
		if c.cfg.Controller.Listen.PublicAPI.Enabled {
			c.logger.Warn("security: public API enabled without authentication",
				slog.String("public_api_tcp", c.cfg.Controller.Listen.PublicAPI.TCP),
				slog.String("risk", "publicly accessible unauthenticated webhooks"))
		}
	}

	// Warn if TLS is not enabled for TCP listener
	if c.cfg.Controller.Listen.TCPAddr != "" && c.cfg.Controller.Listen.TLSCert == "" {
		if !isLocalhostAddr(c.cfg.Controller.Listen.TCPAddr) {
			c.logger.Warn("security: TLS not configured for network listener",
				slog.String("tcp_addr", c.cfg.Controller.Listen.TCPAddr),
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
func (c *Controller) writePIDFile() error {
	// Create parent directory with restrictive permissions (0700)
	dir := filepath.Dir(c.cfg.Controller.PIDFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write PID with 0600 permissions (owner-only access)
	pid := os.Getpid()
	return os.WriteFile(c.cfg.Controller.PIDFile, []byte(fmt.Sprintf("%d\n", pid)), 0600)
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
func (c *Controller) RecordActivity() {
	c.mu.Lock()
	c.lastActivity = time.Now()
	c.mu.Unlock()
}

// timeSinceLastActivity returns duration since last activity.
func (c *Controller) timeSinceLastActivity() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Since(c.lastActivity)
}

// startIdleTimeoutMonitor starts a goroutine that monitors idle timeout.
// Only applies to auto-started controllers.
func (c *Controller) startIdleTimeoutMonitor(ctx context.Context, cancel context.CancelFunc) {
	if !c.autoStarted || c.cfg.Controller.IdleTimeout == 0 {
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
				idle := c.timeSinceLastActivity()
				if idle >= c.cfg.Controller.IdleTimeout {
					c.logger.Info("shutting down due to idle timeout",
						slog.Duration("idle_duration", idle),
						slog.Duration("idle_timeout", c.cfg.Controller.IdleTimeout))
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

// scanAndRegisterPollTriggers scans the workflows directory and registers poll triggers.
func (c *Controller) scanAndRegisterPollTriggers(ctx context.Context) {
	workflowsDir := c.cfg.Controller.WorkflowsDir
	if workflowsDir == "" {
		return
	}

	// Read directory
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		c.logger.Warn("failed to read workflows directory for poll triggers",
			internallog.Error(err),
			slog.String("dir", workflowsDir))
		return
	}

	registeredCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		workflowPath := filepath.Join(workflowsDir, entry.Name())

		// Load workflow
		data, err := os.ReadFile(workflowPath)
		if err != nil {
			c.logger.Warn("failed to read workflow file for poll trigger scan",
				internallog.Error(err),
				slog.String("workflow", workflowPath))
			continue
		}

		wf, err := workflow.ParseDefinition(data)
		if err != nil {
			c.logger.Warn("failed to parse workflow for poll trigger scan",
				internallog.Error(err),
				slog.String("workflow", workflowPath))
			continue
		}

		// Register poll triggers if present
		if err := c.pollTriggerService.RegisterWorkflowTriggers(workflowPath, wf); err != nil {
			c.logger.Warn("failed to register poll trigger",
				internallog.Error(err),
				slog.String("workflow", workflowPath))
		} else if wf.Trigger != nil && wf.Trigger.Poll != nil {
			registeredCount++
			c.logger.Info("registered poll trigger from workflow",
				slog.String("workflow", entry.Name()),
				slog.String("integration", wf.Trigger.Poll.Integration))
		}
	}

	if registeredCount > 0 {
		c.logger.Info("poll trigger scan complete",
			slog.Int("registered", registeredCount))
	}
}
