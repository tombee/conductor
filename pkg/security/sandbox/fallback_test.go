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

func TestFallbackFactory(t *testing.T) {
	t.Run("Type", func(t *testing.T) {
		factory := NewFallbackFactory()
		if factory.Type() != TypeFallback {
			t.Errorf("expected type %s, got %s", TypeFallback, factory.Type())
		}
	})

	t.Run("Available", func(t *testing.T) {
		factory := NewFallbackFactory()
		ctx := context.Background()
		if !factory.Available(ctx) {
			t.Error("fallback factory should always be available")
		}
	})

	t.Run("Create", func(t *testing.T) {
		factory := NewFallbackFactory()
		ctx := context.Background()

		cfg := Config{
			WorkflowID: "test-workflow",
			Env: map[string]string{
				"TEST_VAR": "value",
			},
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		if sb == nil {
			t.Fatal("sandbox should not be nil")
		}

		// Verify it's a fallback sandbox
		_, ok := sb.(*fallbackSandbox)
		if !ok {
			t.Error("expected fallbackSandbox type")
		}
	})
}

func TestFallbackSandbox_Execute(t *testing.T) {
	factory := NewFallbackFactory()
	ctx := context.Background()

	cfg := Config{
		WorkflowID: "test-workflow",
		Env: map[string]string{
			"TEST_VAR": "hello",
		},
	}

	sb, err := factory.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	defer sb.Cleanup()

	t.Run("SimpleCommand", func(t *testing.T) {
		output, err := sb.Execute(ctx, "echo", []string{"test"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "test" {
			t.Errorf("expected 'test', got '%s'", result)
		}
	})

	t.Run("CommandWithEnv", func(t *testing.T) {
		// Test that custom env vars are set
		output, err := sb.Execute(ctx, "sh", []string{"-c", "echo $TEST_VAR"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "hello" {
			t.Errorf("expected 'hello', got '%s'", result)
		}
	})

	t.Run("CredentialEnvFiltering", func(t *testing.T) {
		// Create sandbox with credential env vars (should be filtered)
		cfgWithCreds := Config{
			WorkflowID: "test-workflow",
			Env: map[string]string{
				"AWS_SECRET_ACCESS_KEY": "secret123",
				"SAFE_VAR":              "safe",
			},
		}

		sb2, err := factory.Create(ctx, cfgWithCreds)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb2.Cleanup()

		// Try to access AWS_SECRET_ACCESS_KEY (should be empty/filtered)
		output, err := sb2.Execute(ctx, "sh", []string{"-c", "echo ${AWS_SECRET_ACCESS_KEY:-FILTERED}"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result := strings.TrimSpace(string(output))
		if result != "FILTERED" {
			t.Errorf("AWS_SECRET_ACCESS_KEY should be filtered, got: %s", result)
		}

		// Safe var should still be accessible
		output, err = sb2.Execute(ctx, "sh", []string{"-c", "echo $SAFE_VAR"})
		if err != nil {
			t.Fatalf("execute failed: %v", err)
		}

		result = strings.TrimSpace(string(output))
		if result != "safe" {
			t.Errorf("expected 'safe', got '%s'", result)
		}
	})

	t.Run("CommandError", func(t *testing.T) {
		_, err := sb.Execute(ctx, "false", []string{})
		if err == nil {
			t.Error("expected error from 'false' command")
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()

		_, err := sb.Execute(timeoutCtx, "sleep", []string{"10"})
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestFallbackSandbox_FileOperations(t *testing.T) {
	factory := NewFallbackFactory()
	ctx := context.Background()

	// Create temp directory for workspace
	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := Config{
		WorkflowID: "test-workflow",
		WorkDir:    tmpDir,
	}

	sb, err := factory.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	defer sb.Cleanup()

	t.Run("WriteAndRead", func(t *testing.T) {
		content := []byte("test content")
		path := "test.txt"

		// Write file
		if err := sb.WriteFile(path, content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Read file
		readContent, err := sb.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(readContent) != string(content) {
			t.Errorf("content mismatch: expected %q, got %q", content, readContent)
		}
	})

	t.Run("WriteNestedPath", func(t *testing.T) {
		content := []byte("nested content")
		path := "subdir/nested/file.txt"

		// Write file (should create directories)
		if err := sb.WriteFile(path, content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file exists
		fullPath := filepath.Join(tmpDir, path)
		if _, err := os.Stat(fullPath); err != nil {
			t.Errorf("file not created: %v", err)
		}

		// Read back
		readContent, err := sb.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(readContent) != string(content) {
			t.Errorf("content mismatch")
		}
	})

	t.Run("ReadNonexistent", func(t *testing.T) {
		_, err := sb.ReadFile("nonexistent.txt")
		if err == nil {
			t.Error("expected error reading nonexistent file")
		}
	})
}

func TestFallbackSandbox_Cleanup(t *testing.T) {
	factory := NewFallbackFactory()
	ctx := context.Background()

	cfg := Config{
		WorkflowID: "test-workflow",
	}

	sb, err := factory.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}

	// Get the temp directory path before cleanup
	fbs := sb.(*fallbackSandbox)
	tmpDir := fbs.tmpDir

	// Verify temp dir exists
	if _, err := os.Stat(tmpDir); err != nil {
		t.Fatalf("temp dir should exist before cleanup: %v", err)
	}

	// Cleanup
	if err := sb.Cleanup(); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify temp dir is removed
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Error("temp dir should be removed after cleanup")
	}

	// Second cleanup should be safe (idempotent)
	if err := sb.Cleanup(); err != nil {
		t.Errorf("second Cleanup should not error: %v", err)
	}
}

func TestFallbackSandbox_WorkDirVsTmpDir(t *testing.T) {
	factory := NewFallbackFactory()
	ctx := context.Background()

	t.Run("WithWorkDir", func(t *testing.T) {
		tmpWorkDir, err := os.MkdirTemp("", "workdir-*")
		if err != nil {
			t.Fatalf("failed to create temp workdir: %v", err)
		}
		defer os.RemoveAll(tmpWorkDir)

		cfg := Config{
			WorkflowID: "test-workflow",
			WorkDir:    tmpWorkDir,
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		// Write file
		content := []byte("test")
		if err := sb.WriteFile("test.txt", content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file is in work dir, not sandbox temp dir
		filePath := filepath.Join(tmpWorkDir, "test.txt")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("file should be in work dir: %v", err)
		}
	})

	t.Run("WithoutWorkDir", func(t *testing.T) {
		cfg := Config{
			WorkflowID: "test-workflow",
			// No WorkDir specified
		}

		sb, err := factory.Create(ctx, cfg)
		if err != nil {
			t.Fatalf("failed to create sandbox: %v", err)
		}
		defer sb.Cleanup()

		fbs := sb.(*fallbackSandbox)
		tmpDir := fbs.tmpDir

		// Write file
		content := []byte("test")
		if err := sb.WriteFile("test.txt", content); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file is in sandbox temp dir
		filePath := filepath.Join(tmpDir, "test.txt")
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("file should be in temp dir: %v", err)
		}
	})
}
