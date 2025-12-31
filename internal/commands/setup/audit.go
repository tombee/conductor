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

package setup

import (
	"log/slog"
	"os"
	"time"
)

// AuditLogger logs setup wizard actions for security and troubleshooting.
// All logs are written to stderr with RFC3339 timestamps.
// Credentials are NEVER logged - only metadata about actions.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger for the setup wizard.
func NewAuditLogger() *AuditLogger {
	// Create logger that writes to stderr with RFC3339 timestamps
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format timestamps as RFC3339
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(time.RFC3339))
				}
			}
			return a
		},
	})

	return &AuditLogger{
		logger: slog.New(handler),
	}
}

// LogProviderAdded logs when a provider is added.
func (a *AuditLogger) LogProviderAdded(name, providerType string) {
	a.logger.Info("provider_added",
		"action", "add_provider",
		"provider_name", name,
		"provider_type", providerType,
	)
}

// LogProviderUpdated logs when a provider is updated.
func (a *AuditLogger) LogProviderUpdated(name, providerType string) {
	a.logger.Info("provider_updated",
		"action", "update_provider",
		"provider_name", name,
		"provider_type", providerType,
	)
}

// LogProviderRemoved logs when a provider is removed.
func (a *AuditLogger) LogProviderRemoved(name string) {
	a.logger.Info("provider_removed",
		"action", "remove_provider",
		"provider_name", name,
	)
}

// LogDefaultProviderChanged logs when the default provider is changed.
func (a *AuditLogger) LogDefaultProviderChanged(oldDefault, newDefault string) {
	a.logger.Info("default_provider_changed",
		"action", "change_default_provider",
		"old_default", oldDefault,
		"new_default", newDefault,
	)
}

// LogIntegrationAdded logs when an integration is added.
func (a *AuditLogger) LogIntegrationAdded(name, integrationType string) {
	a.logger.Info("integration_added",
		"action", "add_integration",
		"integration_name", name,
		"integration_type", integrationType,
	)
}

// LogIntegrationUpdated logs when an integration is updated.
func (a *AuditLogger) LogIntegrationUpdated(name, integrationType string) {
	a.logger.Info("integration_updated",
		"action", "update_integration",
		"integration_name", name,
		"integration_type", integrationType,
	)
}

// LogIntegrationRemoved logs when an integration is removed.
func (a *AuditLogger) LogIntegrationRemoved(name string) {
	a.logger.Info("integration_removed",
		"action", "remove_integration",
		"integration_name", name,
	)
}

// LogBackendChanged logs when the secrets backend is changed.
func (a *AuditLogger) LogBackendChanged(oldBackend, newBackend string) {
	a.logger.Info("secrets_backend_changed",
		"action", "change_backend",
		"old_backend", oldBackend,
		"new_backend", newBackend,
	)
}

// LogCredentialStored logs when a credential is stored in a backend.
// The credential value is NEVER logged, only metadata.
func (a *AuditLogger) LogCredentialStored(key, backend string) {
	a.logger.Info("credential_stored",
		"action", "store_credential",
		"credential_key", key,
		"backend", backend,
	)
}

// LogCredentialDeleted logs when a credential is deleted from a backend.
func (a *AuditLogger) LogCredentialDeleted(key, backend string) {
	a.logger.Info("credential_deleted",
		"action", "delete_credential",
		"credential_key", key,
		"backend", backend,
	)
}

// LogConnectionTest logs the result of a connection test.
func (a *AuditLogger) LogConnectionTest(targetType, targetName string, success bool, errorMsg string) {
	if success {
		a.logger.Info("connection_test_success",
			"action", "test_connection",
			"target_type", targetType,
			"target_name", targetName,
			"success", true,
		)
	} else {
		a.logger.Warn("connection_test_failed",
			"action", "test_connection",
			"target_type", targetType,
			"target_name", targetName,
			"success", false,
			"error", errorMsg,
		)
	}
}

// LogModelDiscovery logs the result of model discovery.
func (a *AuditLogger) LogModelDiscovery(providerName string, modelCount int, success bool) {
	if success {
		a.logger.Info("model_discovery_success",
			"action", "discover_models",
			"provider_name", providerName,
			"model_count", modelCount,
			"success", true,
		)
	} else {
		a.logger.Warn("model_discovery_failed",
			"action", "discover_models",
			"provider_name", providerName,
			"success", false,
		)
	}
}

// LogConfigSaved logs when the configuration is saved.
func (a *AuditLogger) LogConfigSaved(path string, providerCount, integrationCount int) {
	a.logger.Info("config_saved",
		"action", "save_config",
		"config_path", path,
		"provider_count", providerCount,
		"integration_count", integrationCount,
	)
}

// LogConfigBackupCreated logs when a config backup is created.
func (a *AuditLogger) LogConfigBackupCreated(backupPath string) {
	a.logger.Info("config_backup_created",
		"action", "create_backup",
		"backup_path", backupPath,
	)
}

// LogPlaintextCredentialDetected logs when a plaintext credential is detected.
func (a *AuditLogger) LogPlaintextCredentialDetected(location string) {
	a.logger.Warn("plaintext_credential_detected",
		"action", "detect_plaintext",
		"location", location,
	)
}

// LogPlaintextCredentialMigrated logs when a plaintext credential is migrated to secure storage.
func (a *AuditLogger) LogPlaintextCredentialMigrated(location, backend string) {
	a.logger.Info("plaintext_credential_migrated",
		"action", "migrate_plaintext",
		"location", location,
		"backend", backend,
	)
}

// LogSetupStarted logs when the setup wizard starts.
func (a *AuditLogger) LogSetupStarted(hasExistingConfig bool) {
	a.logger.Info("setup_started",
		"action", "start_setup",
		"has_existing_config", hasExistingConfig,
	)
}

// LogSetupCompleted logs when the setup wizard completes successfully.
func (a *AuditLogger) LogSetupCompleted() {
	a.logger.Info("setup_completed",
		"action", "complete_setup",
	)
}

// LogSetupCanceled logs when the setup wizard is canceled by the user.
func (a *AuditLogger) LogSetupCanceled(hadUnsavedChanges bool) {
	a.logger.Info("setup_canceled",
		"action", "cancel_setup",
		"had_unsaved_changes", hadUnsavedChanges,
	)
}
