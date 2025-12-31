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

// Package leader provides leader election for distributed controller deployments.
package leader

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

// AdvisoryLockID is the Postgres advisory lock ID for leader election.
// This should be unique across applications sharing the database.
// Using a hash of "conductor" that fits in int64.
const AdvisoryLockID int64 = 0x636F6E6475637464 // "conductd" in hex (truncated)

// Elector manages leader election using PostgreSQL advisory locks.
type Elector struct {
	db         *sql.DB
	instanceID string
	isLeader   bool
	mu         sync.RWMutex
	stopCh     chan struct{}
	doneCh     chan struct{}
	callbacks  []func(isLeader bool)
	logger     *slog.Logger
}

// Config contains leader election configuration.
type Config struct {
	// DB is the database connection.
	DB *sql.DB

	// InstanceID uniquely identifies this controller instance.
	InstanceID string

	// RetryInterval is how often to attempt acquiring leadership.
	RetryInterval time.Duration

	// Logger is the structured logger to use. If nil, uses slog.Default().
	Logger *slog.Logger
}

// NewElector creates a new leader elector.
func NewElector(cfg Config) *Elector {
	if cfg.RetryInterval <= 0 {
		cfg.RetryInterval = 5 * time.Second
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Elector{
		db:         cfg.DB,
		instanceID: cfg.InstanceID,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
		logger:     logger.With(slog.String("component", "leader"), slog.String("instance_id", cfg.InstanceID)),
	}
}

// Start begins the leader election process.
func (e *Elector) Start(ctx context.Context) {
	go e.run(ctx)
}

// Stop stops the leader election process.
func (e *Elector) Stop() {
	close(e.stopCh)
	<-e.doneCh
}

// IsLeader returns whether this instance is currently the leader.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isLeader
}

// OnLeadershipChange registers a callback for leadership changes.
func (e *Elector) OnLeadershipChange(callback func(isLeader bool)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.callbacks = append(e.callbacks, callback)
}

// run is the main leader election loop.
func (e *Elector) run(ctx context.Context) {
	defer close(e.doneCh)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Try to acquire leadership immediately
	e.tryAcquireLeadership(ctx)

	for {
		select {
		case <-ctx.Done():
			e.releaseLeadership(ctx)
			return
		case <-e.stopCh:
			e.releaseLeadership(ctx)
			return
		case <-ticker.C:
			if !e.IsLeader() {
				e.tryAcquireLeadership(ctx)
			} else {
				// Verify we still hold the lock
				if !e.verifyLeadership(ctx) {
					e.setLeader(false)
					e.logger.Warn("Lost leadership, will retry")
				}
			}
		}
	}
}

// tryAcquireLeadership attempts to acquire the leader lock.
func (e *Elector) tryAcquireLeadership(ctx context.Context) {
	// Try to acquire advisory lock (non-blocking)
	var acquired bool
	err := e.db.QueryRowContext(ctx,
		"SELECT pg_try_advisory_lock($1)", AdvisoryLockID,
	).Scan(&acquired)

	if err != nil {
		e.logger.Error("Failed to acquire leadership", slog.Any("error", err))
		return
	}

	if acquired {
		e.setLeader(true)
		e.logger.Info("Acquired leadership")
	}
}

// verifyLeadership verifies that we still hold the leader lock.
func (e *Elector) verifyLeadership(ctx context.Context) bool {
	// Check if we hold the advisory lock
	var holding bool
	err := e.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_locks
			WHERE locktype = 'advisory'
			AND classid = ($1 >> 32)::int
			AND objid = ($1 & 4294967295)::int
			AND pid = pg_backend_pid()
		)
	`, AdvisoryLockID).Scan(&holding)

	if err != nil {
		e.logger.Error("Failed to verify leadership", slog.Any("error", err))
		return false
	}

	return holding
}

// releaseLeadership releases the leader lock if held.
func (e *Elector) releaseLeadership(ctx context.Context) {
	if !e.IsLeader() {
		return
	}

	_, err := e.db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", AdvisoryLockID)
	if err != nil {
		e.logger.Error("Failed to release leadership", slog.Any("error", err))
	}

	e.setLeader(false)
	e.logger.Info("Released leadership")
}

// setLeader updates the leader status and notifies callbacks.
func (e *Elector) setLeader(isLeader bool) {
	e.mu.Lock()
	wasLeader := e.isLeader
	e.isLeader = isLeader
	callbacks := make([]func(bool), len(e.callbacks))
	copy(callbacks, e.callbacks)
	e.mu.Unlock()

	// Notify callbacks if status changed
	if wasLeader != isLeader {
		for _, cb := range callbacks {
			cb(isLeader)
		}
	}
}

// LeaderStatus contains information about leadership status.
type LeaderStatus struct {
	InstanceID string    `json:"instance_id"`
	IsLeader   bool      `json:"is_leader"`
	AcquiredAt time.Time `json:"acquired_at,omitempty"`
}

// Status returns the current leadership status.
func (e *Elector) Status() LeaderStatus {
	return LeaderStatus{
		InstanceID: e.instanceID,
		IsLeader:   e.IsLeader(),
	}
}
