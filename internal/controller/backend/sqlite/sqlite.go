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

// Package sqlite provides a SQLite backend implementation for single-node deployments.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
	_ "modernc.org/sqlite"
)

// Compile-time interface assertions.
var (
	_ backend.RunStore        = (*Backend)(nil)
	_ backend.RunLister       = (*Backend)(nil)
	_ backend.CheckpointStore = (*Backend)(nil)
	_ backend.StepResultStore = (*Backend)(nil)
	_ backend.Backend         = (*Backend)(nil)
	_ backend.ScheduleBackend = (*Backend)(nil)
)

// Backend is a SQLite storage backend.
type Backend struct {
	db *sql.DB
}

// Config contains SQLite connection configuration.
type Config struct {
	// Path is the database file path.
	Path string

	// WAL enables Write-Ahead Logging mode for concurrent reads.
	WAL bool
}

// New creates a new SQLite backend.
func New(cfg Config) (*Backend, error) {
	// Open database connection
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite
	// SQLite serializes writes, so only 1 connection for writes
	db.SetMaxOpenConns(1)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	b := &Backend{db: db}

	// Configure SQLite pragmas
	if err := b.configurePragmas(ctx, cfg.WAL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure pragmas: %w", err)
	}

	// Run migrations
	if err := b.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return b, nil
}

// configurePragmas sets SQLite configuration options.
func (b *Backend) configurePragmas(ctx context.Context, enableWAL bool) error {
	pragmas := []string{
		"PRAGMA foreign_keys=ON",           // Enable foreign key constraints
		"PRAGMA busy_timeout=5000",         // 5 second timeout for lock contention
		"PRAGMA auto_vacuum=INCREMENTAL",   // Incremental auto-vacuum for space reclamation
		"PRAGMA synchronous=NORMAL",        // Balance between performance and durability
	}

	if enableWAL {
		pragmas = append(pragmas, "PRAGMA journal_mode=WAL") // Enable WAL mode for concurrent reads
	}

	for _, pragma := range pragmas {
		if _, err := b.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	return nil
}

// migrate runs database migrations.
func (b *Backend) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			workflow TEXT NOT NULL,
			status TEXT NOT NULL,
			correlation_id TEXT,
			inputs TEXT,
			output TEXT,
			error TEXT,
			current_step TEXT,
			completed INTEGER DEFAULT 0,
			total INTEGER DEFAULT 0,
			parent_run_id TEXT,
			replay_config TEXT,
			started_at TEXT,
			completed_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_workflow ON runs(workflow)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_parent_run_id ON runs(parent_run_id)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			run_id TEXT PRIMARY KEY,
			step_id TEXT NOT NULL,
			step_index INTEGER NOT NULL,
			context TEXT,
			created_at TEXT NOT NULL,
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS step_results (
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			step_index INTEGER NOT NULL,
			inputs TEXT,
			outputs TEXT,
			duration INTEGER NOT NULL,
			status TEXT NOT NULL,
			error TEXT,
			cost_usd REAL DEFAULT 0,
			created_at TEXT NOT NULL,
			PRIMARY KEY (run_id, step_id),
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_step_results_run_id ON step_results(run_id)`,
		`CREATE TABLE IF NOT EXISTS schedule_states (
			name TEXT PRIMARY KEY,
			last_run TEXT,
			next_run TEXT,
			run_count INTEGER DEFAULT 0,
			error_count INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,
	}

	for _, migration := range migrations {
		if _, err := b.db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// CreateRun creates a new run.
func (b *Backend) CreateRun(ctx context.Context, run *backend.Run) error {
	inputsJSON, err := json.Marshal(run.Inputs)
	if err != nil {
		return fmt.Errorf("failed to marshal inputs: %w", err)
	}

	outputJSON, err := json.Marshal(run.Output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	var replayConfigJSON []byte
	if run.ReplayConfig != nil {
		replayConfigJSON, err = json.Marshal(run.ReplayConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal replay_config: %w", err)
		}
	}

	query := `
		INSERT INTO runs (id, workflow_id, workflow, status, correlation_id, inputs, output, error,
			current_step, completed, total, parent_run_id, replay_config, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	startedAt := formatTime(run.StartedAt)
	completedAt := formatTime(run.CompletedAt)

	_, err = b.db.ExecContext(ctx, query,
		run.ID, run.WorkflowID, run.Workflow, run.Status, nullString(run.CorrelationID),
		string(inputsJSON), string(outputJSON), nullString(run.Error),
		nullString(run.CurrentStep), run.Completed, run.Total,
		nullString(run.ParentRunID), nullBytes(replayConfigJSON),
		startedAt, completedAt, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	run.CreatedAt = now
	run.UpdatedAt = now
	return nil
}

// GetRun retrieves a run by ID.
func (b *Backend) GetRun(ctx context.Context, id string) (*backend.Run, error) {
	query := `
		SELECT id, workflow_id, workflow, status, correlation_id, inputs, output, error,
			current_step, completed, total, parent_run_id, replay_config,
			started_at, completed_at, created_at, updated_at
		FROM runs WHERE id = ?
	`

	var run backend.Run
	var inputsJSON, outputJSON, replayConfigJSON sql.NullString
	var correlationID, currentStep, parentRunID, errorStr sql.NullString
	var startedAt, completedAt, createdAt, updatedAt sql.NullString

	err := b.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.WorkflowID, &run.Workflow, &run.Status, &correlationID,
		&inputsJSON, &outputJSON, &errorStr,
		&currentStep, &run.Completed, &run.Total,
		&parentRunID, &replayConfigJSON,
		&startedAt, &completedAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	// Parse nullable strings
	if correlationID.Valid {
		run.CorrelationID = correlationID.String
	}
	if currentStep.Valid {
		run.CurrentStep = currentStep.String
	}
	if parentRunID.Valid {
		run.ParentRunID = parentRunID.String
	}
	if errorStr.Valid {
		run.Error = errorStr.String
	}

	// Parse JSON fields
	if inputsJSON.Valid && inputsJSON.String != "" {
		if err := json.Unmarshal([]byte(inputsJSON.String), &run.Inputs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
		}
	}
	if outputJSON.Valid && outputJSON.String != "" {
		if err := json.Unmarshal([]byte(outputJSON.String), &run.Output); err != nil {
			return nil, fmt.Errorf("failed to unmarshal output: %w", err)
		}
	}
	if replayConfigJSON.Valid && replayConfigJSON.String != "" {
		var replayConfig backend.ReplayConfig
		if err := json.Unmarshal([]byte(replayConfigJSON.String), &replayConfig); err == nil {
			run.ReplayConfig = &replayConfig
		}
	}

	// Parse timestamps
	if startedAt.Valid {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		run.StartedAt = &t
	}
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		run.CompletedAt = &t
	}
	if createdAt.Valid {
		run.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		run.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}

	return &run, nil
}

// UpdateRun updates an existing run.
func (b *Backend) UpdateRun(ctx context.Context, run *backend.Run) error {
	inputsJSON, err := json.Marshal(run.Inputs)
	if err != nil {
		return fmt.Errorf("failed to marshal inputs: %w", err)
	}

	outputJSON, err := json.Marshal(run.Output)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	var replayConfigJSON []byte
	if run.ReplayConfig != nil {
		replayConfigJSON, err = json.Marshal(run.ReplayConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal replay_config: %w", err)
		}
	}

	query := `
		UPDATE runs SET
			workflow_id = ?, workflow = ?, status = ?, correlation_id = ?,
			inputs = ?, output = ?, error = ?, current_step = ?,
			completed = ?, total = ?, parent_run_id = ?, replay_config = ?,
			started_at = ?, completed_at = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	startedAt := formatTime(run.StartedAt)
	completedAt := formatTime(run.CompletedAt)

	result, err := b.db.ExecContext(ctx, query,
		run.WorkflowID, run.Workflow, run.Status, nullString(run.CorrelationID),
		string(inputsJSON), string(outputJSON), nullString(run.Error), nullString(run.CurrentStep),
		run.Completed, run.Total, nullString(run.ParentRunID), nullBytes(replayConfigJSON),
		startedAt, completedAt, now.Format(time.RFC3339),
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("run not found: %s", run.ID)
	}

	run.UpdatedAt = now
	return nil
}

// ListRuns lists runs with optional filtering.
func (b *Backend) ListRuns(ctx context.Context, filter backend.RunFilter) ([]*backend.Run, error) {
	query := `
		SELECT id, workflow_id, workflow, status, correlation_id, inputs, output, error,
			current_step, completed, total, parent_run_id, replay_config,
			started_at, completed_at, created_at, updated_at
		FROM runs WHERE 1=1
	`
	args := []any{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Workflow != "" {
		query += " AND workflow = ?"
		args = append(args, filter.Workflow)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	var runs []*backend.Run
	for rows.Next() {
		var run backend.Run
		var inputsJSON, outputJSON, replayConfigJSON sql.NullString
		var correlationID, currentStep, parentRunID, errorStr sql.NullString
		var startedAt, completedAt, createdAt, updatedAt sql.NullString

		err := rows.Scan(
			&run.ID, &run.WorkflowID, &run.Workflow, &run.Status, &correlationID,
			&inputsJSON, &outputJSON, &errorStr,
			&currentStep, &run.Completed, &run.Total,
			&parentRunID, &replayConfigJSON,
			&startedAt, &completedAt, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		// Parse nullable strings
		if correlationID.Valid {
			run.CorrelationID = correlationID.String
		}
		if currentStep.Valid {
			run.CurrentStep = currentStep.String
		}
		if parentRunID.Valid {
			run.ParentRunID = parentRunID.String
		}
		if errorStr.Valid {
			run.Error = errorStr.String
		}

		// Parse JSON fields
		if inputsJSON.Valid && inputsJSON.String != "" {
			json.Unmarshal([]byte(inputsJSON.String), &run.Inputs)
		}
		if outputJSON.Valid && outputJSON.String != "" {
			json.Unmarshal([]byte(outputJSON.String), &run.Output)
		}
		if replayConfigJSON.Valid && replayConfigJSON.String != "" {
			var replayConfig backend.ReplayConfig
			if err := json.Unmarshal([]byte(replayConfigJSON.String), &replayConfig); err == nil {
				run.ReplayConfig = &replayConfig
			}
		}

		// Parse timestamps
		if startedAt.Valid {
			t, _ := time.Parse(time.RFC3339, startedAt.String)
			run.StartedAt = &t
		}
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			run.CompletedAt = &t
		}
		if createdAt.Valid {
			run.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}
		if updatedAt.Valid {
			run.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		}

		runs = append(runs, &run)
	}

	return runs, nil
}

// DeleteRun deletes a run.
func (b *Backend) DeleteRun(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, "DELETE FROM runs WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}
	return nil
}

// SaveCheckpoint saves a checkpoint.
func (b *Backend) SaveCheckpoint(ctx context.Context, runID string, checkpoint *backend.Checkpoint) error {
	contextJSON, err := json.Marshal(checkpoint.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	query := `
		INSERT INTO checkpoints (run_id, step_id, step_index, context, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (run_id) DO UPDATE SET
			step_id = excluded.step_id,
			step_index = excluded.step_index,
			context = excluded.context,
			created_at = excluded.created_at
	`

	now := time.Now()
	_, err = b.db.ExecContext(ctx, query, runID, checkpoint.StepID, checkpoint.StepIndex, string(contextJSON), now.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	checkpoint.RunID = runID
	checkpoint.CreatedAt = now
	return nil
}

// GetCheckpoint retrieves a checkpoint.
func (b *Backend) GetCheckpoint(ctx context.Context, runID string) (*backend.Checkpoint, error) {
	query := `SELECT run_id, step_id, step_index, context, created_at FROM checkpoints WHERE run_id = ?`

	var checkpoint backend.Checkpoint
	var contextJSON, createdAt sql.NullString

	err := b.db.QueryRowContext(ctx, query, runID).Scan(
		&checkpoint.RunID, &checkpoint.StepID, &checkpoint.StepIndex, &contextJSON, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("checkpoint not found for run: %s", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	if contextJSON.Valid && contextJSON.String != "" {
		if err := json.Unmarshal([]byte(contextJSON.String), &checkpoint.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}
	}

	if createdAt.Valid {
		checkpoint.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}

	return &checkpoint, nil
}

// SaveStepResult saves a step execution result.
func (b *Backend) SaveStepResult(ctx context.Context, result *backend.StepResult) error {
	inputsJSON, err := json.Marshal(result.Inputs)
	if err != nil {
		return fmt.Errorf("failed to marshal inputs: %w", err)
	}

	outputsJSON, err := json.Marshal(result.Outputs)
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	query := `
		INSERT INTO step_results (run_id, step_id, step_index, inputs, outputs, duration, status, error, cost_usd, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (run_id, step_id) DO UPDATE SET
			step_index = excluded.step_index,
			inputs = excluded.inputs,
			outputs = excluded.outputs,
			duration = excluded.duration,
			status = excluded.status,
			error = excluded.error,
			cost_usd = excluded.cost_usd,
			created_at = excluded.created_at
	`

	now := time.Now()
	_, err = b.db.ExecContext(ctx, query,
		result.RunID, result.StepID, result.StepIndex,
		string(inputsJSON), string(outputsJSON), result.Duration.Nanoseconds(),
		result.Status, nullString(result.Error), result.CostUSD, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to save step result: %w", err)
	}

	result.CreatedAt = now
	return nil
}

// GetStepResult retrieves a step result by run ID and step ID.
func (b *Backend) GetStepResult(ctx context.Context, runID, stepID string) (*backend.StepResult, error) {
	query := `
		SELECT run_id, step_id, step_index, inputs, outputs, duration, status, error, cost_usd, created_at
		FROM step_results
		WHERE run_id = ? AND step_id = ?
	`

	var result backend.StepResult
	var inputsJSON, outputsJSON sql.NullString
	var errorStr, createdAt sql.NullString
	var durationNanos int64

	err := b.db.QueryRowContext(ctx, query, runID, stepID).Scan(
		&result.RunID, &result.StepID, &result.StepIndex,
		&inputsJSON, &outputsJSON, &durationNanos,
		&result.Status, &errorStr, &result.CostUSD, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("step result not found: %s (run: %s)", stepID, runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step result: %w", err)
	}

	if inputsJSON.Valid && inputsJSON.String != "" {
		if err := json.Unmarshal([]byte(inputsJSON.String), &result.Inputs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
		}
	}

	if outputsJSON.Valid && outputsJSON.String != "" {
		if err := json.Unmarshal([]byte(outputsJSON.String), &result.Outputs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal outputs: %w", err)
		}
	}

	if errorStr.Valid {
		result.Error = errorStr.String
	}

	if createdAt.Valid {
		result.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}

	result.Duration = time.Duration(durationNanos)

	return &result, nil
}

// ListStepResults retrieves all step results for a run.
func (b *Backend) ListStepResults(ctx context.Context, runID string) ([]*backend.StepResult, error) {
	query := `
		SELECT run_id, step_id, step_index, inputs, outputs, duration, status, error, cost_usd, created_at
		FROM step_results
		WHERE run_id = ?
		ORDER BY step_index ASC
	`

	rows, err := b.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to list step results: %w", err)
	}
	defer rows.Close()

	var results []*backend.StepResult
	for rows.Next() {
		var result backend.StepResult
		var inputsJSON, outputsJSON sql.NullString
		var errorStr, createdAt sql.NullString
		var durationNanos int64

		err := rows.Scan(
			&result.RunID, &result.StepID, &result.StepIndex,
			&inputsJSON, &outputsJSON, &durationNanos,
			&result.Status, &errorStr, &result.CostUSD, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step result: %w", err)
		}

		if inputsJSON.Valid && inputsJSON.String != "" {
			if err := json.Unmarshal([]byte(inputsJSON.String), &result.Inputs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
			}
		}

		if outputsJSON.Valid && outputsJSON.String != "" {
			if err := json.Unmarshal([]byte(outputsJSON.String), &result.Outputs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal outputs: %w", err)
			}
		}

		if errorStr.Valid {
			result.Error = errorStr.String
		}

		if createdAt.Valid {
			result.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}

		result.Duration = time.Duration(durationNanos)

		results = append(results, &result)
	}

	return results, nil
}

// SaveScheduleState saves or updates a schedule state.
func (b *Backend) SaveScheduleState(ctx context.Context, state *backend.ScheduleState) error {
	query := `
		INSERT INTO schedule_states (name, last_run, next_run, run_count, error_count, enabled, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET
			last_run = excluded.last_run,
			next_run = excluded.next_run,
			run_count = excluded.run_count,
			error_count = excluded.error_count,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	lastRun := formatTime(state.LastRun)
	nextRun := formatTime(state.NextRun)
	enabled := 0
	if state.Enabled {
		enabled = 1
	}

	_, err := b.db.ExecContext(ctx, query,
		state.Name, lastRun, nextRun,
		state.RunCount, state.ErrorCount, enabled, now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to save schedule state: %w", err)
	}

	state.UpdatedAt = now
	return nil
}

// GetScheduleState retrieves a schedule state by name.
func (b *Backend) GetScheduleState(ctx context.Context, name string) (*backend.ScheduleState, error) {
	query := `
		SELECT name, last_run, next_run, run_count, error_count, enabled, updated_at
		FROM schedule_states WHERE name = ?
	`

	var state backend.ScheduleState
	var lastRun, nextRun, updatedAt sql.NullString
	var enabled int

	err := b.db.QueryRowContext(ctx, query, name).Scan(
		&state.Name, &lastRun, &nextRun,
		&state.RunCount, &state.ErrorCount, &enabled, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("schedule state not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule state: %w", err)
	}

	if lastRun.Valid {
		t, _ := time.Parse(time.RFC3339, lastRun.String)
		state.LastRun = &t
	}
	if nextRun.Valid {
		t, _ := time.Parse(time.RFC3339, nextRun.String)
		state.NextRun = &t
	}
	if updatedAt.Valid {
		state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}

	state.Enabled = enabled == 1

	return &state, nil
}

// ListScheduleStates returns all schedule states.
func (b *Backend) ListScheduleStates(ctx context.Context) ([]*backend.ScheduleState, error) {
	query := `
		SELECT name, last_run, next_run, run_count, error_count, enabled, updated_at
		FROM schedule_states ORDER BY name
	`

	rows, err := b.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedule states: %w", err)
	}
	defer rows.Close()

	var states []*backend.ScheduleState
	for rows.Next() {
		var state backend.ScheduleState
		var lastRun, nextRun, updatedAt sql.NullString
		var enabled int

		err := rows.Scan(
			&state.Name, &lastRun, &nextRun,
			&state.RunCount, &state.ErrorCount, &enabled, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule state: %w", err)
		}

		if lastRun.Valid {
			t, _ := time.Parse(time.RFC3339, lastRun.String)
			state.LastRun = &t
		}
		if nextRun.Valid {
			t, _ := time.Parse(time.RFC3339, nextRun.String)
			state.NextRun = &t
		}
		if updatedAt.Valid {
			state.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		}

		state.Enabled = enabled == 1

		states = append(states, &state)
	}

	return states, nil
}

// DeleteScheduleState deletes a schedule state.
func (b *Backend) DeleteScheduleState(ctx context.Context, name string) error {
	_, err := b.db.ExecContext(ctx, "DELETE FROM schedule_states WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("failed to delete schedule state: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (b *Backend) Close() error {
	return b.db.Close()
}

// Helper functions

// formatTime converts a *time.Time to RFC3339 string or nil.
func formatTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

// nullString returns nil if string is empty, otherwise the string.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullBytes returns nil if byte slice is empty, otherwise the string representation.
func nullBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
