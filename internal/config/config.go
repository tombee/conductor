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
	Server   ServerConfig           `yaml:"server"`
	Auth     AuthConfig             `yaml:"auth"`
	Log      LogConfig              `yaml:"log"`
	LLM      LLMConfig              `yaml:"llm"`      // Global LLM settings (timeouts, retries, etc.)
	Daemon   DaemonConfig           `yaml:"daemon"`   // Daemon-related settings
	Security security.SecurityConfig `yaml:"security"` // Security framework settings

	// Multi-provider configuration (new format)
	DefaultProvider            string        `yaml:"default_provider,omitempty" json:"default_provider,omitempty"`
	Providers                  ProvidersMap  `yaml:"providers,omitempty" json:"providers,omitempty"`
	AgentMappings              AgentMappings `yaml:"agent_mappings,omitempty" json:"agent_mappings,omitempty"`
	AcknowledgedDefaults       []string      `yaml:"acknowledged_defaults,omitempty" json:"acknowledged_defaults,omitempty"`
	SuppressUnmappedWarnings   bool          `yaml:"suppress_unmapped_warnings,omitempty" json:"suppress_unmapped_warnings,omitempty"`

	// Workspaces configuration
	// Workspaces contain profiles for workflow execution configuration
	Workspaces map[string]Workspace `yaml:"workspaces,omitempty" json:"workspaces,omitempty"`
}

// DaemonConfig configures daemon-related settings.
// This struct includes both CLI daemon connection settings and daemon server settings.
type DaemonConfig struct {
	// ForceInsecure explicitly acknowledges running with insecure configuration.
	// When true, security warnings about disabled auth or TLS are suppressed.
	// This flag is intended for development/testing environments only.
	// Default: false
	ForceInsecure bool `yaml:"force_insecure"`

	// AutoStart enables automatic daemon startup when CLI commands need it.
	// When true, CLI will spawn conductord if not already running.
	// Default: true
	AutoStart bool `yaml:"auto_start"`

	// IdleTimeout is how long an auto-started daemon waits before shutting down due to inactivity.
	// Only applies to daemons started via auto-start (not manually started daemons).
	// Default: 30m
	IdleTimeout time.Duration `yaml:"idle_timeout,omitempty"`

	// SocketPath is the Unix socket path for daemon communication.
	// Environment: CONDUCTOR_SOCKET
	// Default: ~/.conductor/conductor.sock (or XDG_RUNTIME_DIR/conductor/conductor.sock)
	SocketPath string `yaml:"socket_path,omitempty"`

	// APIKey is the API key for authenticating with the daemon.
	// Environment: CONDUCTOR_API_KEY
	APIKey string `yaml:"api_key,omitempty"`

	// Listen configures the daemon's listener (daemon-specific).
	Listen DaemonListenConfig `yaml:"listen,omitempty"`

	// PIDFile is the path to the PID file (daemon-specific). Empty means no PID file.
	PIDFile string `yaml:"pid_file,omitempty"`

	// DataDir is the directory for daemon data (checkpoints, state).
	DataDir string `yaml:"data_dir,omitempty"`

	// WorkflowsDir is the directory to search for workflow files.
	WorkflowsDir string `yaml:"workflows_dir,omitempty"`

	// DaemonLog is daemon-specific logging configuration.
	DaemonLog DaemonLogConfig `yaml:"daemon_log,omitempty"`

	// MaxConcurrentRuns limits concurrent workflow executions.
	MaxConcurrentRuns int `yaml:"max_concurrent_runs,omitempty"`

	// DefaultTimeout is the default timeout for workflow execution.
	DefaultTimeout time.Duration `yaml:"default_timeout,omitempty"`

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout,omitempty"`

	// DrainTimeout is the maximum duration to wait for active workflows to complete during shutdown.
	// When the daemon receives SIGTERM, it stops accepting new workflows and waits up to this
	// duration for existing workflows to complete before forcing shutdown.
	// Environment: CONDUCTOR_DRAIN_TIMEOUT
	// Default: 30s
	DrainTimeout time.Duration `yaml:"drain_timeout,omitempty"`

	// CheckpointsEnabled enables checkpoint saving for crash recovery.
	CheckpointsEnabled bool `yaml:"checkpoints_enabled"`

	// Webhooks configures webhook routes (daemon-specific).
	Webhooks WebhooksConfig `yaml:"webhooks,omitempty"`

	// Schedules configures scheduled workflows (daemon-specific).
	Schedules SchedulesConfig `yaml:"schedules,omitempty"`

	// Endpoints configures named API endpoints (daemon-specific).
	Endpoints EndpointsConfig `yaml:"endpoints,omitempty"`

	// DaemonAuth configures daemon authentication (different from CLI auth).
	DaemonAuth DaemonAuthConfig `yaml:"daemon_auth,omitempty"`

	// Backend configures the storage backend.
	Backend BackendConfig `yaml:"backend,omitempty"`

	// Distributed configures distributed mode.
	Distributed DistributedConfig `yaml:"distributed,omitempty"`

	// Observability configures tracing and metrics.
	Observability ObservabilityConfig `yaml:"observability,omitempty"`
}

// DaemonListenConfig configures how the daemon listens for connections.
type DaemonListenConfig struct {
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

// DaemonLogConfig configures daemon logging (separate from CLI logging).
type DaemonLogConfig struct {
	// Level is the log level (debug, info, warn, error).
	Level string `yaml:"level,omitempty"`

	// Format is the log format (text, json).
	Format string `yaml:"format,omitempty"`
}

// DaemonAuthConfig configures daemon authentication (separate from CLI auth).
type DaemonAuthConfig struct {
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

	// InstanceID uniquely identifies this daemon instance.
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

	// Redaction configures sensitive data handling.
	Redaction RedactionConfig `yaml:"redaction,omitempty"`
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

// ServerConfig configures the RPC server behavior.
type ServerConfig struct {
	// Port specifies the port to bind to.
	// Consumed by: internal/commands/daemon/serve.go:85
	// Default: 9876
	Port int `yaml:"port"`

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	// Consumed by: internal/commands/daemon/serve.go:86
	// Default: 5s
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

// AuthConfig configures authentication settings.
// Note: Rate limiting is handled by internal/rpc/auth.go with hardcoded values.
type AuthConfig struct {
	// TokenLength is the length of generated auth tokens in bytes.
	// Consumed by: token generation (validation only - actual token generation uses crypto/rand)
	// Default: 32
	TokenLength int `yaml:"token_length"`
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

// LLMConfig configures LLM provider settings.
// Note: Connection pooling and trace retention are handled by observability config.
type LLMConfig struct {
	// DefaultProvider is the default LLM provider to use.
	// Consumed by: provider selection logic, multi-provider config
	// Environment: LLM_DEFAULT_PROVIDER
	// Default: anthropic
	DefaultProvider string `yaml:"default_provider"`

	// RequestTimeout is the maximum duration for LLM requests.
	// Consumed by: LLM provider HTTP clients
	// Environment: LLM_REQUEST_TIMEOUT
	// Default: 5s
	RequestTimeout time.Duration `yaml:"request_timeout"`

	// MaxRetries is the maximum number of retry attempts for failed requests.
	// Consumed by: LLM provider retry logic
	// Environment: LLM_MAX_RETRIES
	// Default: 3
	MaxRetries int `yaml:"max_retries"`

	// RetryBackoffBase is the base duration for exponential backoff.
	// Consumed by: LLM provider retry logic
	// Environment: LLM_RETRY_BACKOFF_BASE
	// Default: 100ms
	RetryBackoffBase time.Duration `yaml:"retry_backoff_base"`

	// FailoverProviders is the ordered list of providers to try on failure.
	// When configured, enables automatic failover with circuit breaker.
	// Environment: LLM_FAILOVER_PROVIDERS (comma-separated)
	// Default: empty (failover disabled)
	FailoverProviders []string `yaml:"failover_providers,omitempty"`

	// CircuitBreakerThreshold is the number of consecutive failures before opening the circuit.
	// 0 disables circuit breaker.
	// Environment: LLM_CIRCUIT_BREAKER_THRESHOLD
	// Default: 5
	CircuitBreakerThreshold int `yaml:"circuit_breaker_threshold,omitempty"`

	// CircuitBreakerTimeout is how long to keep the circuit open before trying again.
	// Environment: LLM_CIRCUIT_BREAKER_TIMEOUT
	// Default: 30s
	CircuitBreakerTimeout time.Duration `yaml:"circuit_breaker_timeout,omitempty"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	socketPath := defaultSocketPath()
	dataDir := defaultDataDir()

	return &Config{
		Server: ServerConfig{
			Port:            9876,
			ShutdownTimeout: 5 * time.Second,
		},
		Auth: AuthConfig{
			TokenLength: 32,
		},
		Log: LogConfig{
			Level:     "info",
			Format:    "json",
			AddSource: false,
		},
		LLM: LLMConfig{
			DefaultProvider:         "anthropic",
			RequestTimeout:          5 * time.Second,
			MaxRetries:              3,
			RetryBackoffBase:        100 * time.Millisecond,
			FailoverProviders:       nil,  // Failover disabled by default
			CircuitBreakerThreshold: 5,    // Default threshold
			CircuitBreakerTimeout:   30 * time.Second,
		},
		Security: security.SecurityConfig{
			DefaultProfile: security.ProfileStandard,
			Audit: security.AuditConfig{
				Enabled: false,
			},
			PrewarmSandbox: false,
		},
		Daemon: DaemonConfig{
			AutoStart:   true,
			IdleTimeout: 30 * time.Minute,
			Listen: DaemonListenConfig{
				SocketPath:  socketPath,
				AllowRemote: false,
			},
			PIDFile:            "", // No PID file by default
			DataDir:            dataDir,
			WorkflowsDir:       "./workflows",
			DaemonLog: DaemonLogConfig{
				Level:  "info",
				Format: "text",
			},
			MaxConcurrentRuns:  10,
			DefaultTimeout:     30 * time.Minute,
			ShutdownTimeout:    30 * time.Second,
			DrainTimeout:       30 * time.Second,
			CheckpointsEnabled: true,
			Backend: BackendConfig{
				Type: "memory",
			},
			Distributed: DistributedConfig{
				Enabled:                  false,
				LeaderElection:           true,
				StalledJobTimeoutSeconds: 300, // 5 minutes
			},
			DaemonAuth: DaemonAuthConfig{
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
		// Default workspace with backward-compatible profile
		Workspaces: map[string]Workspace{
			"default": {
				Name:        "default",
				Description: "Default workspace for backward compatibility",
				Profiles: map[string]profile.Profile{
					"default": {
						Name:        "default",
						Description: "Default profile with environment inheritance",
						InheritEnv: profile.InheritEnvConfig{
							Enabled:   true,
							Allowlist: nil, // No restrictions - backward compatible
						},
						Bindings: profile.Bindings{
							Connectors: make(map[string]profile.ConnectorBinding),
							MCPServers: make(map[string]profile.MCPServerBinding),
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

	// Load from file if path provided
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

// LoadDaemon loads configuration for daemon mode.
// This is functionally equivalent to Load but provides a clearer API
// for daemon-specific configuration loading.
func LoadDaemon(configPath string) (*Config, error) {
	return Load(configPath)
}

// applyDefaults fills in zero values with sensible defaults.
// This allows minimal configs (e.g., just providers) to work without
// specifying all fields explicitly.
func (c *Config) applyDefaults() {
	defaults := Default()

	// Server defaults
	if c.Server.Port == 0 {
		c.Server.Port = defaults.Server.Port
	}
	if c.Server.ShutdownTimeout == 0 {
		c.Server.ShutdownTimeout = defaults.Server.ShutdownTimeout
	}

	// Auth defaults
	if c.Auth.TokenLength == 0 {
		c.Auth.TokenLength = defaults.Auth.TokenLength
	}

	// Log defaults
	if c.Log.Level == "" {
		c.Log.Level = defaults.Log.Level
	}
	if c.Log.Format == "" {
		c.Log.Format = defaults.Log.Format
	}

	// LLM defaults
	// Clear LLM.DefaultProvider if it conflicts with configured providers.
	// The multi-provider system uses the root-level DefaultProvider field.
	if c.LLM.DefaultProvider != "" && len(c.Providers) > 0 {
		if _, exists := c.Providers[c.LLM.DefaultProvider]; !exists {
			// LLM.DefaultProvider conflicts with provider config, clear it
			c.LLM.DefaultProvider = ""
		}
	}
	if c.LLM.RequestTimeout == 0 {
		c.LLM.RequestTimeout = defaults.LLM.RequestTimeout
	}
	if c.LLM.MaxRetries == 0 {
		c.LLM.MaxRetries = defaults.LLM.MaxRetries
	}
	if c.LLM.RetryBackoffBase == 0 {
		c.LLM.RetryBackoffBase = defaults.LLM.RetryBackoffBase
	}
	if c.LLM.CircuitBreakerThreshold == 0 {
		c.LLM.CircuitBreakerThreshold = defaults.LLM.CircuitBreakerThreshold
	}
	if c.LLM.CircuitBreakerTimeout == 0 {
		c.LLM.CircuitBreakerTimeout = defaults.LLM.CircuitBreakerTimeout
	}

	// Security defaults
	if c.Security.DefaultProfile == "" {
		c.Security.DefaultProfile = defaults.Security.DefaultProfile
	}

	// Daemon defaults
	if c.Daemon.IdleTimeout == 0 {
		c.Daemon.IdleTimeout = defaults.Daemon.IdleTimeout
	}
	if c.Daemon.Listen.SocketPath == "" {
		c.Daemon.Listen.SocketPath = defaults.Daemon.Listen.SocketPath
	}
	if c.Daemon.DataDir == "" {
		c.Daemon.DataDir = defaults.Daemon.DataDir
	}
	if c.Daemon.WorkflowsDir == "" {
		c.Daemon.WorkflowsDir = defaults.Daemon.WorkflowsDir
	}
	if c.Daemon.DaemonLog.Level == "" {
		c.Daemon.DaemonLog.Level = defaults.Daemon.DaemonLog.Level
	}
	if c.Daemon.DaemonLog.Format == "" {
		c.Daemon.DaemonLog.Format = defaults.Daemon.DaemonLog.Format
	}
	if c.Daemon.MaxConcurrentRuns == 0 {
		c.Daemon.MaxConcurrentRuns = defaults.Daemon.MaxConcurrentRuns
	}
	if c.Daemon.DefaultTimeout == 0 {
		c.Daemon.DefaultTimeout = defaults.Daemon.DefaultTimeout
	}
	if c.Daemon.ShutdownTimeout == 0 {
		c.Daemon.ShutdownTimeout = defaults.Daemon.ShutdownTimeout
	}
	if c.Daemon.DrainTimeout == 0 {
		c.Daemon.DrainTimeout = defaults.Daemon.DrainTimeout
	}
	if c.Daemon.Backend.Type == "" {
		c.Daemon.Backend.Type = defaults.Daemon.Backend.Type
	}
	if c.Daemon.Distributed.StalledJobTimeoutSeconds == 0 {
		c.Daemon.Distributed.StalledJobTimeoutSeconds = defaults.Daemon.Distributed.StalledJobTimeoutSeconds
	}
	if c.Daemon.Observability.ServiceName == "" {
		c.Daemon.Observability.ServiceName = defaults.Daemon.Observability.ServiceName
	}
	if c.Daemon.Observability.ServiceVersion == "" {
		c.Daemon.Observability.ServiceVersion = defaults.Daemon.Observability.ServiceVersion
	}
	if c.Daemon.Observability.Sampling.Type == "" {
		c.Daemon.Observability.Sampling.Type = defaults.Daemon.Observability.Sampling.Type
	}
	if c.Daemon.Observability.Sampling.Rate == 0 {
		c.Daemon.Observability.Sampling.Rate = defaults.Daemon.Observability.Sampling.Rate
	}
	if c.Daemon.Observability.Storage.Backend == "" {
		c.Daemon.Observability.Storage.Backend = defaults.Daemon.Observability.Storage.Backend
	}
	if c.Daemon.Observability.Storage.Retention.TraceDays == 0 {
		c.Daemon.Observability.Storage.Retention.TraceDays = defaults.Daemon.Observability.Storage.Retention.TraceDays
	}
	if c.Daemon.Observability.Storage.Retention.EventDays == 0 {
		c.Daemon.Observability.Storage.Retention.EventDays = defaults.Daemon.Observability.Storage.Retention.EventDays
	}
	if c.Daemon.Observability.Storage.Retention.AggregateDays == 0 {
		c.Daemon.Observability.Storage.Retention.AggregateDays = defaults.Daemon.Observability.Storage.Retention.AggregateDays
	}
	if c.Daemon.Observability.Redaction.Level == "" {
		c.Daemon.Observability.Redaction.Level = defaults.Daemon.Observability.Redaction.Level
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
	// Server configuration
	if val := os.Getenv("SERVER_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Server.ShutdownTimeout = duration
		}
	}

	// Auth configuration
	if val := os.Getenv("AUTH_TOKEN_LENGTH"); val != "" {
		if length, err := strconv.Atoi(val); err == nil {
			c.Auth.TokenLength = length
		}
	}

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

	// LLM configuration
	if val := os.Getenv("LLM_DEFAULT_PROVIDER"); val != "" {
		c.LLM.DefaultProvider = strings.ToLower(val)
	}

	// Multi-provider configuration
	// CONDUCTOR_PROVIDER overrides default_provider
	if val := os.Getenv("CONDUCTOR_PROVIDER"); val != "" {
		c.DefaultProvider = val
	}

	// Daemon configuration (CLI-related)
	if val := os.Getenv("CONDUCTOR_DAEMON_AUTO_START"); val != "" {
		c.Daemon.AutoStart = val == "1" || strings.ToLower(val) == "true"
	}
	if val := os.Getenv("CONDUCTOR_SOCKET"); val != "" {
		c.Daemon.SocketPath = val
	}
	if val := os.Getenv("CONDUCTOR_API_KEY"); val != "" {
		c.Daemon.APIKey = val
	}

	// Daemon configuration (daemon-specific)
	if val := os.Getenv("CONDUCTOR_LISTEN_SOCKET"); val != "" {
		c.Daemon.Listen.SocketPath = val
	}
	if val := os.Getenv("CONDUCTOR_TCP_ADDR"); val != "" {
		c.Daemon.Listen.TCPAddr = val
	}
	if val := os.Getenv("CONDUCTOR_PUBLIC_API_ENABLED"); val != "" {
		c.Daemon.Listen.PublicAPI.Enabled = val == "1" || strings.ToLower(val) == "true"
	}
	if val := os.Getenv("CONDUCTOR_PUBLIC_API_TCP"); val != "" {
		c.Daemon.Listen.PublicAPI.TCP = val
	}
	if val := os.Getenv("CONDUCTOR_PID_FILE"); val != "" {
		c.Daemon.PIDFile = val
	}
	if val := os.Getenv("CONDUCTOR_DATA_DIR"); val != "" {
		c.Daemon.DataDir = val
	}
	if val := os.Getenv("CONDUCTOR_WORKFLOWS_DIR"); val != "" {
		c.Daemon.WorkflowsDir = val
	}
	if val := os.Getenv("CONDUCTOR_DAEMON_LOG_LEVEL"); val != "" {
		c.Daemon.DaemonLog.Level = val
	}
	if val := os.Getenv("CONDUCTOR_DAEMON_LOG_FORMAT"); val != "" {
		c.Daemon.DaemonLog.Format = val
	}
	if val := os.Getenv("CONDUCTOR_MAX_CONCURRENT_RUNS"); val != "" {
		if runs, err := strconv.Atoi(val); err == nil {
			c.Daemon.MaxConcurrentRuns = runs
		}
	}
	if val := os.Getenv("CONDUCTOR_DEFAULT_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Daemon.DefaultTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Daemon.ShutdownTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_DRAIN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Daemon.DrainTimeout = duration
		}
	}
	if val := os.Getenv("CONDUCTOR_CHECKPOINTS_ENABLED"); val != "" {
		c.Daemon.CheckpointsEnabled = val == "1" || strings.ToLower(val) == "true"
	}

	// LLM configuration
	if val := os.Getenv("LLM_REQUEST_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.LLM.RequestTimeout = duration
		}
	}
	if val := os.Getenv("LLM_MAX_RETRIES"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			c.LLM.MaxRetries = retries
		}
	}
	if val := os.Getenv("LLM_RETRY_BACKOFF_BASE"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.LLM.RetryBackoffBase = duration
		}
	}
	if val := os.Getenv("LLM_FAILOVER_PROVIDERS"); val != "" {
		// Parse comma-separated list of provider names
		providers := strings.Split(val, ",")
		for i, p := range providers {
			providers[i] = strings.TrimSpace(p)
		}
		c.LLM.FailoverProviders = providers
	}
	if val := os.Getenv("LLM_CIRCUIT_BREAKER_THRESHOLD"); val != "" {
		if threshold, err := strconv.Atoi(val); err == nil {
			c.LLM.CircuitBreakerThreshold = threshold
		}
	}
	if val := os.Getenv("LLM_CIRCUIT_BREAKER_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.LLM.CircuitBreakerTimeout = duration
		}
	}
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	var errs []string

	// Validate server configuration
	if c.Server.Port < 1024 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("server.port must be between 1024 and 65535, got %d", c.Server.Port))
	}
	if c.Server.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("server.shutdown_timeout must be positive, got %v", c.Server.ShutdownTimeout))
	}

	// Validate auth configuration
	if c.Auth.TokenLength < 16 {
		errs = append(errs, fmt.Sprintf("auth.token_length must be at least 16 bytes, got %d", c.Auth.TokenLength))
	}

	// Validate log configuration
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
	if !validLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf("log.level must be one of [debug, info, warn, warning, error], got %q", c.Log.Level))
	}
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Log.Format] {
		errs = append(errs, fmt.Sprintf("log.format must be one of [json, text], got %q", c.Log.Format))
	}

	// Validate LLM configuration
	// llm.default_provider should reference a provider configured in the providers section
	// or be empty (use root-level default_provider instead)
	// Skip validation if no providers are configured yet (allows default config to pass)
	if c.LLM.DefaultProvider != "" && len(c.Providers) > 0 {
		if _, exists := c.Providers[c.LLM.DefaultProvider]; !exists {
			// Build list of configured provider names
			configuredNames := make([]string, 0, len(c.Providers))
			for name := range c.Providers {
				configuredNames = append(configuredNames, name)
			}
			errs = append(errs, fmt.Sprintf("llm.default_provider %q not found in configured providers %v", c.LLM.DefaultProvider, configuredNames))
		}
	}
	if c.LLM.RequestTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("llm.request_timeout must be positive, got %v", c.LLM.RequestTimeout))
	}
	if c.LLM.MaxRetries < 0 {
		errs = append(errs, fmt.Sprintf("llm.max_retries must be non-negative, got %d", c.LLM.MaxRetries))
	}
	if c.LLM.RetryBackoffBase <= 0 {
		errs = append(errs, fmt.Sprintf("llm.retry_backoff_base must be positive, got %v", c.LLM.RetryBackoffBase))
	}

	// Validate multi-provider configuration
	if c.DefaultProvider != "" {
		// default_provider must reference a configured provider
		if _, exists := c.Providers[c.DefaultProvider]; !exists && len(c.Providers) > 0 {
			errs = append(errs, fmt.Sprintf("default_provider %q not found in providers map. Available: %v", c.DefaultProvider, keysOf(c.Providers)))
		}
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

	// Validate public API configuration
	if c.Daemon.Listen.PublicAPI.Enabled {
		if c.Daemon.Listen.PublicAPI.TCP == "" {
			errs = append(errs, "daemon.listen.public_api.tcp is required when public_api.enabled is true")
		}
	}

	// Validate endpoints configuration
	if c.Daemon.Endpoints.Enabled {
		endpointNames := make(map[string]bool)
		for i, ep := range c.Daemon.Endpoints.Endpoints {
			// Validate required fields
			if ep.Name == "" {
				errs = append(errs, fmt.Sprintf("daemon.endpoints.endpoints[%d]: name is required", i))
			} else {
				// Check for duplicate names
				if endpointNames[ep.Name] {
					errs = append(errs, fmt.Sprintf("daemon.endpoints.endpoints[%d]: duplicate endpoint name %q", i, ep.Name))
				}
				endpointNames[ep.Name] = true
			}

			if ep.Workflow == "" {
				errs = append(errs, fmt.Sprintf("daemon.endpoints.endpoints[%d] (%s): workflow is required", i, ep.Name))
			}

			// Validate timeout is non-negative
			if ep.Timeout < 0 {
				errs = append(errs, fmt.Sprintf("daemon.endpoints.endpoints[%d] (%s): timeout must be non-negative, got %v", i, ep.Name, ep.Timeout))
			}

			// Validate rate limit format if specified
			if ep.RateLimit != "" {
				if err := validateRateLimitFormat(ep.RateLimit); err != nil {
					errs = append(errs, fmt.Sprintf("daemon.endpoints.endpoints[%d] (%s): %v", i, ep.Name, err))
				}
			}
		}
	}

	// Validate retention days (must be positive when observability is enabled)
	if c.Daemon.Observability.Enabled {
		ret := c.Daemon.Observability.Storage.Retention
		if ret.TraceDays <= 0 {
			errs = append(errs, fmt.Sprintf("daemon.observability.storage.retention.trace_days must be positive, got %d", ret.TraceDays))
		}
		if ret.EventDays <= 0 {
			errs = append(errs, fmt.Sprintf("daemon.observability.storage.retention.event_days must be positive, got %d", ret.EventDays))
		}
		if ret.AggregateDays <= 0 {
			errs = append(errs, fmt.Sprintf("daemon.observability.storage.retention.aggregate_days must be positive, got %d", ret.AggregateDays))
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

// CheckpointDir returns the checkpoint directory path for the daemon.
func (c *DaemonConfig) CheckpointDir() string {
	if !c.CheckpointsEnabled {
		return ""
	}
	return filepath.Join(c.DataDir, "checkpoints")
}
