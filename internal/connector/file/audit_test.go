package file

import (
	"bytes"
	"log/slog"
	"testing"
	"time"
)

func TestSlogAuditLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewSlogAuditLogger(logger)

	entry := AuditEntry{
		Timestamp:    time.Now(),
		Operation:    "write_text",
		Path:         "/tmp/test.txt",
		Result:       "success",
		Duration:     100 * time.Millisecond,
		BytesWritten: 1024,
		WorkflowID:   "wf-123",
		StepID:       "step-456",
	}

	auditLogger.Log(entry)

	output := buf.String()
	if output == "" {
		t.Error("Expected audit log output, got empty string")
	}

	// Check that the log contains key fields
	expectedFields := []string{
		"write_text",
		"/tmp/test.txt",
		"success",
		"1024",
		"wf-123",
		"step-456",
	}

	for _, field := range expectedFields {
		if !bytes.Contains(buf.Bytes(), []byte(field)) {
			t.Errorf("Expected log to contain %q, got: %s", field, output)
		}
	}
}

func TestSlogAuditLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	auditLogger := NewSlogAuditLogger(logger)

	entry := AuditEntry{
		Timestamp: time.Now(),
		Operation: "read_text",
		Path:      "/tmp/missing.txt",
		Result:    "error",
		Duration:  10 * time.Millisecond,
		Error:     "file not found",
	}

	auditLogger.Log(entry)

	output := buf.String()
	if !bytes.Contains(buf.Bytes(), []byte("error")) {
		t.Errorf("Expected error in log, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("file not found")) {
		t.Errorf("Expected error message in log, got: %s", output)
	}
}

func TestSlogAuditLogger_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	auditLogger := NewSlogAuditLogger(nil)

	entry := AuditEntry{
		Timestamp: time.Now(),
		Operation: "write_text",
		Path:      "/tmp/test.txt",
		Result:    "success",
	}

	// Should not panic
	auditLogger.Log(entry)
}

func TestNoopAuditLogger(t *testing.T) {
	// Should not panic
	logger := &NoopAuditLogger{}

	entry := AuditEntry{
		Timestamp: time.Now(),
		Operation: "write_text",
		Path:      "/tmp/test.txt",
		Result:    "success",
	}

	// Should not panic or do anything
	logger.Log(entry)
}
