package polltrigger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// StateManager handles persistence of poll trigger state using SQLite.
type StateManager struct {
	db *sql.DB
}

// StateConfig contains configuration for the state manager.
type StateConfig struct {
	// Path is the filesystem path to the SQLite database file.
	// Default: ~/.local/share/conductor/poll-state.db
	Path string

	// MaxOpenConns sets the maximum number of open connections.
	// For SQLite, this should typically be low to avoid lock contention.
	MaxOpenConns int
}

// NewStateManager creates a new state manager with SQLite backend.
func NewStateManager(cfg StateConfig) (*StateManager, error) {
	if cfg.Path == "" {
		// Default to user's local data directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		cfg.Path = filepath.Join(homeDir, ".local", "share", "conductor", "poll-state.db")
	}

	// Create parent directory if it doesn't exist
	if cfg.Path != ":memory:" {
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// SQLite connection string with WAL mode for better concurrency and durability
	connStr := cfg.Path
	if cfg.Path != ":memory:" {
		connStr += "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	}

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	maxConns := cfg.MaxOpenConns
	if maxConns == 0 {
		maxConns = 5
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

	sm := &StateManager{db: db}

	// Run migrations
	if err := sm.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return sm, nil
}

// migrate creates the database schema.
func (sm *StateManager) migrate(ctx context.Context) error {
	// Enable foreign keys
	if _, err := sm.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create poll_state table
	schema := `
	CREATE TABLE IF NOT EXISTS poll_state (
		trigger_id TEXT PRIMARY KEY,
		workflow_path TEXT NOT NULL,
		integration TEXT NOT NULL,
		last_poll_time DATETIME,
		high_water_mark DATETIME,
		seen_events TEXT,
		cursor TEXT,
		last_error TEXT,
		error_count INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_poll_state_workflow ON poll_state(workflow_path);
	CREATE INDEX IF NOT EXISTS idx_poll_state_integration ON poll_state(integration);
	`

	if _, err := sm.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// GetState retrieves the poll state for a given trigger ID.
// Returns nil if no state exists yet.
func (sm *StateManager) GetState(ctx context.Context, triggerID string) (*PollState, error) {
	query := `
	SELECT trigger_id, workflow_path, integration, last_poll_time, high_water_mark,
	       seen_events, cursor, last_error, error_count, created_at, updated_at
	FROM poll_state
	WHERE trigger_id = ?
	`

	row := sm.db.QueryRowContext(ctx, query, triggerID)

	var state PollState
	var seenEventsJSON sql.NullString
	var lastPollTime, highWaterMark sql.NullTime
	var cursor, lastError sql.NullString

	err := row.Scan(
		&state.TriggerID,
		&state.WorkflowPath,
		&state.Integration,
		&lastPollTime,
		&highWaterMark,
		&seenEventsJSON,
		&cursor,
		&lastError,
		&state.ErrorCount,
		&state.CreatedAt,
		&state.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	// Parse optional fields
	if lastPollTime.Valid {
		state.LastPollTime = lastPollTime.Time
	}
	if highWaterMark.Valid {
		state.HighWaterMark = highWaterMark.Time
	}
	if cursor.Valid {
		state.Cursor = cursor.String
	}
	if lastError.Valid {
		state.LastError = lastError.String
	}

	// Parse seen_events JSON
	if seenEventsJSON.Valid && seenEventsJSON.String != "" {
		if err := json.Unmarshal([]byte(seenEventsJSON.String), &state.SeenEvents); err != nil {
			return nil, fmt.Errorf("failed to parse seen_events: %w", err)
		}
	}
	if state.SeenEvents == nil {
		state.SeenEvents = make(map[string]int64)
	}

	return &state, nil
}

// SaveState creates or updates the poll state for a trigger.
func (sm *StateManager) SaveState(ctx context.Context, state *PollState) error {
	// Serialize seen_events
	seenEventsJSON, err := json.Marshal(state.SeenEvents)
	if err != nil {
		return fmt.Errorf("failed to marshal seen_events: %w", err)
	}

	// Set updated_at to now
	state.UpdatedAt = time.Now()

	query := `
	INSERT INTO poll_state (
		trigger_id, workflow_path, integration, last_poll_time, high_water_mark,
		seen_events, cursor, last_error, error_count, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(trigger_id) DO UPDATE SET
		workflow_path = excluded.workflow_path,
		integration = excluded.integration,
		last_poll_time = excluded.last_poll_time,
		high_water_mark = excluded.high_water_mark,
		seen_events = excluded.seen_events,
		cursor = excluded.cursor,
		last_error = excluded.last_error,
		error_count = excluded.error_count,
		updated_at = excluded.updated_at
	`

	_, err = sm.db.ExecContext(ctx, query,
		state.TriggerID,
		state.WorkflowPath,
		state.Integration,
		state.LastPollTime,
		state.HighWaterMark,
		string(seenEventsJSON),
		state.Cursor,
		state.LastError,
		state.ErrorCount,
		state.CreatedAt,
		state.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}

// DeleteState removes the poll state for a trigger.
func (sm *StateManager) DeleteState(ctx context.Context, triggerID string) error {
	query := `DELETE FROM poll_state WHERE trigger_id = ?`

	_, err := sm.db.ExecContext(ctx, query, triggerID)
	if err != nil {
		return fmt.Errorf("failed to delete state: %w", err)
	}

	return nil
}

// PruneSeenEvents removes old entries from the seen_events map to prevent unbounded growth.
// Removes events older than TTL (default 24h) and enforces a maximum count (default 10,000).
func (sm *StateManager) PruneSeenEvents(state *PollState, ttlSeconds int64, maxCount int) {
	if ttlSeconds == 0 {
		ttlSeconds = 86400 // 24 hours default
	}
	if maxCount == 0 {
		maxCount = 10000 // 10k events default
	}

	now := time.Now().Unix()
	cutoff := now - ttlSeconds

	// Remove events older than TTL
	for eventID, timestamp := range state.SeenEvents {
		if timestamp < cutoff {
			delete(state.SeenEvents, eventID)
		}
	}

	// If still over max count, remove oldest events (FIFO eviction)
	if len(state.SeenEvents) > maxCount {
		// Build a sorted list of (timestamp, eventID) pairs
		type entry struct {
			eventID   string
			timestamp int64
		}
		entries := make([]entry, 0, len(state.SeenEvents))
		for eventID, timestamp := range state.SeenEvents {
			entries = append(entries, entry{eventID, timestamp})
		}

		// Sort by timestamp (oldest first)
		// Using a simple bubble sort for small datasets
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[i].timestamp > entries[j].timestamp {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		// Remove oldest until we're at maxCount
		toRemove := len(entries) - maxCount
		for i := 0; i < toRemove; i++ {
			delete(state.SeenEvents, entries[i].eventID)
		}
	}
}

// Close closes the database connection.
func (sm *StateManager) Close() error {
	if sm.db != nil {
		return sm.db.Close()
	}
	return nil
}
