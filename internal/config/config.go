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

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	conductorerrors "github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/profile"
	"github.com/tombee/conductor/pkg/security"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidConfig is returned when configuration validation fails.
	ErrInvalidConfig = errors.New("config: invalid configuration")
)

// Config represents the complete Conductor configuration.
type Config struct {
	// Version indicates the config format version (1 = initial public release)
	Version int `yaml:"version,omitempty" json:"version,omitempty"`

	Log        LogConfig               `yaml:"log"`
	Controller ControllerConfig        `yaml:"controller"` // Controller service settings
	Security   security.SecurityConfig `yaml:"security"`   // Security framework settings

	// Multi-provider configuration
	Providers ProvidersMap `yaml:"providers,omitempty" json:"providers,omitempty"`
	AgentMappings            AgentMappings `yaml:"agent_mappings,omitempty" json:"agent_mappings,omitempty"`
	AcknowledgedDefaults     []string      `yaml:"acknowledged_defaults,omitempty" json:"acknowledged_defaults,omitempty"`
	SuppressUnmappedWarnings bool          `yaml:"suppress_unmapped_warnings,omitempty" json:"suppress_unmapped_warnings,omitempty"`

	// Tiers maps abstract tier names to specific provider/model references.
	// Format: "provider/model" (e.g., "anthropic/claude-3-5-haiku-20241022")
	// Supported tiers: fast, balanced, strategic
	Tiers map[string]string `yaml:"tiers,omitempty" json:"tiers,omitempty"`

	// Workspaces configuration
	// Workspaces contain profiles for workflow execution configuration
	Workspaces map[string]Workspace `yaml:"workspaces,omitempty" json:"workspaces,omitempty"`
}

// ControllerConfig configures controller service settings.
// This struct includes both CLI controller connection settings and controller server settings.
type ControllerConfig struct {
	// ForceInsecure explicitly acknowledges running with insecure configuration.
	// When true, security warnings about disabled auth or TLS are suppressed.
	// This flag is intended for development/testing environments only.
	// Default: false
	ForceInsecure bool `yaml:"force_insecure"`

	// AutoStart enables automatic controller startup when CLI commands need it.
	// When true, CLI will spawn the controller if not already running.
	// Default: true
	AutoStart bool `yaml:"auto_start"`

	// IdleTimeout is how long an auto-started controller waits before shutting down due to inactivity.
	// Only applies to controllers started via auto-start (not manually started controllers).
	// Default: 30m
	IdleTimeout time.Duration `yaml:"idle_timeout,omitempty"`

	// SocketPath is the Unix socket path for controller communication.
	// Environment: CONDUCTOR_SOCKET
	// Default: ~/.conductor/conductor.sock (or XDG_RUNTIME_DIR/conductor/conductor.sock)
	SocketPath string `yaml:"socket_path,omitempty"`

	// APIKey is the API key for authenticating with the controller.
	// Environment: CONDUCTOR_API_KEY
	APIKey string `yaml:"api_key,omitempty"`

	// Listen configures the controller's listener.
	Listen ControllerListenConfig `yaml:"listen,omitempty"`

	// PIDFile is the path to the PID file. Empty means no PID file.
	PIDFile string `yaml:"pid_file,omitempty"`

	// DataDir is the directory for controller data (checkpoints, state).
	DataDir string `yaml:"data_dir,omitempty"`

	// WorkflowsDir is the directory to search for workflow files.
	WorkflowsDir string `yaml:"workflows_dir,omitempty"`

	// ControllerLog is controller-specific logging configuration.
	ControllerLog ControllerLogConfig `yaml:"controller_log,omitempty"`

	// MaxConcurrentRuns limits concurrent workflow executions.
	MaxConcurrentRuns int `yaml:"max_concurrent_runs,omitempty"`

	// DefaultTimeout is the default timeout for workflow execution.
	DefaultTimeout time.Duration `yaml:"default_timeout,omitempty"`

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout,omitempty"`

	// DrainTimeout is the maximum duration to wait for active workflows to complete during shutdown.
	// When the controller receives SIGTERM, it stops accepting new workflows and waits up to this
	// duration for existing workflows to complete before forcing shutdown.
	// Environment: CONDUCTOR_DRAIN_TIMEOUT
	// Default: 30s
	DrainTimeout time.Duration `yaml:"drain_timeout,omitempty"`

	// RunRetention is how long completed runs are kept in memory before cleanup.
	// The cleanup loop runs every 60 minutes and removes runs older than this duration.
	// This only affects in-memory storage; backend persistence is handled separately.
	// Default: 24h
	RunRetention time.Duration `yaml:"run_retention,omitempty"`

	// CheckpointsEnabled enables checkpoint saving for crash recovery.
	CheckpointsEnabled bool `yaml:"checkpoints_enabled"`

	// Webhooks configures webhook routes (controller-specific).
	Webhooks WebhooksConfig `yaml:"webhooks,omitempty"`

	// Schedules configures scheduled workflows (controller-specific).
	Schedules SchedulesConfig `yaml:"schedules,omitempty"`

	// Endpoints configures named API endpoints (controller-specific).
	Endpoints EndpointsConfig `yaml:"endpoints,omitempty"`

	// FileWatchers configures file system watchers (controller-specific).
	FileWatchers FileWatchersConfig `yaml:"file_watchers,omitempty"`

	// ControllerAuth configures controller authentication (different from CLI auth).
	ControllerAuth ControllerAuthConfig `yaml:"controller_auth,omitempty"`

	// Backend configures the storage backend.
	Backend BackendConfig `yaml:"backend,omitempty"`

	// Distributed configures distributed mode.
	Distributed DistributedConfig `yaml:"distributed,omitempty"`

	// Observability configures tracing and metrics.
	Observability ObservabilityConfig `yaml:"observability,omitempty"`
}

// ControllerListenConfig configures how the controller listens for connections.
type ControllerListenConfig struct {
	// SocketPath is the Unix socket path (default).
	SocketPath string `yaml:"socket_path,omitempty"`

	// TCPAddr is an optional TCP address to listen on (e.g., ":9000").
	TCPAddr string `yaml:"tcp_addr,omitempty"`

	// AllowRemote must be true to bind to non-localhost TCP addresses.
	AllowRemote bool `yaml:"allow_remote"`

	// TLSCert is the path to TLS certificate for HTTPS.
	TLSCert string `yaml:"tls_cert,omitempty"`

	// TLSKey is the path to TLS key for HTTPS.
	TLSKey string `yaml:"tls_key,omitempty"`

	// PublicAPI configures an optional public-facing API server for webhooks and triggers.
	PublicAPI PublicAPIConfig `yaml:"public_api,omitempty"`
}

// PublicAPIConfig configures the public-facing API server.
// The public API serves webhooks and API-triggered workflows on a separate port
// from the control plane, enabling secure deployments where management APIs
// remain private while webhooks are publicly accessible.
type PublicAPIConfig struct {
	// Enabled activates the public API server (default: false).
	// When disabled, webhook and API trigger endpoints are not available.
	// Environment: CONDUCTOR_PUBLIC_API_ENABLED
	Enabled bool `yaml:"enabled"`

	// TCP is the TCP address to bind the public API server (e.g., ":9001", "0.0.0.0:9001").
	// Required when Enabled is true.
	// Environment: CONDUCTOR_PUBLIC_API_TCP
	TCP string `yaml:"tcp,omitempty"`
}

// ControllerLogConfig configures controller logging (separate from CLI logging).
type ControllerLogConfig struct {
	// Level is the log level (debug, info, warn, error).
	Level string `yaml:"level,omitempty"`

	// Format is the log format (text, json).
	Format string `yaml:"format,omitempty"`
}

// ControllerAuthConfig configures controller authentication (separate from CLI auth).
type ControllerAuthConfig struct {
	// Enabled controls whether authentication is required.
	Enabled bool `yaml:"enabled"`

	// APIKeys is the list of valid API keys.
	APIKeys []string `yaml:"api_keys,omitempty"`

	// AllowUnixSocket allows unauthenticated access via Unix socket.
	AllowUnixSocket bool `yaml:"allow_unix_socket"`
}

// BackendConfig configures the storage backend.
type BackendConfig struct {
	// Type is the backend type: "memory" or "postgres".
	Type string `yaml:"type,omitempty"`

	// Postgres contains PostgreSQL-specific configuration.
	Postgres PostgresConfig `yaml:"postgres,omitempty"`
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	// ConnectionString is the PostgreSQL connection URL.
	ConnectionString string `yaml:"connection_string,omitempty"`

	// MaxOpenConns sets the maximum number of open connections.
	MaxOpenConns int `yaml:"max_open_conns,omitempty"`

	// MaxIdleConns sets the maximum number of idle connections.
	MaxIdleConns int `yaml:"max_idle_conns,omitempty"`

	// ConnMaxLifetimeSeconds sets the maximum lifetime of a connection.
	ConnMaxLifetimeSeconds int `yaml:"conn_max_lifetime_seconds,omitempty"`
}

// DistributedConfig configures distributed mode settings.
type DistributedConfig struct {
	// Enabled activates distributed mode (requires Postgres backend).
	Enabled bool `yaml:"enabled"`

	// InstanceID uniquely identifies this controller instance.
	// If empty, a random ID is generated.
	InstanceID string `yaml:"instance_id,omitempty"`

	// LeaderElection enables leader election for scheduler.
	LeaderElection bool `yaml:"leader_election"`

	// StalledJobTimeoutSeconds is how long before a locked job is considered stalled.
	StalledJobTimeoutSeconds int `yaml:"stalled_job_timeout_seconds,omitempty"`
}

// WebhooksConfig configures webhook handling.
type WebhooksConfig struct {
	// Routes defines webhook routes.
	Routes []WebhookRoute `yaml:"routes,omitempty"`
}

// WebhookRoute defines a webhook route mapping.
type WebhookRoute struct {
	// Path is the URL path (e.g., "/webhooks/github").
	Path string `yaml:"path"`

	// Source is the webhook source type (github, slack, generic).
	Source string `yaml:"source"`

	// Workflow is the workflow to trigger.
	Workflow string `yaml:"workflow"`

	// Events limits which events trigger the workflow.
	Events []string `yaml:"events,omitempty"`

	// Secret is used for signature verification.
	Secret string `yaml:"secret,omitempty"`

	// InputMapping defines how to map payload to inputs.
	InputMapping map[string]string `yaml:"input_mapping,omitempty"`
}

// SchedulesConfig configures workflow scheduling.
type SchedulesConfig struct {
	// Enabled controls whether the scheduler runs.
	Enabled bool `yaml:"enabled"`

	// Schedules defines the scheduled workflows.
	Schedules []ScheduleEntry `yaml:"schedules,omitempty"`
}

// ScheduleEntry defines a scheduled workflow.
type ScheduleEntry struct {
	// Name is the unique schedule identifier.
	Name string `yaml:"name"`

	// Cron is the cron expression.
	Cron string `yaml:"cron"`

	// Workflow is the workflow to run.
	Workflow string `yaml:"workflow"`

	// Inputs are the workflow inputs.
	Inputs map[string]any `yaml:"inputs,omitempty"`

	// Enabled controls if this schedule is active.
	Enabled bool `yaml:"enabled"`

	// Timezone for cron evaluation.
	Timezone string `yaml:"timezone,omitempty"`
}

// EndpointsConfig configures named API endpoints.
type EndpointsConfig struct {
	// Enabled controls whether endpoints are active.
	Enabled bool `yaml:"enabled"`

	// Endpoints defines the available API endpoints.
	Endpoints []EndpointEntry `yaml:"endpoints,omitempty"`
}

// EndpointEntry defines a named API endpoint.
type EndpointEntry struct {
	// Name is the unique endpoint identifier.
	Name string `yaml:"name"`

	// Description provides documentation for this endpoint.
	Description string `yaml:"description,omitempty"`

	// Workflow is the workflow file to execute.
	Workflow string `yaml:"workflow"`

	// Inputs are default inputs merged with caller-provided inputs.
	Inputs map[string]any `yaml:"inputs,omitempty"`

	// Scopes defines which API key scopes can call this endpoint.
	Scopes []string `yaml:"scopes,omitempty"`

	// RateLimit specifies request limit (e.g., "100/hour", "10/minute").
	RateLimit string `yaml:"rate_limit,omitempty"`

	// Timeout is the maximum execution time.
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// Public indicates this endpoint requires no authentication.
	Public bool `yaml:"public,omitempty"`
}

// FileWatchersConfig configures file system watchers.
type FileWatchersConfig struct {
	// Enabled controls whether file watchers are active.
	Enabled bool `yaml:"enabled"`

	// Watchers defines the configured file watchers.
	Watchers []FileWatcherEntry `yaml:"watchers,omitempty"`
}

// FileWatcherEntry defines a file system watcher.
type FileWatcherEntry struct {
	// Name is the unique watcher identifier.
	Name string `yaml:"name"`

	// Workflow is the workflow file to execute when events occur.
	Workflow string `yaml:"workflow"`

	// Paths are the filesystem paths to watch.
	Paths []string `yaml:"paths"`

	// IncludePatterns are glob patterns for files to include (optional).
	IncludePatterns []string `yaml:"include_patterns,omitempty"`

	// ExcludePatterns are glob patterns for files to exclude (optional).
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty"`

	// Events are the event types to watch (created, modified, deleted, renamed).
	// Defaults to ["created"] if not specified.
	Events []string `yaml:"events,omitempty"`

	// DebounceWindow is the duration to wait for additional events (e.g., "1s", "500ms").
	DebounceWindow string `yaml:"debounce_window,omitempty"`

	// BatchMode enables batching of events during debounce window.
	BatchMode bool `yaml:"batch_mode,omitempty"`

	// MaxTriggersPerMinute limits the rate of workflow triggers (0 = unlimited).
	MaxTriggersPerMinute int `yaml:"max_triggers_per_minute,omitempty"`

	// Inputs are default inputs passed to the workflow.
	Inputs map[string]any `yaml:"inputs,omitempty"`

	// Enabled controls if this watcher is active.
	Enabled bool `yaml:"enabled"`
}

// ObservabilityConfig configures tracing and observability.
type ObservabilityConfig struct {
	// Enabled controls whether tracing is active.
	Enabled bool `yaml:"enabled"`

	// ServiceName identifies this service in traces.
	ServiceName string `yaml:"service_name,omitempty"`

	// ServiceVersion is the application version.
	ServiceVersion string `yaml:"service_version,omitempty"`

	// Sampling configures trace sampling.
	Sampling SamplingConfig `yaml:"sampling,omitempty"`

	// Storage configures trace storage.
	Storage StorageConfig `yaml:"storage,omitempty"`

	// Exporters configures OTLP export destinations.
	Exporters []ExporterConfig `yaml:"exporters,omitempty"`

	// BatchSize is the maximum number of spans per export batch (default: 512).
	BatchSize int `yaml:"batch_size,omitempty"`

	// BatchInterval is how often to flush spans in seconds (default: 5).
	BatchInterval int `yaml:"batch_interval,omitempty"`

	// Redaction configures sensitive data handling.
	Redaction RedactionConfig `yaml:"redaction,omitempty"`

	// Audit configures audit logging.
	Audit AuditConfig `yaml:"audit,omitempty"`
}

// SamplingConfig controls which traces are recorded.
type SamplingConfig struct {
	// Enabled activates sampling (default: false - sample all).
	Enabled bool `yaml:"enabled"`

	// Type is the sampling strategy: "head" or "tail".
	Type string `yaml:"type,omitempty"`

	// Rate is the fraction of traces to sample (0.0 - 1.0).
	Rate float64 `yaml:"rate,omitempty"`

	// AlwaysSampleErrors samples all traces with errors.
	AlwaysSampleErrors bool `yaml:"always_sample_errors"`
}

// StorageConfig controls local trace storage.
type StorageConfig struct {
	// Backend is the storage type: "sqlite" or "memory".
	Backend string `yaml:"backend,omitempty"`

	// Path is the SQLite database path (for backend=sqlite).
	Path string `yaml:"path,omitempty"`

	// Retention defines how long to keep traces.
	Retention RetentionConfig `yaml:"retention,omitempty"`
}

// RetentionConfig defines data retention policies.
type RetentionConfig struct {
	// TraceDays is how long to keep trace data (in days).
	TraceDays int `yaml:"trace_days,omitempty"`

	// EventDays is how long to keep event data (in days).
	EventDays int `yaml:"event_days,omitempty"`

	// AggregateDays is how long to keep aggregated metrics (in days).
	AggregateDays int `yaml:"aggregate_days,omitempty"`

	// CleanupInterval is how often to run cleanup (in hours). Default: 1 hour.
	CleanupInterval int `yaml:"cleanup_interval,omitempty"`
}

// ExporterConfig defines an OTLP export destination.
type ExporterConfig struct {
	// Type is the exporter type: "otlp", "otlp-http", or "console".
	Type string `yaml:"type"`

	// Endpoint is the OTLP receiver URL.
	Endpoint string `yaml:"endpoint,omitempty"`

	// Headers are additional HTTP headers for authentication.
	Headers map[string]string `yaml:"headers,omitempty"`

	// TLS configures secure connections.
	TLS TLSConfig `yaml:"tls,omitempty"`

	// TimeoutSeconds is the export timeout in seconds.
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty"`
}

// TLSConfig configures TLS for exporters.
type TLSConfig struct {
	// Enabled activates TLS.
	Enabled bool `yaml:"enabled"`

	// VerifyCertificate controls certificate validation.
	VerifyCertificate bool `yaml:"verify_certificate"`

	// CACertPath is the path to the CA certificate.
	CACertPath string `yaml:"ca_cert_path,omitempty"`
}

// RedactionConfig controls sensitive data redaction.
type RedactionConfig struct {
	// Level is the redaction mode: "none", "standard", or "strict".
	Level string `yaml:"level,omitempty"`

	// Patterns are custom redaction patterns.
	Patterns []RedactionPattern `yaml:"patterns,omitempty"`
}

// RedactionPattern defines a sensitive data pattern.
type RedactionPattern struct {
	// Name identifies this pattern.
	Name string `yaml:"name"`

	// Regex is the pattern to match.
	Regex string `yaml:"regex"`

	// Replacement is the string to substitute.
	Replacement string `yaml:"replacement,omitempty"`
}

// AuditConfig configures audit logging for API access.
type AuditConfig struct {
	// Enabled controls whether audit logging is active.
	Enabled bool `yaml:"enabled"`

	// Destination is where audit logs are written: "file", "stdout", or "syslog".
	Destination string `yaml:"destination,omitempty"`

	// FilePath is the path to the audit log file (when destination=file).
	FilePath string `yaml:"file_path,omitempty"`

	// TrustedProxies is a list of IP addresses to trust X-Forwarded-For from.
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`
}

// Workspace represents a security and configuration isolation boundary.
// Workspaces contain named profiles that define execution configurations
// for workflows.
type Workspace struct {
	// Name is the workspace identifier
	Name string `yaml:"name,omitempty" json:"name,omitempty"`

	// Description provides human-readable context about this workspace
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Profiles maps profile names to their configuration
	// Each workspace can have multiple profiles (dev, staging, prod, etc.)
	Profiles map[string]profile.Profile `yaml:"profiles,omitempty" json:"profiles,omitempty"`

	// DefaultProfile is the profile to use when none is specified
	// If empty, uses "default" profile if it exists
	DefaultProfile string `yaml:"default_profile,omitempty" json:"default_profile,omitempty"`
}


// LogConfig configures logging behavior.
type LogConfig struct {
	// Level sets the minimum log level (debug, info, warn, error).
	// Environment: LOG_LEVEL
	// Default: info
	Level string `yaml:"level"`

	// Format sets the output format (json, text).
	// Environment: LOG_FORMAT
	// Default: json
	Format string `yaml:"format"`

	// AddSource adds source file and line information to logs.
	// Environment: LOG_SOURCE
	// Default: false
	AddSource bool `yaml:"add_source"`
}


// Default returns a Config with sensible defaults.
func Default() *Config {
	socketPath := defaultSocketPath()
	dataDir := defaultDataDir()

	return &Config{
		Log: LogConfig{
			Level:     "info",
			Format:    "json",
			AddSource: false,
		},
		Security: security.SecurityConfig{
			DefaultProfile: security.ProfileStandard,
			Audit: security.AuditConfig{
				Enabled: false,
			},
		},
		Controller: ControllerConfig{
			AutoStart:   true,
			IdleTimeout: 30 * time.Minute,
			Listen: ControllerListenConfig{
				SocketPath:  socketPath,
				AllowRemote: false,
			},
			PIDFile:      "", // No PID file by default
			DataDir:      dataDir,
			WorkflowsDir: "./workflows",
			ControllerLog: ControllerLogConfig{
				Level:  "info",
				Format: "text",
			},
			MaxConcurrentRuns:  10,
			DefaultTimeout:     30 * time.Minute,
			ShutdownTimeout:    30 * time.Second,
			DrainTimeout:       30 * time.Second,
			RunRetention:       24 * time.Hour,
			CheckpointsEnabled: true,
			Backend: BackendConfig{
				Type: "memory",
			},
			Distributed: DistributedConfig{
				Enabled:                  false,
				LeaderElection:           true,
				StalledJobTimeoutSeconds: 300, // 5 minutes
			},
			ControllerAuth: ControllerAuthConfig{
				Enabled:         true, // Secure by default
				AllowUnixSocket: true, // Convenient local development
			},
			Observability: ObservabilityConfig{
				Enabled:        false, // Opt-in
				ServiceName:    "conductor",
				ServiceVersion: "unknown",
				Sampling: SamplingConfig{
					Enabled:            false,
					Type:               "head",
					Rate:               1.0, // Sample all by default
					AlwaysSampleErrors: true,
				},
				Storage: StorageConfig{
					Backend: "sqlite",
					Path:    "", // Will be set to DataDir/traces.db
					Retention: RetentionConfig{
						TraceDays:     7,  // 7 days
						EventDays:     30, // 30 days
						AggregateDays: 90, // 90 days
					},
				},
				Exporters: nil, // No exporters by default
				Redaction: RedactionConfig{
					Level:    "strict", // Strict by default for safety
					Patterns: nil,
				},
			},
		},
		// Default workspace with permissive profile
		Workspaces: map[string]Workspace{
			"default": {
				Name:        "default",
				Description: "Default workspace",
				Profiles: map[string]profile.Profile{
					"default": {
						Name:        "default",
						Description: "Default profile with environment inheritance",
						InheritEnv: profile.InheritEnvConfig{
							Enabled:   true,
							Allowlist: nil, // No restrictions - allows all env vars
						},
						Bindings: profile.Bindings{
							Integrations: make(map[string]profile.IntegrationBinding),
							MCPServers:   make(map[string]profile.MCPServerBinding),
						},
					},
				},
				DefaultProfile: "default",
			},
		},
	}
}

// Load loads configuration from environment variables and optionally from a YAML file.
// Environment variables take precedence over file-based configuration.
// If configPath is empty, only environment variables are used.
func Load(configPath string) (*Config, error) {
	cfg := Default()

	// If no config path provided, try the default config file
	if configPath == "" {
		defaultPath, err := ConfigPath()
		if err == nil {
			// Check if default config exists
			if _, statErr := os.Stat(defaultPath); statErr == nil {
				configPath = defaultPath
			}
		}
	}

	// Load from file if path provided or found
	if configPath != "" {
		if err := cfg.loadFromFile(configPath); err != nil {
			return nil, &conductorerrors.ConfigError{
				Key:    "config_file",
				Reason: fmt.Sprintf("failed to load from %s", configPath),
				Cause:  err,
			}
		}
	}

	// Apply defaults to any zero values (handles minimal configs)
	cfg.applyDefaults()

	// Override with environment variables
	cfg.loadFromEnv()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, &conductorerrors.ConfigError{
			Key:    "validation",
			Reason: "configuration validation failed",
			Cause:  err,
		}
	}

	return cfg, nil
}

// LoadWithSecrets loads configuration and resolves all secret references.
// It returns the config and any warnings about plaintext API keys.
func LoadWithSecrets(configPath string) (*Config, []string, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, nil, err
	}

	// Resolve all secret references in providers
	ctx := context.Background()
	warnings, err := ResolveSecretsInProviders(ctx, cfg.Providers)
	if err != nil {
		return nil, nil, &conductorerrors.ConfigError{
			Key:    "secrets",
			Reason: "failed to resolve secret references",
			Cause:  err,
		}
	}

	return cfg, warnings, nil
}

// LoadController loads configuration for controller mode.
// It loads from config.yaml and merges provider settings from settings.yaml.
func LoadController(configPath string) (*Config, error) {
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	// Also load provider settings from settings.yaml if available
	settingsPath, err := SettingsPath()
	if err == nil {
		if _, statErr := os.Stat(settingsPath); statErr == nil {
			// Read and merge settings
			data, readErr := os.ReadFile(settingsPath)
			if readErr == nil {
				var settings Config
				if yamlErr := yaml.Unmarshal(data, &settings); yamlErr == nil {
					// Merge provider-related settings if not already set
					if len(cfg.Providers) == 0 && len(settings.Providers) > 0 {
						cfg.Providers = settings.Providers
					}
					if len(cfg.Tiers) == 0 && len(settings.Tiers) > 0 {
						cfg.Tiers = settings.Tiers
					}
				}
			}
		}
	}

	return cfg, nil
}

// applyDefaults fills in zero values with sensible defaults.
// This allows minimal configs (e.g., just providers) to work without
// specifying all fields explicitly.
func (c *Config) applyDefaults() {
	defaults := Default()

	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = defaults.Log.Level
	}
	if c.Log.Format == "" {
		c.Log.Format = defaults.Log.Format
	}

	// Security defaults
	if c.Security.DefaultProfile == "" {
		c.Security.DefaultProfile = defaults.Security.DefaultProfile
	}

	// Controller defaults
	if c.Controller.IdleTimeout == 0 {
		c.Controller.IdleTimeout = defaults.Controller.IdleTimeout
	}
	if c.Controller.Listen.SocketPath == "" {
		c.Controller.Listen.SocketPath = defaults.Controller.Listen.SocketPath
	}
	if c.Controller.DataDir == "" {
		c.Controller.DataDir = defaults.Controller.DataDir
	}
	if c.Controller.WorkflowsDir == "" {
		c.Controller.WorkflowsDir = defaults.Controller.WorkflowsDir
	}
	if c.Controller.ControllerLog.Level == "" {
		c.Controller.ControllerLog.Level = defaults.Controller.ControllerLog.Level
	}
	if c.Controller.ControllerLog.Format == "" {
		c.Controller.ControllerLog.Format = defaults.Controller.ControllerLog.Format
	}
	if c.Controller.MaxConcurrentRuns == 0 {
		c.Controller.MaxConcurrentRuns = defaults.Controller.MaxConcurrentRuns
	}
	if c.Controller.DefaultTimeout == 0 {
		c.Controller.DefaultTimeout = defaults.Controller.DefaultTimeout
	}
	if c.Controller.ShutdownTimeout == 0 {
		c.Controller.ShutdownTimeout = defaults.Controller.ShutdownTimeout
	}
	if c.Controller.DrainTimeout == 0 {
		c.Controller.DrainTimeout = defaults.Controller.DrainTimeout
	}
	if c.Controller.RunRetention == 0 {
		c.Controller.RunRetention = defaults.Controller.RunRetention
	}
	if c.Controller.Backend.Type == "" {
		c.Controller.Backend.Type = defaults.Controller.Backend.Type
	}
	if c.Controller.Distributed.StalledJobTimeoutSeconds == 0 {
		c.Controller.Distributed.StalledJobTimeoutSeconds = defaults.Controller.Distributed.StalledJobTimeoutSeconds
	}
	if c.Controller.Observability.ServiceName == "" {
		c.Controller.Observability.ServiceName = defaults.Controller.Observability.ServiceName
	}
	if c.Controller.Observability.ServiceVersion == "" {
		c.Controller.Observability.ServiceVersion = defaults.Controller.Observability.ServiceVersion
	}
	if c.Controller.Observability.Sampling.Type == "" {
		c.Controller.Observability.Sampling.Type = defaults.Controller.Observability.Sampling.Type
	}
	if c.Controller.Observability.Sampling.Rate == 0 {
		c.Controller.Observability.Sampling.Rate = defaults.Controller.Observability.Sampling.Rate
	}
	if c.Controller.Observability.Storage.Backend == "" {
		c.Controller.Observability.Storage.Backend = defaults.Controller.Observability.Storage.Backend
	}
	if c.Controller.Observability.Storage.Retention.TraceDays == 0 {
		c.Controller.Observability.Storage.Retention.TraceDays = defaults.Controller.Observability.Storage.Retention.TraceDays
	}
	if c.Controller.Observability.Storage.Retention.EventDays == 0 {
		c.Controller.Observability.Storage.Retention.EventDays = defaults.Controller.Observability.Storage.Retention.EventDays
	}
	if c.Controller.Observability.Storage.Retention.AggregateDays == 0 {
		c.Controller.Observability.Storage.Retention.AggregateDays = defaults.Controller.Observability.Storage.Retention.AggregateDays
	}
	if c.Controller.Observability.Redaction.Level == "" {
		c.Controller.Observability.Redaction.Level = defaults.Controller.Observability.Redaction.Level
	}

	// Workspace defaults
	// If no workspaces are configured, use the default workspace
	if len(c.Workspaces) == 0 {
		c.Workspaces = defaults.Workspaces
	}
}

// loadFromFile loads configuration from a YAML file.
func (c *Config) loadFromFile(path string) error {
	// Expand home directory if present
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables.
func (c *Config) loadFromEnv() {
	// Log configuration
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		c.Log.Level = strings.ToLower(val)
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		c.Log.Format = strings.ToLower(val)
	}
	if val := os.Getenv("LOG_SOURCE"); val != "" {
		c.Log.AddSource = val == "1" || strings.ToLower(val) == "true"
	}

	// Controller configuration (CLI-related)
	if val := os.Getenv("CONDUCTOR_CONTROLLER_AUTO_START"); val != "" {
		c.Controller.AutoStart = val == "1" || strings.ToLower(val) == "true"
	}
	if val := os.Getenv("CONDUCTOR_SOCKET"); val != "" {
		c.Controller.SocketPath = val
	}
	if val := os.Getenv("CONDUCTOR_API_KEY"); val != "" {
		c.Controller.APIKey = val
	}

	// Controller configuration (controller-specific)
	if val := os.Getenv("CONDUCTOR_LISTEN_SOCKET"); val != "" {
		c.Controller.Listen.SocketPath = val
	}
	if val := os.Getenv("CONDUCTOR_TCP_ADDR"); val != "" {
		c.Controller.Listen.TCPAddr = val
	}
	if val := os.Getenv("CONDUCTOR_PUBLIC_API_ENABLED"); val != "" {
		c.Controller.Listen.PublicAPI.Enabled = val == "1" || strings.ToLower(val) == "true"
	}
	if val := os.Getenv("CONDUCTOR_PUBLIC_API_TCP"); val != "" {
		c.Controller.Listen.PublicAPI.TCP = val
	}
	if val := os.Getenv("CONDUCTOR_PID_FILE"); val != "" {
		c.Controller.PIDFile = val
	}
	if val := os.Getenv("CONDUCTOR_DATA_DIR"); val != "" {
		c.Controller.DataDir = val
	}
	if val := os.Getenv("CONDUCTOR_WORKFLOWS_DIR"); val != "" {
		c.Controller.WorkflowsDir = val
	}
	if val := os.Getenv("CONDUCTOR_CONTROLLER_LOG_LEVEL"); val != "" {
		c.Controller.ControllerLog.Level = val
	}
	if val := os.Getenv("CONDUCTOR_CONTROLLER_LOG_FORMAT"); val != "" {
		c.Controller.ControllerLog.Format = val
	}
	if val := os.Getenv("CONDUCTOR_MAX_CONCURRENT_RUNS"); val != "" {
		if runs, err := strconv.Atoi(val); err == nil {
			c.Controller.MaxConcurrentRuns = runs
		}
	}
	if val := os.Getenv("CONDUCTOR_DEFAULT_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Controller.DefaultTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Controller.ShutdownTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_DRAIN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Controller.DrainTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_CHECKPOINTS_ENABLED"); val != "" {
		c.Controller.CheckpointsEnabled = val == "1" || strings.ToLower(val) == "true"
	}
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	var errs []string

	// Validate log configuration
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
	if !validLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf("log.level must be one of [debug, info, warn, warning, error], got %q", c.Log.Level))
	}
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Log.Format] {
		errs = append(errs, fmt.Sprintf("log.format must be one of [json, text], got %q", c.Log.Format))
	}

	// Validate each provider configuration
	for name, provider := range c.Providers {
		if provider.Type == "" {
			errs = append(errs, fmt.Sprintf("provider %q must have a type field", name))
		}
		// Note: Additional provider-specific validation will be done by provider implementations
	}

	// Validate agent mappings reference valid providers
	for agent, provider := range c.AgentMappings {
		if _, exists := c.Providers[provider]; !exists {
			errs = append(errs, fmt.Sprintf("agent_mappings[%q] references unknown provider %q. Available: %v", agent, provider, keysOf(c.Providers)))
		}
	}

	// Validate tier mappings
	tierErrs := c.ValidateTiers()
	for _, tierErr := range tierErrs {
		errs = append(errs, tierErr.Error())
	}

	// Validate public API configuration
	if c.Controller.Listen.PublicAPI.Enabled {
		if c.Controller.Listen.PublicAPI.TCP == "" {
			errs = append(errs, "controller.listen.public_api.tcp is required when public_api.enabled is true")
		}
	}

	// Validate endpoints configuration
	if c.Controller.Endpoints.Enabled {
		endpointNames := make(map[string]bool)
		for i, ep := range c.Controller.Endpoints.Endpoints {
			// Validate required fields
			if ep.Name == "" {
				errs = append(errs, fmt.Sprintf("controller.endpoints.endpoints[%d]: name is required", i))
			} else {
				// Check for duplicate names
				if endpointNames[ep.Name] {
					errs = append(errs, fmt.Sprintf("controller.endpoints.endpoints[%d]: duplicate endpoint name %q", i, ep.Name))
				}
				endpointNames[ep.Name] = true
			}

			if ep.Workflow == "" {
				errs = append(errs, fmt.Sprintf("controller.endpoints.endpoints[%d] (%s): workflow is required", i, ep.Name))
			}

			// Validate timeout is non-negative
			if ep.Timeout < 0 {
				errs = append(errs, fmt.Sprintf("controller.endpoints.endpoints[%d] (%s): timeout must be non-negative, got %v", i, ep.Name, ep.Timeout))
			}

			// Validate rate limit format if specified
			if ep.RateLimit != "" {
				if err := validateRateLimitFormat(ep.RateLimit); err != nil {
					errs = append(errs, fmt.Sprintf("controller.endpoints.endpoints[%d] (%s): %v", i, ep.Name, err))
				}
			}
		}
	}

	// Validate retention days (must be positive when observability is enabled)
	if c.Controller.Observability.Enabled {
		ret := c.Controller.Observability.Storage.Retention
		if ret.TraceDays <= 0 {
			errs = append(errs, fmt.Sprintf("controller.observability.storage.retention.trace_days must be positive, got %d", ret.TraceDays))
		}
		if ret.EventDays <= 0 {
			errs = append(errs, fmt.Sprintf("controller.observability.storage.retention.event_days must be positive, got %d", ret.EventDays))
		}
		if ret.AggregateDays <= 0 {
			errs = append(errs, fmt.Sprintf("controller.observability.storage.retention.aggregate_days must be positive, got %d", ret.AggregateDays))
		}

		// Validate sampling rate is in valid range [0.0, 1.0]
		if c.Controller.Observability.Sampling.Enabled {
			rate := c.Controller.Observability.Sampling.Rate
			if rate < 0.0 || rate > 1.0 {
				errs = append(errs, fmt.Sprintf("controller.observability.sampling.rate must be between 0.0 and 1.0, got %f", rate))
			}
		}

		// Validate audit configuration
		if c.Controller.Observability.Audit.Enabled {
			validDestinations := map[string]bool{"file": true, "stdout": true, "syslog": true}
			if c.Controller.Observability.Audit.Destination == "" {
				errs = append(errs, "controller.observability.audit.destination is required when audit.enabled is true")
			} else if !validDestinations[c.Controller.Observability.Audit.Destination] {
				errs = append(errs, fmt.Sprintf("controller.observability.audit.destination must be one of [file, stdout, syslog], got %q", c.Controller.Observability.Audit.Destination))
			}

			// Validate file_path is set when destination is file
			if c.Controller.Observability.Audit.Destination == "file" && c.Controller.Observability.Audit.FilePath == "" {
				errs = append(errs, "controller.observability.audit.file_path is required when audit.destination is 'file'")
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w:\n  - %s", ErrInvalidConfig, strings.Join(errs, "\n  - "))
	}

	return nil
}

// keysOf returns the keys of a ProvidersMap as a slice
func keysOf(m ProvidersMap) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// validateRateLimitFormat validates rate limit string format (e.g., "100/hour", "10/minute")
func validateRateLimitFormat(rateLimit string) error {
	parts := strings.Split(rateLimit, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid rate_limit format %q, expected format: <count>/<unit> (e.g., 100/hour, 10/minute)", rateLimit)
	}

	// Validate count is a positive integer
	count, err := strconv.Atoi(parts[0])
	if err != nil || count <= 0 {
		return fmt.Errorf("invalid rate_limit count %q, must be a positive integer", parts[0])
	}

	// Validate unit
	validUnits := map[string]bool{
		"second": true,
		"minute": true,
		"hour":   true,
		"day":    true,
	}
	if !validUnits[parts[1]] {
		return fmt.Errorf("invalid rate_limit unit %q, must be one of: second, minute, hour, day", parts[1])
	}

	return nil
}

// defaultSocketPath returns the default Unix socket path.
func defaultSocketPath() string {
	// Use XDG_RUNTIME_DIR if available (Linux)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "conductor", "conductor.sock")
	}

	// Fall back to ~/.conductor/conductor.sock
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/conductor.sock"
	}

	return filepath.Join(homeDir, ".conductor", "conductor.sock")
}

// defaultDataDir returns the default data directory.
func defaultDataDir() string {
	// Use XDG_DATA_HOME if available
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "conductor")
	}

	// Fall back to ~/.conductor/data
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/conductor-data"
	}

	return filepath.Join(homeDir, ".conductor", "data")
}

// CheckpointDir returns the checkpoint directory path for the controller.
func (c *ControllerConfig) CheckpointDir() string {
	if !c.CheckpointsEnabled {
		return ""
	}
	return filepath.Join(c.DataDir, "checkpoints")
}

// GetPrimaryProvider returns the primary provider name from tiers or first available provider.
// It checks tiers in priority order: balanced, fast, strategic.
// If no tiers are configured, returns the first provider name alphabetically for determinism.
// Returns empty string if no providers are configured.
func (c *Config) GetPrimaryProvider() string {
	// Check tiers in priority order
	for _, tier := range []string{"balanced", "fast", "strategic"} {
		if tierRef, ok := c.Tiers[tier]; ok {
			if idx := strings.Index(tierRef, "/"); idx > 0 {
				return tierRef[:idx]
			}
		}
	}

	// Fallback to first provider alphabetically (deterministic ordering)
	if len(c.Providers) > 0 {
		names := make([]string, 0, len(c.Providers))
		for name := range c.Providers {
			names = append(names, name)
		}
		sort.Strings(names)
		return names[0]
	}

	return ""
}
