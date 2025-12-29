package file

import (
	"log/slog"
	"time"
)

// AuditEntry represents a logged file operation
type AuditEntry struct {
	Timestamp    time.Time
	Operation    string
	Path         string
	Result       string // "success" or "error"
	Duration     time.Duration
	BytesRead    int64
	BytesWritten int64
	Error        string // if Result == "error"
	WorkflowID   string
	StepID       string
}

// AuditLogger logs file operations
type AuditLogger interface {
	Log(entry AuditEntry)
}

// SlogAuditLogger implements AuditLogger using structured logging with slog
type SlogAuditLogger struct {
	logger *slog.Logger
}

// NewSlogAuditLogger creates an audit logger that uses slog
func NewSlogAuditLogger(logger *slog.Logger) *SlogAuditLogger {
	return &SlogAuditLogger{
		logger: logger,
	}
}

// Log writes an audit entry using structured logging
func (l *SlogAuditLogger) Log(entry AuditEntry) {
	if l.logger == nil {
		return
	}

	attrs := []slog.Attr{
		slog.String("operation", entry.Operation),
		slog.String("path", entry.Path),
		slog.String("result", entry.Result),
		slog.Duration("duration", entry.Duration),
	}

	// Add optional fields only if they're set
	if entry.BytesRead > 0 {
		attrs = append(attrs, slog.Int64("bytes_read", entry.BytesRead))
	}
	if entry.BytesWritten > 0 {
		attrs = append(attrs, slog.Int64("bytes_written", entry.BytesWritten))
	}
	if entry.Error != "" {
		attrs = append(attrs, slog.String("error", entry.Error))
	}
	if entry.WorkflowID != "" {
		attrs = append(attrs, slog.String("workflow_id", entry.WorkflowID))
	}
	if entry.StepID != "" {
		attrs = append(attrs, slog.String("step_id", entry.StepID))
	}

	// Log at appropriate level based on result
	if entry.Result == "error" {
		l.logger.LogAttrs(nil, slog.LevelError, "file operation failed", attrs...)
	} else {
		l.logger.LogAttrs(nil, slog.LevelInfo, "file operation completed", attrs...)
	}
}

// NoopAuditLogger is a no-op implementation for when auditing is disabled
type NoopAuditLogger struct{}

// Log does nothing
func (n *NoopAuditLogger) Log(entry AuditEntry) {}
