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

// Package storage provides trace and event storage implementations.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/tombee/conductor/pkg/observability"
)

// SQLiteStore provides SQLite-backed storage for traces and events.
type SQLiteStore struct {
	db            *sql.DB
	encryptionKey *EncryptionKey
}

// Config contains SQLite storage configuration.
type Config struct {
	// Path is the filesystem path to the SQLite database file.
	// Special value ":memory:" creates an in-memory database.
	Path string

	// MaxOpenConns sets the maximum number of open connections.
	// For SQLite, this should typically be 1 to avoid lock contention.
	MaxOpenConns int

	// EnableEncryption enables AES-256-GCM encryption for stored data.
	// Requires CONDUCTOR_TRACE_KEY environment variable to be set.
	EnableEncryption bool
}

// New creates a new SQLite storage backend.
func New(cfg Config) (*SQLiteStore, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	// SQLite connection string with WAL mode for better concurrency
	connStr := cfg.Path
	if cfg.Path != ":memory:" {
		connStr += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	// With WAL mode, SQLite can handle multiple readers concurrently
	maxConns := cfg.MaxOpenConns
	if maxConns == 0 {
		maxConns = 5 // Allow multiple concurrent reads
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(2)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	store := &SQLiteStore{db: db}

	// Load encryption key if enabled
	if cfg.EnableEncryption {
		key, err := LoadEncryptionKey()
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to load encryption key: %w", err)
		}
		if key == nil {
			db.Close()
			return nil, fmt.Errorf("encryption enabled but no key found (set CONDUCTOR_TRACE_KEY)")
		}
		store.encryptionKey = key
	}

	// Run migrations
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// migrate creates the database schema.
func (s *SQLiteStore) migrate(ctx context.Context) error {
	// Enable foreign keys (disabled by default in SQLite)
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	migrations := []string{
		// Spans table stores individual span data
		`CREATE TABLE IF NOT EXISTS spans (
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			parent_id TEXT,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			status_code INTEGER NOT NULL,
			status_message TEXT,
			attributes TEXT,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (trace_id, span_id)
		)`,
		// Index for finding spans by trace
		`CREATE INDEX IF NOT EXISTS idx_spans_trace_id ON spans(trace_id)`,
		// Index for finding root spans (no parent)
		`CREATE INDEX IF NOT EXISTS idx_spans_parent_id ON spans(parent_id) WHERE parent_id IS NOT NULL`,
		// Index for time-based queries
		`CREATE INDEX IF NOT EXISTS idx_spans_start_time ON spans(start_time)`,
		// Index for active spans (not yet ended)
		`CREATE INDEX IF NOT EXISTS idx_spans_active ON spans(end_time) WHERE end_time IS NULL`,

		// Events table stores span events
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trace_id TEXT NOT NULL,
			span_id TEXT NOT NULL,
			name TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			attributes TEXT,
			created_at INTEGER NOT NULL
		)`,
		// Index for finding events by span
		`CREATE INDEX IF NOT EXISTS idx_events_span ON events(trace_id, span_id)`,
		// Index for finding events by name
		`CREATE INDEX IF NOT EXISTS idx_events_name ON events(name)`,
		// Index for time-based event queries
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp)`,

		// Traces table provides a summary view of complete traces
		`CREATE TABLE IF NOT EXISTS traces (
			trace_id TEXT PRIMARY KEY,
			root_span_id TEXT,
			name TEXT,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			duration_ns INTEGER,
			status_code INTEGER,
			span_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			attributes TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		// Index for finding traces by status
		`CREATE INDEX IF NOT EXISTS idx_traces_status ON traces(status_code)`,
		// Index for time-based trace queries
		`CREATE INDEX IF NOT EXISTS idx_traces_start_time ON traces(start_time)`,
	}

	for _, migration := range migrations {
		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// StoreSpan stores a span in the database.
func (s *SQLiteStore) StoreSpan(ctx context.Context, span *observability.Span) error {
	if span == nil {
		return fmt.Errorf("span is nil")
	}
	if span.TraceID == "" {
		return fmt.Errorf("span trace_id is required")
	}
	if span.SpanID == "" {
		return fmt.Errorf("span span_id is required")
	}

	// Serialize attributes
	attributesJSON, err := json.Marshal(span.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Encrypt attributes if encryption is enabled
	attributesJSON, err = s.encryptData(attributesJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt attributes: %w", err)
	}

	// Convert times to Unix nanoseconds
	startTime := span.StartTime.UnixNano()
	var endTime *int64
	if !span.EndTime.IsZero() {
		et := span.EndTime.UnixNano()
		endTime = &et
	}

	// Store span
	query := `
		INSERT INTO spans (trace_id, span_id, parent_id, name, kind, start_time, end_time,
			status_code, status_message, attributes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(trace_id, span_id) DO UPDATE SET
			parent_id = excluded.parent_id,
			name = excluded.name,
			kind = excluded.kind,
			end_time = excluded.end_time,
			status_code = excluded.status_code,
			status_message = excluded.status_message,
			attributes = excluded.attributes
	`

	now := time.Now().UnixNano()
	var parentID *string
	if span.ParentID != "" {
		parentID = &span.ParentID
	}

	_, err = s.db.ExecContext(ctx, query,
		span.TraceID, span.SpanID, parentID, span.Name, span.Kind,
		startTime, endTime, span.Status.Code, span.Status.Message,
		attributesJSON, now,
	)
	if err != nil {
		return fmt.Errorf("failed to store span: %w", err)
	}

	// Store events
	for _, event := range span.Events {
		if err := s.storeEvent(ctx, span.TraceID, span.SpanID, &event); err != nil {
			return fmt.Errorf("failed to store event: %w", err)
		}
	}

	// Update trace summary
	if err := s.updateTraceSummary(ctx, span.TraceID); err != nil {
		return fmt.Errorf("failed to update trace summary: %w", err)
	}

	return nil
}

// storeEvent stores a span event.
func (s *SQLiteStore) storeEvent(ctx context.Context, traceID, spanID string, event *observability.Event) error {
	attributesJSON, err := json.Marshal(event.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Encrypt attributes if encryption is enabled
	attributesJSON, err = s.encryptData(attributesJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt event attributes: %w", err)
	}

	query := `
		INSERT INTO events (trace_id, span_id, name, timestamp, attributes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	timestamp := event.Timestamp.UnixNano()
	now := time.Now().UnixNano()

	_, err = s.db.ExecContext(ctx, query, traceID, spanID, event.Name, timestamp, attributesJSON, now)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	return nil
}

// updateTraceSummary updates the trace summary table.
func (s *SQLiteStore) updateTraceSummary(ctx context.Context, traceID string) error {
	// Calculate trace statistics from spans
	query := `
		INSERT INTO traces (trace_id, root_span_id, name, start_time, end_time, duration_ns,
			status_code, span_count, error_count, created_at, updated_at)
		SELECT
			?,
			(SELECT span_id FROM spans WHERE trace_id = ? AND parent_id IS NULL LIMIT 1),
			(SELECT name FROM spans WHERE trace_id = ? AND parent_id IS NULL LIMIT 1),
			MIN(start_time),
			MAX(end_time),
			CASE WHEN MAX(end_time) IS NOT NULL THEN MAX(end_time) - MIN(start_time) ELSE NULL END,
			(SELECT status_code FROM spans WHERE trace_id = ? AND parent_id IS NULL LIMIT 1),
			COUNT(*),
			SUM(CASE WHEN status_code = 2 THEN 1 ELSE 0 END),
			?,
			?
		FROM spans WHERE trace_id = ?
		ON CONFLICT(trace_id) DO UPDATE SET
			root_span_id = excluded.root_span_id,
			name = excluded.name,
			end_time = excluded.end_time,
			duration_ns = excluded.duration_ns,
			status_code = excluded.status_code,
			span_count = excluded.span_count,
			error_count = excluded.error_count,
			updated_at = excluded.updated_at
	`

	now := time.Now().UnixNano()
	_, err := s.db.ExecContext(ctx, query, traceID, traceID, traceID, traceID, now, now, traceID)
	if err != nil {
		return fmt.Errorf("failed to update trace summary: %w", err)
	}

	return nil
}

// GetSpan retrieves a span by trace ID and span ID.
func (s *SQLiteStore) GetSpan(ctx context.Context, traceID, spanID string) (*observability.Span, error) {
	query := `
		SELECT trace_id, span_id, parent_id, name, kind, start_time, end_time,
			status_code, status_message, attributes
		FROM spans WHERE trace_id = ? AND span_id = ?
	`

	var span observability.Span
	var parentID *string
	var endTime *int64
	var startTime int64
	var attributesJSON []byte

	err := s.db.QueryRowContext(ctx, query, traceID, spanID).Scan(
		&span.TraceID, &span.SpanID, &parentID, &span.Name, &span.Kind,
		&startTime, &endTime, &span.Status.Code, &span.Status.Message,
		&attributesJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("span not found: %s/%s", traceID, spanID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get span: %w", err)
	}

	if parentID != nil {
		span.ParentID = *parentID
	}

	span.StartTime = time.Unix(0, startTime)
	if endTime != nil {
		span.EndTime = time.Unix(0, *endTime)
	}

	if len(attributesJSON) > 0 {
		// Decrypt attributes if encryption is enabled
		decrypted, err := s.decryptData(attributesJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt attributes: %w", err)
		}
		if err := json.Unmarshal(decrypted, &span.Attributes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
	}

	// Load events
	events, err := s.getSpanEvents(ctx, traceID, spanID)
	if err != nil {
		return nil, fmt.Errorf("failed to get span events: %w", err)
	}
	span.Events = events

	return &span, nil
}

// getSpanEvents retrieves all events for a span.
func (s *SQLiteStore) getSpanEvents(ctx context.Context, traceID, spanID string) ([]observability.Event, error) {
	query := `
		SELECT name, timestamp, attributes
		FROM events WHERE trace_id = ? AND span_id = ?
		ORDER BY timestamp ASC
	`

	rows, err := s.db.QueryContext(ctx, query, traceID, spanID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []observability.Event
	for rows.Next() {
		var event observability.Event
		var timestamp int64
		var attributesJSON []byte

		if err := rows.Scan(&event.Name, &timestamp, &attributesJSON); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.Timestamp = time.Unix(0, timestamp)

		if len(attributesJSON) > 0 {
			// Decrypt attributes if encryption is enabled
			decrypted, err := s.decryptData(attributesJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt event attributes: %w", err)
			}
			if err := json.Unmarshal(decrypted, &event.Attributes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event attributes: %w", err)
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// TraceFilter contains filters for trace queries.
type TraceFilter struct {
	// Status filters by status code
	Status *observability.StatusCode

	// Since filters traces that started after this time
	Since *time.Time

	// Until filters traces that started before this time
	Until *time.Time

	// Limit limits the number of results
	Limit int

	// Offset skips the first N results
	Offset int
}

// ListTraces lists traces matching the filter.
func (s *SQLiteStore) ListTraces(ctx context.Context, filter TraceFilter) ([]string, error) {
	query := "SELECT trace_id FROM traces WHERE 1=1"
	args := []any{}

	if filter.Status != nil {
		query += " AND status_code = ?"
		args = append(args, *filter.Status)
	}

	if filter.Since != nil {
		query += " AND start_time >= ?"
		args = append(args, filter.Since.UnixNano())
	}

	if filter.Until != nil {
		query += " AND start_time <= ?"
		args = append(args, filter.Until.UnixNano())
	}

	query += " ORDER BY start_time DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list traces: %w", err)
	}
	defer rows.Close()

	var traceIDs []string
	for rows.Next() {
		var traceID string
		if err := rows.Scan(&traceID); err != nil {
			return nil, fmt.Errorf("failed to scan trace ID: %w", err)
		}
		traceIDs = append(traceIDs, traceID)
	}

	return traceIDs, nil
}

// GetTraceByRunID retrieves a trace ID by run ID from span attributes.
// Returns empty string if no trace is found with the given run ID.
// Note: This performs a full scan of spans since encrypted attributes cannot be indexed.
// For better performance with large datasets, consider storing run_id in a separate indexed column.
func (s *SQLiteStore) GetTraceByRunID(ctx context.Context, runID string) (string, error) {
	// Query recent trace IDs (most likely to match) ordered by start time descending
	query := `
		SELECT DISTINCT s.trace_id
		FROM spans s
		JOIN traces t ON s.trace_id = t.trace_id
		ORDER BY t.start_time DESC
		LIMIT 1000
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to query traces: %w", err)
	}

	// Collect trace IDs before closing rows to avoid connection deadlock
	var traceIDs []string
	for rows.Next() {
		var traceID string
		if err := rows.Scan(&traceID); err != nil {
			continue
		}
		traceIDs = append(traceIDs, traceID)
	}
	rows.Close()

	// Check each trace for matching run_id
	for _, traceID := range traceIDs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Get spans for this trace
		spans, err := s.GetTraceSpans(ctx, traceID)
		if err != nil {
			continue
		}

		// Check if any span has the matching run_id
		for _, span := range spans {
			if runIDAttr, ok := span.Attributes["run_id"].(string); ok && runIDAttr == runID {
				return traceID, nil
			}
		}
	}

	return "", nil
}

// GetTraceSpans retrieves all spans for a trace.
func (s *SQLiteStore) GetTraceSpans(ctx context.Context, traceID string) ([]*observability.Span, error) {
	query := `
		SELECT span_id, parent_id, name, kind, start_time, end_time,
			status_code, status_message, attributes
		FROM spans WHERE trace_id = ?
		ORDER BY start_time ASC
	`

	rows, err := s.db.QueryContext(ctx, query, traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query spans: %w", err)
	}
	defer rows.Close()

	var spans []*observability.Span
	var spanIDs []string

	for rows.Next() {
		span := &observability.Span{TraceID: traceID}
		var parentID *string
		var endTime *int64
		var startTime int64
		var attributesJSON []byte

		err := rows.Scan(
			&span.SpanID, &parentID, &span.Name, &span.Kind,
			&startTime, &endTime, &span.Status.Code, &span.Status.Message,
			&attributesJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan span: %w", err)
		}

		if parentID != nil {
			span.ParentID = *parentID
		}

		span.StartTime = time.Unix(0, startTime)
		if endTime != nil {
			span.EndTime = time.Unix(0, *endTime)
		}

		if len(attributesJSON) > 0 {
			// Decrypt attributes if encryption is enabled
			decrypted, err := s.decryptData(attributesJSON)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt attributes: %w", err)
			}
			if err := json.Unmarshal(decrypted, &span.Attributes); err != nil {
				return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
			}
		}

		spanIDs = append(spanIDs, span.SpanID)
		spans = append(spans, span)
	}

	// Close rows before loading events to free the connection
	rows.Close()

	// Now load events for all spans (after rows are closed)
	for i, span := range spans {
		events, err := s.getSpanEvents(ctx, traceID, spanIDs[i])
		if err != nil {
			return nil, fmt.Errorf("failed to get span events: %w", err)
		}
		span.Events = events
	}

	return spans, nil
}

// DeleteTracesOlderThan deletes traces that started before the given time.
// Returns the number of traces deleted.
func (s *SQLiteStore) DeleteTracesOlderThan(ctx context.Context, before time.Time) (int64, error) {
	// First delete from traces table
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM traces WHERE start_time < ?",
		before.UnixNano(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old traces: %w", err)
	}

	count, _ := result.RowsAffected()

	// Delete orphaned spans (cascade should handle this, but be explicit)
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM spans WHERE trace_id NOT IN (SELECT trace_id FROM traces)
	`)
	if err != nil {
		return count, fmt.Errorf("failed to delete orphaned spans: %w", err)
	}

	// Delete orphaned events (cascade should handle this, but be explicit)
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM events WHERE trace_id NOT IN (SELECT trace_id FROM traces)
	`)
	if err != nil {
		return count, fmt.Errorf("failed to delete orphaned events: %w", err)
	}

	return count, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection.
// This is exported for testing and advanced use cases.
func (s *SQLiteStore) DB() *sql.DB {
	return s.db
}

// encryptData encrypts data if encryption is enabled.
func (s *SQLiteStore) encryptData(data []byte) ([]byte, error) {
	if s.encryptionKey == nil {
		return data, nil
	}

	encrypted, err := s.encryptionKey.Encrypt(data)
	if err != nil {
		return nil, err
	}
	return []byte(encrypted), nil
}

// decryptData decrypts data if encryption is enabled.
func (s *SQLiteStore) decryptData(data []byte) ([]byte, error) {
	if s.encryptionKey == nil {
		return data, nil
	}

	if len(data) == 0 {
		return data, nil
	}

	return s.encryptionKey.Decrypt(string(data))
}
