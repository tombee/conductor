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

// Package config provides daemon-specific configuration.
//
// Deprecated: This package is deprecated. Use github.com/tombee/conductor/internal/config instead.
// The configuration system has been consolidated into internal/config for consistency.
// Migration guide:
//   - Replace "internal/daemon/config" with "internal/config"
//   - Use config.LoadDaemon() instead of daemon/config.Load()
//   - All types are now in the internal/config package
package config

import (
	mainconfig "github.com/tombee/conductor/internal/config"
)

// Deprecated type aliases for backward compatibility.
// These types now point to the unified config in internal/config.

// Config holds daemon configuration.
// Deprecated: Use github.com/tombee/conductor/internal/config.Config instead.
type Config = mainconfig.Config

// AuthConfig configures authentication.
// Deprecated: Use github.com/tombee/conductor/internal/config.DaemonAuthConfig instead.
type AuthConfig = mainconfig.DaemonAuthConfig

// BackendConfig configures the storage backend.
// Deprecated: Use github.com/tombee/conductor/internal/config.BackendConfig instead.
type BackendConfig = mainconfig.BackendConfig

// PostgresConfig contains PostgreSQL connection settings.
// Deprecated: Use github.com/tombee/conductor/internal/config.PostgresConfig instead.
type PostgresConfig = mainconfig.PostgresConfig

// DistributedConfig configures distributed mode settings.
// Deprecated: Use github.com/tombee/conductor/internal/config.DistributedConfig instead.
type DistributedConfig = mainconfig.DistributedConfig

// ObservabilityConfig configures tracing and observability.
// Deprecated: Use github.com/tombee/conductor/internal/config.ObservabilityConfig instead.
type ObservabilityConfig = mainconfig.ObservabilityConfig

// SamplingConfig controls which traces are recorded.
// Deprecated: Use github.com/tombee/conductor/internal/config.SamplingConfig instead.
type SamplingConfig = mainconfig.SamplingConfig

// StorageConfig controls local trace storage.
// Deprecated: Use github.com/tombee/conductor/internal/config.StorageConfig instead.
type StorageConfig = mainconfig.StorageConfig

// RetentionConfig defines data retention policies.
// Deprecated: Use github.com/tombee/conductor/internal/config.RetentionConfig instead.
type RetentionConfig = mainconfig.RetentionConfig

// ExporterConfig defines an OTLP export destination.
// Deprecated: Use github.com/tombee/conductor/internal/config.ExporterConfig instead.
type ExporterConfig = mainconfig.ExporterConfig

// TLSConfig configures TLS for exporters.
// Deprecated: Use github.com/tombee/conductor/internal/config.TLSConfig instead.
type TLSConfig = mainconfig.TLSConfig

// RedactionConfig controls sensitive data redaction.
// Deprecated: Use github.com/tombee/conductor/internal/config.RedactionConfig instead.
type RedactionConfig = mainconfig.RedactionConfig

// RedactionPattern defines a sensitive data pattern.
// Deprecated: Use github.com/tombee/conductor/internal/config.RedactionPattern instead.
type RedactionPattern = mainconfig.RedactionPattern

// SchedulesConfig configures workflow scheduling.
// Deprecated: Use github.com/tombee/conductor/internal/config.SchedulesConfig instead.
type SchedulesConfig = mainconfig.SchedulesConfig

// ScheduleEntry defines a scheduled workflow.
// Deprecated: Use github.com/tombee/conductor/internal/config.ScheduleEntry instead.
type ScheduleEntry = mainconfig.ScheduleEntry

// WebhooksConfig configures webhook handling.
// Deprecated: Use github.com/tombee/conductor/internal/config.WebhooksConfig instead.
type WebhooksConfig = mainconfig.WebhooksConfig

// WebhookRoute defines a webhook route mapping.
// Deprecated: Use github.com/tombee/conductor/internal/config.WebhookRoute instead.
type WebhookRoute = mainconfig.WebhookRoute

// ListenConfig configures how the daemon listens for connections.
// Deprecated: Use github.com/tombee/conductor/internal/config.DaemonListenConfig instead.
type ListenConfig = mainconfig.DaemonListenConfig

// LogConfig configures daemon logging.
// Deprecated: Use github.com/tombee/conductor/internal/config.DaemonLogConfig instead.
type LogConfig = mainconfig.DaemonLogConfig

// DefaultConfig returns a configuration with sensible defaults.
// Deprecated: Use github.com/tombee/conductor/internal/config.Default() instead.
func DefaultConfig() *Config {
	cfg := mainconfig.Default()
	return cfg
}

// Load loads daemon configuration from defaults and environment variables.
// Deprecated: Use github.com/tombee/conductor/internal/config.LoadDaemon() instead.
func Load() (*Config, error) {
	return mainconfig.LoadDaemon("")
}
