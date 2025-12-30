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

package runner

import (
	"context"
	"log/slog"
	"time"
)

// StartCleanupLoop starts a background goroutine that periodically cleans up old completed runs.
// The loop runs every 60 minutes and removes runs older than the retention period.
// The loop respects context cancellation for graceful shutdown.
func (s *StateManager) StartCleanupLoop(ctx context.Context, retention time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("cleanup loop stopped", "reason", ctx.Err())
			return
		case <-ticker.C:
			deleted := s.CleanupCompletedRuns(retention)
			if deleted > 0 {
				logger.Info("cleaned up old runs", "deleted", deleted, "retention", retention)
			}
		}
	}
}
