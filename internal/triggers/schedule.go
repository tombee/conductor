package triggers

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/config"
)

// AddSchedule adds a new schedule trigger to the configuration.
func (m *Manager) AddSchedule(ctx context.Context, req CreateScheduleRequest) error {
	// Validate inputs
	if err := ValidateWorkflowExists(m.workflowsDir, req.Workflow); err != nil {
		return err
	}

	if req.Name == "" {
		return fmt.Errorf("schedule name cannot be empty")
	}

	// Determine cron expression and timezone
	var cronExpr, timezone string
	var err error

	if req.Cron != "" && (req.Every != "" || req.At != "") {
		return fmt.Errorf("cannot use both --cron and --every/--at")
	}

	if req.Cron != "" {
		cronExpr = req.Cron
		timezone = req.Timezone
		if timezone == "" {
			timezone = "UTC"
		}
	} else {
		cronExpr, timezone, err = ParseEverySchedule(req.Every, req.At, req.Timezone)
		if err != nil {
			return err
		}
	}

	// Validate cron expression
	if err := ValidateCron(cronExpr); err != nil {
		return err
	}

	// Validate timezone
	if err := ValidateTimezone(timezone); err != nil {
		return err
	}

	// Load config with lock
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Enable schedules if not already configured
	cfg.Controller.Schedules.Enabled = true

	// Initialize schedules section if needed
	if cfg.Controller.Schedules.Schedules == nil {
		cfg.Controller.Schedules.Schedules = []config.ScheduleEntry{}
	}

	// Check for duplicate name
	for _, schedule := range cfg.Controller.Schedules.Schedules {
		if schedule.Name == req.Name {
			lock.Release()
			return fmt.Errorf("schedule name already exists: %s", req.Name)
		}
	}

	// Add new schedule
	newSchedule := config.ScheduleEntry{
		Name:     req.Name,
		Cron:     cronExpr,
		Workflow: req.Workflow,
		Inputs:   req.Inputs,
		Enabled:  true,
		Timezone: timezone,
	}
	cfg.Controller.Schedules.Schedules = append(cfg.Controller.Schedules.Schedules, newSchedule)

	// Save config
	return m.saveConfig(cfg, lock)
}

// ListSchedules returns all configured schedule triggers.
func (m *Manager) ListSchedules(ctx context.Context) ([]ScheduleTrigger, error) {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	var triggers []ScheduleTrigger
	for _, schedule := range cfg.Controller.Schedules.Schedules {
		triggers = append(triggers, ScheduleTrigger{
			Name:     schedule.Name,
			Cron:     schedule.Cron,
			Workflow: schedule.Workflow,
			Inputs:   schedule.Inputs,
			Enabled:  schedule.Enabled,
			Timezone: schedule.Timezone,
		})
	}

	return triggers, nil
}

// RemoveSchedule removes a schedule trigger by name.
func (m *Manager) RemoveSchedule(ctx context.Context, name string) error {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Find and remove schedule
	found := false
	newSchedules := make([]config.ScheduleEntry, 0, len(cfg.Controller.Schedules.Schedules))
	for _, schedule := range cfg.Controller.Schedules.Schedules {
		if schedule.Name == name {
			found = true
			continue
		}
		newSchedules = append(newSchedules, schedule)
	}

	if !found {
		lock.Release()
		return fmt.Errorf("schedule not found: %s", name)
	}

	cfg.Controller.Schedules.Schedules = newSchedules

	return m.saveConfig(cfg, lock)
}
