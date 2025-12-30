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

// Package postgres provides a PostgreSQL backend implementation for distributed deployments.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/controller/backend"
)

// Compile-time interface assertions.
// Ensures Backend implements all segregated interfaces.
var (
	_ backend.RunStore        = (*Backend)(nil)
	_ backend.RunLister       = (*Backend)(nil)
	_ backend.CheckpointStore = (*Backend)(nil)
	_ backend.StepResultStore = (*Backend)(nil)
	_ backend.Backend         = (*Backend)(nil)
	_ backend.ScheduleBackend = (*Backend)(nil)
)

// Backend is a PostgreSQL storage backend.
type Backend struct {
	db *sql.DB
}

// Config contains PostgreSQL connection configuration.
type Config struct {
	// ConnectionString is the PostgreSQL connection URL.
	// Format: postgres://user:password@host:port/database?sslmode=disable
	ConnectionString string

	// MaxOpenConns sets the maximum number of open connections.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections.
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum lifetime of a connection.
	ConnMaxLifetime time.Duration
}

// New creates a new PostgreSQL backend.
func New(cfg Config) (*Backend, error) {
	db, err := sql.Open("pgx", cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	b := &Backend{db: db}

	// Run migrations
	if err := b.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return b, nil
}

// migrate runs database migrations.
func (b *Backend) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS runs (
			id VARCHAR(36) PRIMARY KEY,
			workflow_id VARCHAR(255) NOT NULL,
			workflow VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			inputs JSONB,
			output JSONB,
			error TEXT,
			current_step VARCHAR(255),
			completed INTEGER DEFAULT 0,
			total INTEGER DEFAULT 0,
			started_at TIMESTAMPTZ,
			completed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_workflow ON runs(workflow)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			run_id VARCHAR(36) PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
			step_id VARCHAR(255) NOT NULL,
			step_index INTEGER NOT NULL,
			context JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS job_queue (
			id SERIAL PRIMARY KEY,
			run_id VARCHAR(36) NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			priority INTEGER DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			locked_by VARCHAR(255),
			locked_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(run_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_job_queue_status ON job_queue(status, priority DESC)`,
		`CREATE TABLE IF NOT EXISTS schedule_states (
			name VARCHAR(255) PRIMARY KEY,
			last_run TIMESTAMPTZ,
			next_run TIMESTAMPTZ,
			run_count BIGINT DEFAULT 0,
			error_count BIGINT DEFAULT 0,
			enabled BOOLEAN DEFAULT true,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS step_results (
			run_id VARCHAR(36) NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			step_id VARCHAR(255) NOT NULL,
			step_index INTEGER NOT NULL,
			inputs JSONB,
			outputs JSONB,
			duration BIGINT NOT NULL,
			status VARCHAR(50) NOT NULL,
			error TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (run_id, step_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_step_results_run_id ON step_results(run_id)`,
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

	query := `
		INSERT INTO runs (id, workflow_id, workflow, status, inputs, output, error,
			current_step, completed, total, started_at, completed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	now := time.Now()
	_, err = b.db.ExecContext(ctx, query,
		run.ID, run.WorkflowID, run.Workflow, run.Status,
		inputsJSON, outputJSON, run.Error,
		run.CurrentStep, run.Completed, run.Total,
		run.StartedAt, run.CompletedAt, now, now,
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
		SELECT id, workflow_id, workflow, status, inputs, output, error,
			current_step, completed, total, started_at, completed_at, created_at, updated_at
		FROM runs WHERE id = $1
	`

	var run backend.Run
	var inputsJSON, outputJSON []byte

	err := b.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID, &run.WorkflowID, &run.Workflow, &run.Status,
		&inputsJSON, &outputJSON, &run.Error,
		&run.CurrentStep, &run.Completed, &run.Total,
		&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	if len(inputsJSON) > 0 {
		json.Unmarshal(inputsJSON, &run.Inputs)
	}
	if len(outputJSON) > 0 {
		json.Unmarshal(outputJSON, &run.Output)
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

	query := `
		UPDATE runs SET
			workflow_id = $2, workflow = $3, status = $4, inputs = $5, output = $6,
			error = $7, current_step = $8, completed = $9, total = $10,
			started_at = $11, completed_at = $12, updated_at = $13
		WHERE id = $1
	`

	now := time.Now()
	result, err := b.db.ExecContext(ctx, query,
		run.ID, run.WorkflowID, run.Workflow, run.Status,
		inputsJSON, outputJSON, run.Error,
		run.CurrentStep, run.Completed, run.Total,
		run.StartedAt, run.CompletedAt, now,
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
		SELECT id, workflow_id, workflow, status, inputs, output, error,
			current_step, completed, total, started_at, completed_at, created_at, updated_at
		FROM runs WHERE 1=1
	`
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.Workflow != "" {
		query += fmt.Sprintf(" AND workflow = $%d", argIdx)
		args = append(args, filter.Workflow)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
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
		var inputsJSON, outputJSON []byte

		err := rows.Scan(
			&run.ID, &run.WorkflowID, &run.Workflow, &run.Status,
			&inputsJSON, &outputJSON, &run.Error,
			&run.CurrentStep, &run.Completed, &run.Total,
			&run.StartedAt, &run.CompletedAt, &run.CreatedAt, &run.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		if len(inputsJSON) > 0 {
			json.Unmarshal(inputsJSON, &run.Inputs)
		}
		if len(outputJSON) > 0 {
			json.Unmarshal(outputJSON, &run.Output)
		}

		runs = append(runs, &run)
	}

	return runs, nil
}

// DeleteRun deletes a run.
func (b *Backend) DeleteRun(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, "DELETE FROM runs WHERE id = $1", id)
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
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (run_id) DO UPDATE SET
			step_id = EXCLUDED.step_id,
			step_index = EXCLUDED.step_index,
			context = EXCLUDED.context,
			created_at = EXCLUDED.created_at
	`

	now := time.Now()
	_, err = b.db.ExecContext(ctx, query, runID, checkpoint.StepID, checkpoint.StepIndex, contextJSON, now)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	checkpoint.RunID = runID
	checkpoint.CreatedAt = now
	return nil
}

// GetCheckpoint retrieves a checkpoint.
func (b *Backend) GetCheckpoint(ctx context.Context, runID string) (*backend.Checkpoint, error) {
	query := `SELECT run_id, step_id, step_index, context, created_at FROM checkpoints WHERE run_id = $1`

	var checkpoint backend.Checkpoint
	var contextJSON []byte

	err := b.db.QueryRowContext(ctx, query, runID).Scan(
		&checkpoint.RunID, &checkpoint.StepID, &checkpoint.StepIndex, &contextJSON, &checkpoint.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("checkpoint not found for run: %s", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	if len(contextJSON) > 0 {
		json.Unmarshal(contextJSON, &checkpoint.Context)
	}

	return &checkpoint, nil
}

// Close closes the database connection.
func (b *Backend) Close() error {
	return b.db.Close()
}

// DB returns the underlying database connection.
// This is used for leader election and other distributed operations.
func (b *Backend) DB() *sql.DB {
	return b.db
}

// --- Distributed Job Queue Operations ---

// EnqueueJob adds a job to the queue.
func (b *Backend) EnqueueJob(ctx context.Context, runID string, priority int) error {
	query := `
		INSERT INTO job_queue (run_id, priority, status, created_at)
		VALUES ($1, $2, 'pending', NOW())
		ON CONFLICT (run_id) DO NOTHING
	`
	_, err := b.db.ExecContext(ctx, query, runID, priority)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}
	return nil
}

// DequeueJob claims and returns the next available job using row locking.
// This implements "SELECT FOR UPDATE SKIP LOCKED" for distributed job claiming.
func (b *Backend) DequeueJob(ctx context.Context, workerID string) (string, error) {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Select and lock the next available job
	query := `
		SELECT run_id FROM job_queue
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var runID string
	err = tx.QueryRowContext(ctx, query).Scan(&runID)
	if err == sql.ErrNoRows {
		return "", nil // No jobs available
	}
	if err != nil {
		return "", fmt.Errorf("failed to dequeue job: %w", err)
	}

	// Mark the job as running
	updateQuery := `
		UPDATE job_queue SET status = 'running', locked_by = $1, locked_at = NOW()
		WHERE run_id = $2
	`
	_, err = tx.ExecContext(ctx, updateQuery, workerID, runID)
	if err != nil {
		return "", fmt.Errorf("failed to lock job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return runID, nil
}

// CompleteJob marks a job as completed.
func (b *Backend) CompleteJob(ctx context.Context, runID string) error {
	_, err := b.db.ExecContext(ctx, "DELETE FROM job_queue WHERE run_id = $1", runID)
	if err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}
	return nil
}

// FailJob marks a job as failed and returns it to the queue.
func (b *Backend) FailJob(ctx context.Context, runID string) error {
	query := `
		UPDATE job_queue SET status = 'pending', locked_by = NULL, locked_at = NULL
		WHERE run_id = $1
	`
	_, err := b.db.ExecContext(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("failed to fail job: %w", err)
	}
	return nil
}

// RecoverStalledJobs recovers jobs that have been locked for too long.
func (b *Backend) RecoverStalledJobs(ctx context.Context, timeout time.Duration) (int64, error) {
	query := `
		UPDATE job_queue SET status = 'pending', locked_by = NULL, locked_at = NULL
		WHERE status = 'running' AND locked_at < $1
	`
	result, err := b.db.ExecContext(ctx, query, time.Now().Add(-timeout))
	if err != nil {
		return 0, fmt.Errorf("failed to recover stalled jobs: %w", err)
	}
	return result.RowsAffected()
}

// --- Schedule State Operations ---

// SaveScheduleState saves or updates a schedule state.
func (b *Backend) SaveScheduleState(ctx context.Context, state *backend.ScheduleState) error {
	query := `
		INSERT INTO schedule_states (name, last_run, next_run, run_count, error_count, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (name) DO UPDATE SET
			last_run = EXCLUDED.last_run,
			next_run = EXCLUDED.next_run,
			run_count = EXCLUDED.run_count,
			error_count = EXCLUDED.error_count,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := b.db.ExecContext(ctx, query,
		state.Name, state.LastRun, state.NextRun,
		state.RunCount, state.ErrorCount, state.Enabled, now,
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
		FROM schedule_states WHERE name = $1
	`

	var state backend.ScheduleState
	err := b.db.QueryRowContext(ctx, query, name).Scan(
		&state.Name, &state.LastRun, &state.NextRun,
		&state.RunCount, &state.ErrorCount, &state.Enabled, &state.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("schedule state not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule state: %w", err)
	}

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
		err := rows.Scan(
			&state.Name, &state.LastRun, &state.NextRun,
			&state.RunCount, &state.ErrorCount, &state.Enabled, &state.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule state: %w", err)
		}
		states = append(states, &state)
	}

	return states, nil
}

// DeleteScheduleState deletes a schedule state.
func (b *Backend) DeleteScheduleState(ctx context.Context, name string) error {
	_, err := b.db.ExecContext(ctx, "DELETE FROM schedule_states WHERE name = $1", name)
	if err != nil {
		return fmt.Errorf("failed to delete schedule state: %w", err)
	}
	return nil
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
		INSERT INTO step_results (run_id, step_id, step_index, inputs, outputs, duration, status, error, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (run_id, step_id) DO UPDATE SET
			step_index = EXCLUDED.step_index,
			inputs = EXCLUDED.inputs,
			outputs = EXCLUDED.outputs,
			duration = EXCLUDED.duration,
			status = EXCLUDED.status,
			error = EXCLUDED.error,
			created_at = EXCLUDED.created_at
	`

	now := time.Now()
	_, err = b.db.ExecContext(ctx, query,
		result.RunID, result.StepID, result.StepIndex,
		inputsJSON, outputsJSON, result.Duration.Nanoseconds(),
		result.Status, result.Error, now,
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
		SELECT run_id, step_id, step_index, inputs, outputs, duration, status, error, created_at
		FROM step_results
		WHERE run_id = $1 AND step_id = $2
	`

	var result backend.StepResult
	var inputsJSON, outputsJSON []byte
	var durationNanos int64

	err := b.db.QueryRowContext(ctx, query, runID, stepID).Scan(
		&result.RunID, &result.StepID, &result.StepIndex,
		&inputsJSON, &outputsJSON, &durationNanos,
		&result.Status, &result.Error, &result.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("step result not found: %s (run: %s)", stepID, runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get step result: %w", err)
	}

	if len(inputsJSON) > 0 {
		if err := json.Unmarshal(inputsJSON, &result.Inputs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
		}
	}

	if len(outputsJSON) > 0 {
		if err := json.Unmarshal(outputsJSON, &result.Outputs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal outputs: %w", err)
		}
	}

	result.Duration = time.Duration(durationNanos)

	return &result, nil
}

// ListStepResults retrieves all step results for a run.
func (b *Backend) ListStepResults(ctx context.Context, runID string) ([]*backend.StepResult, error) {
	query := `
		SELECT run_id, step_id, step_index, inputs, outputs, duration, status, error, created_at
		FROM step_results
		WHERE run_id = $1
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
		var inputsJSON, outputsJSON []byte
		var durationNanos int64

		err := rows.Scan(
			&result.RunID, &result.StepID, &result.StepIndex,
			&inputsJSON, &outputsJSON, &durationNanos,
			&result.Status, &result.Error, &result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step result: %w", err)
		}

		if len(inputsJSON) > 0 {
			if err := json.Unmarshal(inputsJSON, &result.Inputs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
			}
		}

		if len(outputsJSON) > 0 {
			if err := json.Unmarshal(outputsJSON, &result.Outputs); err != nil {
				return nil, fmt.Errorf("failed to unmarshal outputs: %w", err)
			}
		}

		result.Duration = time.Duration(durationNanos)

		results = append(results, &result)
	}

	return results, nil
}
