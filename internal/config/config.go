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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidConfig is returned when configuration validation fails.
	ErrInvalidConfig = errors.New("config: invalid configuration")
)

// Config represents the complete Conductor configuration.
type Config struct {
	Server ServerConfig `yaml:"server"`
	Auth   AuthConfig   `yaml:"auth"`
	Log    LogConfig    `yaml:"log"`
	LLM    LLMConfig    `yaml:"llm"`
}

// ServerConfig configures the RPC server behavior.
type ServerConfig struct {
	// PortRange specifies the range of ports to try (inclusive).
	// Environment: SERVER_PORT_MIN, SERVER_PORT_MAX
	// Default: [9876, 9899]
	PortRange [2]int `yaml:"port_range"`

	// HealthCheckInterval is the interval between health check polls.
	// Environment: SERVER_HEALTH_CHECK_INTERVAL
	// Default: 500ms
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	// Environment: SERVER_SHUTDOWN_TIMEOUT
	// Default: 5s
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`

	// ReadTimeout is the maximum duration for reading requests.
	// Environment: SERVER_READ_TIMEOUT
	// Default: 10s
	ReadTimeout time.Duration `yaml:"read_timeout"`
}

// AuthConfig configures authentication settings.
type AuthConfig struct {
	// TokenLength is the length of generated auth tokens in bytes.
	// Environment: AUTH_TOKEN_LENGTH
	// Default: 32
	TokenLength int `yaml:"token_length"`

	// RateLimitMaxAttempts is the maximum failed auth attempts per window.
	// Environment: AUTH_RATE_LIMIT_MAX_ATTEMPTS
	// Default: 5
	RateLimitMaxAttempts int `yaml:"rate_limit_max_attempts"`

	// RateLimitWindow is the time window for rate limiting.
	// Environment: AUTH_RATE_LIMIT_WINDOW
	// Default: 1m
	RateLimitWindow time.Duration `yaml:"rate_limit_window"`

	// RateLimitLockout is the lockout duration after exceeding rate limit.
	// Environment: AUTH_RATE_LIMIT_LOCKOUT
	// Default: 60s
	RateLimitLockout time.Duration `yaml:"rate_limit_lockout"`
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
// Prepared for Phase 1b implementation.
type LLMConfig struct {
	// DefaultProvider is the default LLM provider to use.
	// Environment: LLM_DEFAULT_PROVIDER
	// Default: anthropic
	DefaultProvider string `yaml:"default_provider"`

	// RequestTimeout is the maximum duration for LLM requests.
	// Environment: LLM_REQUEST_TIMEOUT
	// Default: 5s
	RequestTimeout time.Duration `yaml:"request_timeout"`

	// MaxRetries is the maximum number of retry attempts for failed requests.
	// Environment: LLM_MAX_RETRIES
	// Default: 3
	MaxRetries int `yaml:"max_retries"`

	// RetryBackoffBase is the base duration for exponential backoff.
	// Environment: LLM_RETRY_BACKOFF_BASE
	// Default: 100ms
	RetryBackoffBase time.Duration `yaml:"retry_backoff_base"`

	// ConnectionPoolSize is the number of HTTP connections per provider.
	// Environment: LLM_CONNECTION_POOL_SIZE
	// Default: 10
	ConnectionPoolSize int `yaml:"connection_pool_size"`

	// ConnectionIdleTimeout is the idle timeout for pooled connections.
	// Environment: LLM_CONNECTION_IDLE_TIMEOUT
	// Default: 30s
	ConnectionIdleTimeout time.Duration `yaml:"connection_idle_timeout"`

	// TraceRetentionDays is the number of days to retain request traces.
	// Environment: LLM_TRACE_RETENTION_DAYS
	// Default: 7
	TraceRetentionDays int `yaml:"trace_retention_days"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			PortRange:           [2]int{9876, 9899},
			HealthCheckInterval: 500 * time.Millisecond,
			ShutdownTimeout:     5 * time.Second,
			ReadTimeout:         10 * time.Second,
		},
		Auth: AuthConfig{
			TokenLength:          32,
			RateLimitMaxAttempts: 5,
			RateLimitWindow:      1 * time.Minute,
			RateLimitLockout:     60 * time.Second,
		},
		Log: LogConfig{
			Level:     "info",
			Format:    "json",
			AddSource: false,
		},
		LLM: LLMConfig{
			DefaultProvider:       "anthropic",
			RequestTimeout:        5 * time.Second,
			MaxRetries:            3,
			RetryBackoffBase:      100 * time.Millisecond,
			ConnectionPoolSize:    10,
			ConnectionIdleTimeout: 30 * time.Second,
			TraceRetentionDays:    7,
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
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
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
	if val := os.Getenv("SERVER_PORT_MIN"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.Server.PortRange[0] = port
		}
	}
	if val := os.Getenv("SERVER_PORT_MAX"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.Server.PortRange[1] = port
		}
	}
	if val := os.Getenv("SERVER_HEALTH_CHECK_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Server.HealthCheckInterval = duration
		}
	}
	if val := os.Getenv("SERVER_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Server.ShutdownTimeout = duration
		}
	}
	if val := os.Getenv("SERVER_READ_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Server.ReadTimeout = duration
		}
	}

	// Auth configuration
	if val := os.Getenv("AUTH_TOKEN_LENGTH"); val != "" {
		if length, err := strconv.Atoi(val); err == nil {
			c.Auth.TokenLength = length
		}
	}
	if val := os.Getenv("AUTH_RATE_LIMIT_MAX_ATTEMPTS"); val != "" {
		if attempts, err := strconv.Atoi(val); err == nil {
			c.Auth.RateLimitMaxAttempts = attempts
		}
	}
	if val := os.Getenv("AUTH_RATE_LIMIT_WINDOW"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Auth.RateLimitWindow = duration
		}
	}
	if val := os.Getenv("AUTH_RATE_LIMIT_LOCKOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.Auth.RateLimitLockout = duration
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
	if val := os.Getenv("LLM_CONNECTION_POOL_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			c.LLM.ConnectionPoolSize = size
		}
	}
	if val := os.Getenv("LLM_CONNECTION_IDLE_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			c.LLM.ConnectionIdleTimeout = duration
		}
	}
	if val := os.Getenv("LLM_TRACE_RETENTION_DAYS"); val != "" {
		if days, err := strconv.Atoi(val); err == nil {
			c.LLM.TraceRetentionDays = days
		}
	}
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	var errs []string

	// Validate server configuration
	if c.Server.PortRange[0] < 1024 || c.Server.PortRange[0] > 65535 {
		errs = append(errs, fmt.Sprintf("server.port_range[0] must be between 1024 and 65535, got %d", c.Server.PortRange[0]))
	}
	if c.Server.PortRange[1] < 1024 || c.Server.PortRange[1] > 65535 {
		errs = append(errs, fmt.Sprintf("server.port_range[1] must be between 1024 and 65535, got %d", c.Server.PortRange[1]))
	}
	if c.Server.PortRange[0] > c.Server.PortRange[1] {
		errs = append(errs, fmt.Sprintf("server.port_range[0] (%d) must be <= port_range[1] (%d)", c.Server.PortRange[0], c.Server.PortRange[1]))
	}
	if c.Server.HealthCheckInterval <= 0 {
		errs = append(errs, fmt.Sprintf("server.health_check_interval must be positive, got %v", c.Server.HealthCheckInterval))
	}
	if c.Server.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("server.shutdown_timeout must be positive, got %v", c.Server.ShutdownTimeout))
	}
	if c.Server.ReadTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("server.read_timeout must be positive, got %v", c.Server.ReadTimeout))
	}

	// Validate auth configuration
	if c.Auth.TokenLength < 16 {
		errs = append(errs, fmt.Sprintf("auth.token_length must be at least 16 bytes, got %d", c.Auth.TokenLength))
	}
	if c.Auth.RateLimitMaxAttempts <= 0 {
		errs = append(errs, fmt.Sprintf("auth.rate_limit_max_attempts must be positive, got %d", c.Auth.RateLimitMaxAttempts))
	}
	if c.Auth.RateLimitWindow <= 0 {
		errs = append(errs, fmt.Sprintf("auth.rate_limit_window must be positive, got %v", c.Auth.RateLimitWindow))
	}
	if c.Auth.RateLimitLockout <= 0 {
		errs = append(errs, fmt.Sprintf("auth.rate_limit_lockout must be positive, got %v", c.Auth.RateLimitLockout))
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
	validProviders := map[string]bool{"anthropic": true, "openai": true, "ollama": true}
	if !validProviders[c.LLM.DefaultProvider] {
		errs = append(errs, fmt.Sprintf("llm.default_provider must be one of [anthropic, openai, ollama], got %q", c.LLM.DefaultProvider))
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
	if c.LLM.ConnectionPoolSize <= 0 {
		errs = append(errs, fmt.Sprintf("llm.connection_pool_size must be positive, got %d", c.LLM.ConnectionPoolSize))
	}
	if c.LLM.ConnectionIdleTimeout <= 0 {
		errs = append(errs, fmt.Sprintf("llm.connection_idle_timeout must be positive, got %v", c.LLM.ConnectionIdleTimeout))
	}
	if c.LLM.TraceRetentionDays < 0 {
		errs = append(errs, fmt.Sprintf("llm.trace_retention_days must be non-negative, got %d", c.LLM.TraceRetentionDays))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w:\n  - %s", ErrInvalidConfig, strings.Join(errs, "\n  - "))
	}

	return nil
}
