package file

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObservability_Integration(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Set up audit logging
	var auditBuf bytes.Buffer
	auditSlogger := slog.New(slog.NewJSONHandler(&auditBuf, nil))
	auditLogger := NewSlogAuditLogger(auditSlogger)

	// Set up quota tracking
	var quotaBuf bytes.Buffer
	quotaSlogger := slog.New(slog.NewTextHandler(&quotaBuf, nil))
	quotaConfig := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
		Logger:         quotaSlogger,
	}

	// Create connector with observability features
	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   outDir,
		AuditLogger: auditLogger,
		QuotaConfig: quotaConfig,
	}

	connector, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Perform a write operation
	ctx := context.Background()
	result, err := connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "$out/test.txt",
		"content": "Hello, World!",
	})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check audit log
	auditOutput := auditBuf.String()
	if !strings.Contains(auditOutput, "write_text") {
		t.Errorf("Expected audit log to contain operation name, got: %s", auditOutput)
	}
	if !strings.Contains(auditOutput, "success") {
		t.Errorf("Expected audit log to show success, got: %s", auditOutput)
	}

	// Write more data to trigger quota warning
	largeContent := strings.Repeat("x", 850) // Total will be ~863 bytes (> 80%)
	_, err = connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "$out/large.txt",
		"content": largeContent,
	})

	if err != nil {
		t.Errorf("Expected no error for write within quota, got: %v", err)
	}

	// Check quota warning was logged
	quotaOutput := quotaBuf.String()
	if !strings.Contains(quotaOutput, "disk quota warning") {
		t.Errorf("Expected quota warning, got: %s", quotaOutput)
	}

	// Try to exceed quota
	tooLarge := strings.Repeat("y", 200) // Would exceed 95%
	_, err = connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "$out/toolarge.txt",
		"content": tooLarge,
	})

	if err == nil {
		t.Error("Expected error when exceeding quota")
	}

	if opErr, ok := err.(*OperationError); ok {
		if opErr.ErrorType != ErrorTypeDiskFull {
			t.Errorf("Expected ErrorTypeDiskFull, got %s: %v", opErr.ErrorType, err)
		}
	} else {
		t.Errorf("Expected OperationError, got %T: %v", err, err)
	}
}

func TestObservability_WithoutAuditLogger(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create connector without audit logger (should use noop)
	config := &Config{
		WorkflowDir: tmpDir,
		AuditLogger: nil, // Will default to NoopAuditLogger
	}

	connector, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Should work fine without audit logging
	ctx := context.Background()
	_, err = connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "test.txt",
		"content": "Hello!",
	})

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestObservability_WithoutQuotaTracker(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create connector without quota tracker
	config := &Config{
		WorkflowDir: tmpDir,
		QuotaConfig: nil, // No quota tracking
	}

	connector, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Should work fine without quota tracking
	ctx := context.Background()
	largeContent := strings.Repeat("x", 100000) // Large content
	_, err = connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "test.txt",
		"content": largeContent,
	})

	if err != nil {
		t.Errorf("Expected no error without quota tracking, got: %v", err)
	}
}

func TestObservability_ErrorMetrics(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Set up audit logging to capture errors
	var auditBuf bytes.Buffer
	auditSlogger := slog.New(slog.NewJSONHandler(&auditBuf, nil))
	auditLogger := NewSlogAuditLogger(auditSlogger)

	config := &Config{
		WorkflowDir: tmpDir,
		AuditLogger: auditLogger,
	}

	connector, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	// Try to read a non-existent file
	ctx := context.Background()
	_, err = connector.Execute(ctx, "read_text", map[string]interface{}{
		"path": "missing.txt",
	})

	if err == nil {
		t.Error("Expected error reading missing file")
	}

	// Check that error was logged
	auditOutput := auditBuf.String()
	if !strings.Contains(auditOutput, "error") {
		t.Errorf("Expected error in audit log, got: %s", auditOutput)
	}
	if !strings.Contains(auditOutput, "file not found") {
		t.Errorf("Expected 'file not found' in audit log, got: %s", auditOutput)
	}
}

func TestObservability_AppendWithQuota(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Set up quota tracking
	quotaConfig := &QuotaConfig{
		DefaultQuota:   100,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   outDir,
		QuotaConfig: quotaConfig,
	}

	connector, err := New(config)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Write initial content
	_, err = connector.Execute(ctx, "write_text", map[string]interface{}{
		"path":    "$out/test.txt",
		"content": strings.Repeat("a", 50),
	})
	if err != nil {
		t.Fatalf("Initial write failed: %v", err)
	}

	// Append within quota
	_, err = connector.Execute(ctx, "append", map[string]interface{}{
		"path":    "$out/test.txt",
		"content": strings.Repeat("b", 30),
	})
	if err != nil {
		t.Fatalf("Append within quota failed: %v", err)
	}

	// Try to append beyond quota
	_, err = connector.Execute(ctx, "append", map[string]interface{}{
		"path":    "$out/test.txt",
		"content": strings.Repeat("c", 50),
	})
	if err == nil {
		t.Error("Expected error when appending beyond quota")
	}
}
