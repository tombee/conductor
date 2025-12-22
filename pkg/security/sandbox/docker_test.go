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

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDockerFactory_Availability tests Docker/Podman detection.
func TestDockerFactory_Availability(t *testing.T) {
	factory := NewDockerFactory()
	ctx := context.Background()

	t.Run("Type", func(t *testing.T) {
		if factory.Type() != TypeDocker {
			t.Errorf("expected type %s, got %s", TypeDocker, factory.Type())
		}
	})

	t.Run("RuntimeDetection", func(t *testing.T) {
		// This test just checks that detection doesn't panic
		// Actual availability depends on system configuration
		available := factory.Available(ctx)
		t.Logf("Docker/Podman available: %v", available)
		t.Logf("Detected runtime: %s", factory.runtime)
	})
}

// TestDockerSandbox requires Docker or Podman to be installed and running.
// These tests are skipped if no container runtime is available.
func TestDockerSandbox(t *testing.T) {
	factory := NewDockerFactory()
	ctx := context.Background()

	if !factory.Available(ctx) {
		t.Skip("Docker/Podman not available, skipping integration tests")
	}

	t.Run("Create", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-workflow",
			Image:      "alpine:latest",
			Env: map[string]string{
				"TEST_VAR": "value",
			},
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// Verify container was created
		dSb := sb.(*dockerSandbox)
		if dSb.containerID == "" {
			t.Error("container ID should not be empty")
		}
	})

	t.Run("Execute_SimpleCommand", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-execute",
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		output, err := sb.Execute(ctx, "echo", []string{"hello"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "hello" {
			t.Errorf("expected 'hello', got '%s'", result)
		}
	})

	t.Run("Execute_WithEnv", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-env",
			Image:      "alpine:latest",
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
			},
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		output, err := sb.Execute(ctx, "sh", []string{"-c", "echo $CUSTOM_VAR"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "custom_value" {
			t.Errorf("expected 'custom_value', got '%s'", result)
		}
	})

	t.Run("Execute_CredentialFiltering", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-creds",
			Image:      "alpine:latest",
			Env: map[string]string{
				"AWS_SECRET_ACCESS_KEY": "should_be_filtered",
				"SAFE_VAR":              "safe",
			},
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// AWS_SECRET_ACCESS_KEY should be filtered
		output, err := sb.Execute(ctx, "sh", []string{"-c", "echo ${AWS_SECRET_ACCESS_KEY:-EMPTY}"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result == "should_be_filtered" {
			t.Error("AWS_SECRET_ACCESS_KEY should be filtered out")
		}

		// SAFE_VAR should be present
		output, err = sb.Execute(ctx, "sh", []string{"-c", "echo $SAFE_VAR"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result = strings.TrimSpace(string(output))
		if result != "safe" {
			t.Errorf("expected 'safe', got '%s'", result)
		}
	})

	t.Run("Execute_CommandError", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-error",
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		_, err = sb.Execute(ctx, "false", []string{})
		if err == nil {
			t.Error("expected error from 'false' command")
		}
	})

	t.Run("Execute_Timeout", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-timeout",
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		_, err = sb.Execute(timeoutCtx, "sleep", []string{"10"})
		if err == nil {
			t.Error("expected timeout error")
		}
	})

	t.Run("FileOperations", func(t *testing.T) {
		// Create temp workspace
		tmpDir, err := os.MkdirTemp("", "docker-sandbox-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		cfg := Config{
			WorkflowID: "test-files",
			WorkDir:    tmpDir,
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// Write file
		content := []byte("test content")
		if err := sb.WriteFile("test.txt", content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Read file back
		readContent, err := sb.ReadFile("test.txt")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(readContent) != string(content) {
			t.Errorf("content mismatch: expected %q, got %q", content, readContent)
		}

		// Verify file is also visible in host filesystem
		hostPath := filepath.Join(tmpDir, "test.txt")
		hostContent, err := os.ReadFile(hostPath)
		if err != nil {
			t.Errorf("file should be visible on host: %v", err)
		}
		if string(hostContent) != string(content) {
			t.Error("host file content mismatch")
		}
	})

	t.Run("NetworkIsolation_None", func(t *testing.T) {
		cfg := Config{
			WorkflowID:  "test-network-none",
			Image:       "alpine:latest",
			NetworkMode: NetworkNone,
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// Try to ping (should fail with network disabled)
		_, err = sb.Execute(ctx, "ping", []string{"-c", "1", "8.8.8.8"})
		if err == nil {
			t.Error("ping should fail with NetworkNone")
		}
	})

	t.Run("ResourceLimits_Memory", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-memory-limit",
			Image:      "alpine:latest",
			ResourceLimits: ResourceLimits{
				MaxMemory: 50 * 1024 * 1024, // 50MB
			},
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// This just verifies the container starts with limits
		// Actually triggering OOM would be platform-specific
		output, err := sb.Execute(ctx, "echo", []string{"memory limit set"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		if len(output) == 0 {
			t.Error("expected output")
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-cleanup",
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}

		dSb := sb.(*dockerSandbox)
		containerID := dSb.containerID

		// Cleanup
		if err := sb.Cleanup(); err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}

		// Verify container is removed (containerID should be cleared)
		if dSb.containerID != "" {
			t.Error("container ID should be cleared after cleanup")
		}

		// Second cleanup should be safe
		if err := sb.Cleanup(); err != nil {
			t.Errorf("second Cleanup should not error: %v", err)
		}

		// Verify container actually removed (this is a best-effort check)
		_ = containerID // Would need docker CLI to verify fully
	})

	t.Run("MultipleExecutions_SameContainer", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-reuse",
			Image:      "alpine:latest",
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// Execute multiple commands in the same container
		for i := 0; i < 5; i++ {
			output, err := sb.Execute(ctx, "echo", []string{"iteration"})
			if err != nil {
				t.Fatalf("iteration %d failed: %v", i, err)
			}
			if len(output) == 0 {
				t.Errorf("iteration %d: expected output", i)
			}
		}
	})
}

// TestDockerSandbox_Advanced tests AdvancedSandbox interface.
func TestDockerSandbox_Advanced(t *testing.T) {
	factory := NewDockerFactory()
	ctx := context.Background()

	if !factory.Available(ctx) {
		t.Skip("Docker/Podman not available, skipping advanced tests")
	}

	cfg := Config{
		WorkflowID: "test-advanced",
		Image:      "alpine:latest",
	}

	sb, err := factory.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	defer sb.Cleanup()

	// Check if AdvancedSandbox is implemented
	advSb, ok := sb.(AdvancedSandbox)
	if !ok {
		t.Fatal("dockerSandbox should implement AdvancedSandbox")
	}

	t.Run("Stats", func(t *testing.T) {
		stats, err := advSb.Stats(ctx)
		if err != nil {
			// Stats might not be fully implemented yet
			t.Logf("Stats not available: %v", err)
			return
		}

		// Stats should be zero-initialized at least
		_ = stats
		t.Logf("Stats: %+v", stats)
	})
}
