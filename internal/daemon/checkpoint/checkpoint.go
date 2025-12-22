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

// Package checkpoint provides workflow checkpoint management for crash recovery.
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Checkpoint represents a saved point in workflow execution.
type Checkpoint struct {
	RunID       string         `json:"run_id"`
	WorkflowID  string         `json:"workflow_id"`
	StepID      string         `json:"step_id"`
	StepIndex   int            `json:"step_index"`
	Context     map[string]any `json:"context"`      // Workflow execution context
	StepOutputs map[string]any `json:"step_outputs"` // Results from completed steps
	CreatedAt   time.Time      `json:"created_at"`
}

// Manager handles checkpoint storage and retrieval.
type Manager struct {
	mu       sync.RWMutex
	dir      string
	enabled  bool
	interval time.Duration
}

// ManagerConfig contains checkpoint manager configuration.
type ManagerConfig struct {
	// Dir is the directory to store checkpoint files.
	// If empty, checkpointing is disabled.
	Dir string

	// Interval is how often to save checkpoints during execution.
	// Default is after each step.
	Interval time.Duration
}

// NewManager creates a new checkpoint manager.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	m := &Manager{
		dir:      cfg.Dir,
		enabled:  cfg.Dir != "",
		interval: cfg.Interval,
	}

	if m.enabled {
		if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
		}
	}

	return m, nil
}

// Save saves a checkpoint for a run.
func (m *Manager) Save(ctx context.Context, cp *Checkpoint) error {
	if !m.enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cp.CreatedAt = time.Now()

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	path := m.checkpointPath(cp.RunID)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return nil
}

// Load loads a checkpoint for a run.
func (m *Manager) Load(ctx context.Context, runID string) (*Checkpoint, error) {
	if !m.enabled {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	path := m.checkpointPath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &cp, nil
}

// Delete removes a checkpoint for a run.
func (m *Manager) Delete(ctx context.Context, runID string) error {
	if !m.enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.checkpointPath(runID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	return nil
}

// ListInterrupted returns a list of run IDs that have checkpoints
// (indicating interrupted runs that may need to be resumed).
func (m *Manager) ListInterrupted(ctx context.Context) ([]string, error) {
	if !m.enabled {
		return nil, nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var runIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			runIDs = append(runIDs, name[:len(name)-5])
		}
	}

	return runIDs, nil
}

// Enabled returns whether checkpointing is enabled.
func (m *Manager) Enabled() bool {
	return m.enabled
}

// checkpointPath returns the file path for a run's checkpoint.
func (m *Manager) checkpointPath(runID string) string {
	return filepath.Join(m.dir, runID+".json")
}
