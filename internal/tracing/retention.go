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

package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tombee/conductor/internal/tracing/storage"
)

// RetentionManager handles automatic cleanup of old traces.
type RetentionManager struct {
	store           *storage.SQLiteStore
	maxAge          time.Duration
	cleanupInterval time.Duration
	logger          *slog.Logger
	stopCh          chan struct{}
	doneCh          chan struct{}
}

// NewRetentionManager creates a new retention manager.
// maxAge is how long to keep traces before deletion.
// cleanupInterval is how often to run the cleanup job.
func NewRetentionManager(store *storage.SQLiteStore, maxAge, cleanupInterval time.Duration, logger *slog.Logger) *RetentionManager {
	if maxAge == 0 {
		maxAge = 7 * 24 * time.Hour // Default: 7 days
	}
	if cleanupInterval == 0 {
		cleanupInterval = 1 * time.Hour // Default: Run every hour
	}

	return &RetentionManager{
		store:           store,
		maxAge:          maxAge,
		cleanupInterval: cleanupInterval,
		logger:          logger,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
}

// Start begins the retention cleanup loop.
// This runs in a background goroutine and returns immediately.
func (r *RetentionManager) Start() {
	go r.run()
}

// Stop gracefully stops the retention manager.
// It waits for any in-progress cleanup to complete.
func (r *RetentionManager) Stop() {
	close(r.stopCh)
	<-r.doneCh
}

// run is the main retention loop.
func (r *RetentionManager) run() {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	// Run cleanup immediately on start
	r.cleanup()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCh:
			r.logger.Info("retention manager stopping")
			return
		}
	}
}

// cleanup performs a single cleanup pass.
func (r *RetentionManager) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	before := time.Now().Add(-r.maxAge)

	r.logger.Debug("starting trace retention cleanup",
		"before", before.Format(time.RFC3339),
		"max_age", r.maxAge,
	)

	deleted, err := r.store.DeleteTracesOlderThan(ctx, before)
	if err != nil {
		r.logger.Error("failed to clean up old traces", "error", err)
		return
	}

	if deleted > 0 {
		r.logger.Info("cleaned up old traces",
			"count", deleted,
			"before", before.Format(time.RFC3339),
		)
	} else {
		r.logger.Debug("no old traces to clean up")
	}
}

// CleanupNow forces an immediate cleanup pass.
// This blocks until cleanup completes.
func (r *RetentionManager) CleanupNow(ctx context.Context) error {
	before := time.Now().Add(-r.maxAge)

	r.logger.Info("manual trace cleanup triggered",
		"before", before.Format(time.RFC3339),
		"max_age", r.maxAge,
	)

	deleted, err := r.store.DeleteTracesOlderThan(ctx, before)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	r.logger.Info("manual trace cleanup complete",
		"count", deleted,
	)

	return nil
}
