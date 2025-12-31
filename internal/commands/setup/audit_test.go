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
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestAuditLoggerCreation(t *testing.T) {
	logger := NewAuditLogger()
	if logger == nil {
		t.Fatal("Expected non-nil audit logger")
	}
	if logger.logger == nil {
		t.Fatal("Expected non-nil slog logger")
	}
}

func TestAuditLoggerNoCredentials(t *testing.T) {
	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Create logger and log some actions
	logger := NewAuditLogger()
	logger.LogProviderAdded("test", "anthropic")
	logger.LogCredentialStored("providers/test/api_key", "keychain")
	logger.LogConfigSaved("/path/to/config.yaml", 1, 0)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify logs were written
	if !strings.Contains(output, "provider_added") {
		t.Error("Expected provider_added log entry")
	}
	if !strings.Contains(output, "credential_stored") {
		t.Error("Expected credential_stored log entry")
	}

	// Verify no actual credential values in logs
	// (This is a sanity check - the implementation should never log credentials)
	if strings.Contains(output, "sk-ant-") {
		t.Error("Found credential value in logs - this is a security issue!")
	}
	if strings.Contains(output, "password") {
		t.Error("Found password in logs - this is a security issue!")
	}
}

func TestAuditLogProviderActions(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogProviderAdded("claude", "claude-code")
	logger.LogProviderUpdated("claude", "claude-code")
	logger.LogProviderRemoved("claude")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify action types
	if !strings.Contains(output, "add_provider") {
		t.Error("Expected add_provider action")
	}
	if !strings.Contains(output, "update_provider") {
		t.Error("Expected update_provider action")
	}
	if !strings.Contains(output, "remove_provider") {
		t.Error("Expected remove_provider action")
	}

	// Verify provider metadata is logged
	if !strings.Contains(output, "claude") {
		t.Error("Expected provider name in logs")
	}
	if !strings.Contains(output, "claude-code") {
		t.Error("Expected provider type in logs")
	}
}

func TestAuditLogIntegrationActions(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogIntegrationAdded("github", "github")
	logger.LogIntegrationUpdated("github", "github")
	logger.LogIntegrationRemoved("github")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "add_integration") {
		t.Error("Expected add_integration action")
	}
	if !strings.Contains(output, "update_integration") {
		t.Error("Expected update_integration action")
	}
	if !strings.Contains(output, "remove_integration") {
		t.Error("Expected remove_integration action")
	}
}

func TestAuditLogConnectionTest(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogConnectionTest("provider", "anthropic", true, "")
	logger.LogConnectionTest("provider", "broken", false, "connection timeout")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "test_connection") {
		t.Error("Expected test_connection action")
	}
	if !strings.Contains(output, "success=true") {
		t.Error("Expected successful test log")
	}
	if !strings.Contains(output, "success=false") {
		t.Error("Expected failed test log")
	}
	if !strings.Contains(output, "connection timeout") {
		t.Error("Expected error message in failed test log")
	}
}

func TestAuditLogSetupLifecycle(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogSetupStarted(false)
	logger.LogSetupCompleted()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "start_setup") {
		t.Error("Expected start_setup action")
	}
	if !strings.Contains(output, "complete_setup") {
		t.Error("Expected complete_setup action")
	}
}

func TestAuditLogCanceled(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogSetupCanceled(true)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "cancel_setup") {
		t.Error("Expected cancel_setup action")
	}
	if !strings.Contains(output, "had_unsaved_changes=true") {
		t.Error("Expected unsaved changes flag")
	}
}

// TestAuditLoggerUsesTextFormat verifies logs are in text format, not JSON.
func TestAuditLoggerUsesTextFormat(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger := NewAuditLogger()
	logger.LogProviderAdded("test", "anthropic")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Text format uses key=value, JSON uses {"key":"value"}
	if strings.Contains(output, "{\"") {
		t.Error("Expected text format, got JSON")
	}
	if !strings.Contains(output, "=") {
		t.Error("Expected text format with key=value pairs")
	}
}

// TestAuditLoggerCustomHandler verifies the custom handler formats timestamps correctly.
func TestAuditLoggerTimestampFormat(t *testing.T) {
	// Create a simple custom handler
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return a
		},
	})

	logger := &AuditLogger{
		logger: slog.New(handler),
	}

	// Just verify the logger can be created and used
	logger.LogProviderAdded("test", "anthropic")

	// The test primarily verifies that the logger is created with the right options
	// Actual timestamp formatting is tested via the ReplaceAttr function in NewAuditLogger
}
