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

// Package scheduler provides cron-based workflow scheduling.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/daemon/runner"
	"github.com/tombee/conductor/internal/log"
)

// Schedule defines a scheduled workflow execution.
type Schedule struct {
	// Name is the unique identifier for this schedule
	Name string `yaml:"name" json:"name"`

	// Cron is the cron expression (standard 5-field format)
	// Format: minute hour day-of-month month day-of-week
	Cron string `yaml:"cron" json:"cron"`

	// Workflow is the workflow file to run
	Workflow string `yaml:"workflow" json:"workflow"`

	// Inputs are the inputs to pass to the workflow
	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`

	// Enabled indicates if the schedule is active
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Timezone for cron evaluation (defaults to UTC)
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`

	// computed fields
	cronExpr   *CronExpr
	nextRun    time.Time
	lastRun    *time.Time
	runCount   int64
	errorCount int64
}

// Config contains scheduler configuration.
type Config struct {
	// Schedules defines the scheduled workflows
	Schedules []Schedule `yaml:"schedules" json:"schedules"`

	// WorkflowsDir is where to find workflow files
	WorkflowsDir string `yaml:"workflows_dir" json:"workflows_dir"`
}

// Scheduler manages scheduled workflow execution.
type Scheduler struct {
	mu           sync.RWMutex
	schedules    map[string]*Schedule
	runner       *runner.Runner
	workflowsDir string
	stopCh       chan struct{}
	doneCh       chan struct{}
	running      bool
	logger       *slog.Logger
}

// New creates a new scheduler.
func New(cfg Config, r *runner.Runner) (*Scheduler, error) {
	logger := slog.Default().With(slog.String("component", "scheduler"))

	s := &Scheduler{
		schedules:    make(map[string]*Schedule),
		runner:       r,
		workflowsDir: cfg.WorkflowsDir,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		logger:       logger,
	}

	// Parse and add schedules
	for _, sched := range cfg.Schedules {
		if err := s.AddSchedule(sched); err != nil {
			return nil, fmt.Errorf("invalid schedule %s: %w", sched.Name, err)
		}
	}

	return s, nil
}

// AddSchedule adds a new schedule.
func (s *Scheduler) AddSchedule(sched Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse cron expression
	expr, err := ParseCron(sched.Cron)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	sched.cronExpr = expr

	// Calculate next run time
	loc := time.UTC
	if sched.Timezone != "" {
		loc, err = time.LoadLocation(sched.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
	}
	sched.nextRun = expr.Next(time.Now().In(loc))

	s.schedules[sched.Name] = &sched
	return nil
}

// RemoveSchedule removes a schedule.
func (s *Scheduler) RemoveSchedule(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, name)
}

// GetSchedule returns a schedule by name.
func (s *Scheduler) GetSchedule(name string) (*Schedule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sched, ok := s.schedules[name]
	return sched, ok
}

// ListSchedules returns all schedules.
func (s *Scheduler) ListSchedules() []Schedule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Schedule, 0, len(s.schedules))
	for _, sched := range s.schedules {
		result = append(result, *sched)
	}
	return result
}

// SetEnabled enables or disables a schedule.
func (s *Scheduler) SetEnabled(name string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sched, ok := s.schedules[name]
	if !ok {
		return fmt.Errorf("schedule not found: %s", name)
	}
	sched.Enabled = enabled
	return nil
}

// Start starts the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	go s.run(ctx)
}

// Stop stops the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	<-s.doneCh
}

// run is the main scheduler loop.
func (s *Scheduler) run(ctx context.Context) {
	defer close(s.doneCh)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.tick(ctx, now)
		}
	}
}

// tick checks for due schedules and triggers them.
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sched := range s.schedules {
		if !sched.Enabled {
			continue
		}

		if now.After(sched.nextRun) || now.Equal(sched.nextRun) {
			// Time to run this schedule
			go s.triggerSchedule(ctx, sched)

			// Calculate next run time
			loc := time.UTC
			if sched.Timezone != "" {
				if l, err := time.LoadLocation(sched.Timezone); err == nil {
					loc = l
				}
			}
			sched.nextRun = sched.cronExpr.Next(now.In(loc))
			sched.lastRun = &now
			sched.runCount++
		}
	}
}

// triggerSchedule triggers a scheduled workflow.
func (s *Scheduler) triggerSchedule(ctx context.Context, sched *Schedule) {
	schedLogger := s.logger.With(slog.String("schedule", sched.Name), slog.String(log.WorkflowKey, sched.Workflow))

	// Check if runner is draining (graceful shutdown in progress)
	if s.runner.IsDraining() {
		schedLogger.Info("Skipping scheduled execution during graceful shutdown")
		return
	}

	schedLogger.Info("Triggering scheduled workflow")

	// Find workflow file
	workflowPath, err := s.findWorkflow(sched.Workflow)
	if err != nil {
		schedLogger.Error("Workflow not found", slog.Any("error", err))
		s.mu.Lock()
		sched.errorCount++
		s.mu.Unlock()
		return
	}

	// Read workflow file
	workflowYAML, err := os.ReadFile(workflowPath)
	if err != nil {
		schedLogger.Error("Failed to read workflow", slog.Any("error", err))
		s.mu.Lock()
		sched.errorCount++
		s.mu.Unlock()
		return
	}

	// Add schedule metadata to inputs
	inputs := make(map[string]any)
	for k, v := range sched.Inputs {
		inputs[k] = v
	}
	inputs["_scheduled"] = true
	inputs["_schedule_name"] = sched.Name

	// Submit workflow
	run, err := s.runner.Submit(ctx, runner.SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       inputs,
	})
	if err != nil {
		schedLogger.Error("Failed to submit workflow", slog.Any("error", err))
		s.mu.Lock()
		sched.errorCount++
		s.mu.Unlock()
		return
	}

	schedLogger.Info("Started workflow run", slog.String(log.RunIDKey, run.ID))
}

// findWorkflow finds a workflow file.
func (s *Scheduler) findWorkflow(name string) (string, error) {
	extensions := []string{".yaml", ".yml", ""}
	baseDirs := []string{s.workflowsDir, "."}

	for _, baseDir := range baseDirs {
		if baseDir == "" {
			continue
		}
		for _, ext := range extensions {
			path := filepath.Join(baseDir, name+ext)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("workflow not found: %s", name)
}

// ScheduleStatus contains status information for a schedule.
type ScheduleStatus struct {
	Name       string     `json:"name"`
	Cron       string     `json:"cron"`
	Workflow   string     `json:"workflow"`
	Enabled    bool       `json:"enabled"`
	NextRun    time.Time  `json:"next_run"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	RunCount   int64      `json:"run_count"`
	ErrorCount int64      `json:"error_count"`
}

// GetStatus returns the status of all schedules.
func (s *Scheduler) GetStatus() []ScheduleStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ScheduleStatus, 0, len(s.schedules))
	for _, sched := range s.schedules {
		result = append(result, ScheduleStatus{
			Name:       sched.Name,
			Cron:       sched.Cron,
			Workflow:   sched.Workflow,
			Enabled:    sched.Enabled,
			NextRun:    sched.nextRun,
			LastRun:    sched.lastRun,
			RunCount:   sched.runCount,
			ErrorCount: sched.errorCount,
		})
	}
	return result
}

// GetScheduleCount returns the total number of schedules.
func (s *Scheduler) GetScheduleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.schedules)
}

// GetEnabledScheduleCount returns the number of enabled schedules.
func (s *Scheduler) GetEnabledScheduleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, sched := range s.schedules {
		if sched.Enabled {
			count++
		}
	}
	return count
}
