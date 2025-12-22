package cost

import (
	"context"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// CostStore defines the interface for persistent cost record storage.
type CostStore interface {
	// Store saves a cost record.
	Store(ctx context.Context, record llm.CostRecord) error

	// GetByID retrieves a cost record by its ID.
	GetByID(ctx context.Context, id string) (*llm.CostRecord, error)

	// GetByRequestID retrieves a cost record by request ID.
	GetByRequestID(ctx context.Context, requestID string) (*llm.CostRecord, error)

	// GetByRunID retrieves all cost records for a specific run.
	GetByRunID(ctx context.Context, runID string) ([]llm.CostRecord, error)

	// GetByWorkflowID retrieves all cost records for a specific workflow.
	GetByWorkflowID(ctx context.Context, workflowID string) ([]llm.CostRecord, error)

	// GetByUserID retrieves all cost records for a specific user.
	GetByUserID(ctx context.Context, userID string) ([]llm.CostRecord, error)

	// GetByProvider retrieves all cost records for a specific provider.
	GetByProvider(ctx context.Context, provider string) ([]llm.CostRecord, error)

	// GetByModel retrieves all cost records for a specific model.
	GetByModel(ctx context.Context, model string) ([]llm.CostRecord, error)

	// GetByTimeRange retrieves cost records within a time range.
	GetByTimeRange(ctx context.Context, start, end time.Time) ([]llm.CostRecord, error)

	// Aggregate computes aggregated cost statistics.
	Aggregate(ctx context.Context, opts AggregateOptions) (*llm.CostAggregate, error)

	// AggregateByProvider returns aggregates grouped by provider.
	AggregateByProvider(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error)

	// AggregateByModel returns aggregates grouped by model.
	AggregateByModel(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error)

	// AggregateByWorkflow returns aggregates grouped by workflow.
	AggregateByWorkflow(ctx context.Context, opts AggregateOptions) (map[string]llm.CostAggregate, error)

	// DeleteOlderThan removes records older than the specified duration.
	// Used for retention policy enforcement.
	DeleteOlderThan(ctx context.Context, age time.Duration) (int64, error)

	// Close closes the store and releases resources.
	Close() error
}

// AggregateOptions specifies filtering options for aggregation queries.
type AggregateOptions struct {
	// StartTime filters records after this time (inclusive).
	StartTime *time.Time

	// EndTime filters records before this time (exclusive).
	EndTime *time.Time

	// Provider filters records for a specific provider.
	Provider string

	// Model filters records for a specific model.
	Model string

	// WorkflowID filters records for a specific workflow.
	WorkflowID string

	// UserID filters records for a specific user.
	UserID string

	// RunID filters records for a specific run.
	RunID string
}

// AuditLogEntry records access to cost data for compliance.
type AuditLogEntry struct {
	// ID is the unique audit entry identifier.
	ID string

	// Timestamp is when the access occurred.
	Timestamp time.Time

	// UserID is who accessed the data.
	UserID string

	// Action describes what was accessed (e.g., "view_costs", "export_csv").
	Action string

	// Resource identifies what was accessed (e.g., "workflow:abc123", "provider:anthropic").
	Resource string

	// IPAddress is the source IP address.
	IPAddress string

	// UserAgent is the client user agent.
	UserAgent string

	// Success indicates if the access was authorized and successful.
	Success bool

	// ErrorMessage contains error details if Success is false.
	ErrorMessage string
}

// AuditStore defines the interface for audit log storage.
type AuditStore interface {
	// Log records an audit entry.
	Log(ctx context.Context, entry AuditLogEntry) error

	// GetByUser retrieves audit entries for a specific user.
	GetByUser(ctx context.Context, userID string, limit int) ([]AuditLogEntry, error)

	// GetByTimeRange retrieves audit entries within a time range.
	GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]AuditLogEntry, error)

	// GetRecent retrieves the most recent audit entries.
	GetRecent(ctx context.Context, limit int) ([]AuditLogEntry, error)

	// Close closes the audit store.
	Close() error
}
