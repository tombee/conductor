package polltrigger

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
	"go.opentelemetry.io/otel/metric"
)

// Service manages poll triggers for the controller.
// It coordinates the scheduler, state manager, and integration pollers to
// execute poll triggers and fire workflows for new events.
type Service struct {
	cfg           ServiceConfig
	logger        *slog.Logger
	scheduler     *Scheduler
	stateManager  *StateManager
	pollers       map[string]IntegrationPoller
	registrations map[string]*PollTriggerRegistration
	workflowFirer WorkflowFirer
	rateLimiter   *RateLimiter
	metrics       *MetricsCollector
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	started       bool
}

// ServiceConfig contains configuration for the poll trigger service.
type ServiceConfig struct {
	// StateDBPath is the path to the SQLite state database.
	// If empty, defaults to ~/.local/share/conductor/poll-state.db
	StateDBPath string

	// Logger is the structured logger for the service.
	Logger *slog.Logger

	// WorkflowFirer is the callback to fire workflows when triggers match.
	WorkflowFirer WorkflowFirer

	// PollTimeout is the maximum duration for a single poll execution.
	// Default: 10 seconds
	PollTimeout time.Duration

	// MeterProvider is the OpenTelemetry meter provider for metrics.
	// If nil, metrics will not be collected.
	MeterProvider metric.MeterProvider
}

// WorkflowFirer is called when a poll trigger matches an event.
// It should execute the workflow with the given trigger context.
type WorkflowFirer func(ctx context.Context, workflowPath string, triggerContext *PollTriggerContext) error

// NewService creates a new poll trigger service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if cfg.WorkflowFirer == nil {
		return nil, fmt.Errorf("workflow firer is required")
	}
	if cfg.PollTimeout == 0 {
		cfg.PollTimeout = 10 * time.Second
	}

	// Create state manager
	stateManager, err := NewStateManager(StateConfig{
		Path: cfg.StateDBPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		cfg:           cfg,
		logger:        cfg.Logger,
		stateManager:  stateManager,
		pollers:       make(map[string]IntegrationPoller),
		registrations: make(map[string]*PollTriggerRegistration),
		workflowFirer: cfg.WorkflowFirer,
		rateLimiter:   NewRateLimiter(),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Create metrics collector if meter provider is available
	if cfg.MeterProvider != nil {
		metrics, err := NewMetricsCollector(cfg.MeterProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics collector: %w", err)
		}
		s.metrics = metrics
	}

	// Create scheduler with poll handler
	s.scheduler = NewScheduler(s.handlePoll)

	return s, nil
}

// RegisterPoller registers an integration poller for a specific integration.
func (s *Service) RegisterPoller(poller IntegrationPoller) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := poller.Name()
	if _, exists := s.pollers[name]; exists {
		return fmt.Errorf("poller %s already registered", name)
	}

	s.pollers[name] = poller
	s.logger.Info("registered poll integration poller",
		slog.String("integration", name))

	return nil
}

// Start starts the poll trigger service.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("service already started")
	}

	s.started = true
	s.logger.Info("poll trigger service started")

	return nil
}

// Stop gracefully stops the poll trigger service.
// In-flight polls are allowed to complete within a timeout.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()

	s.logger.Info("stopping poll trigger service")

	// Cancel context to signal shutdown
	s.cancel()

	// Stop the scheduler
	s.scheduler.Stop()

	// Wait for in-flight polls with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("all poll triggers stopped")
	case <-ctx.Done():
		s.logger.Warn("poll trigger shutdown timed out, some polls may not have completed")
	}

	// Close state manager
	if err := s.stateManager.Close(); err != nil {
		return fmt.Errorf("failed to close state manager: %w", err)
	}

	return nil
}

// RegisterTrigger registers a poll trigger from a workflow configuration.
func (s *Service) RegisterTrigger(reg *PollTriggerRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate registration
	if reg.TriggerID == "" {
		return fmt.Errorf("trigger ID is required")
	}
	if reg.WorkflowPath == "" {
		return fmt.Errorf("workflow path is required")
	}
	if reg.Integration == "" {
		return fmt.Errorf("integration is required")
	}
	if reg.Interval < MinPollInterval {
		reg.Interval = MinPollInterval
	}

	// Validate query parameters to prevent injection attacks
	if err := ValidateQueryParameters(reg.Query); err != nil {
		return fmt.Errorf("invalid query parameters: %w", err)
	}

	// Check if integration poller is registered
	if _, exists := s.pollers[reg.Integration]; !exists {
		return fmt.Errorf("integration %s not registered", reg.Integration)
	}

	// Store registration
	s.registrations[reg.TriggerID] = reg

	// Register with scheduler
	if err := s.scheduler.Register(s.ctx, reg.TriggerID, reg.Interval); err != nil {
		return fmt.Errorf("failed to register with scheduler: %w", err)
	}

	// Update metrics
	if s.metrics != nil {
		s.metrics.SetActiveTriggers(len(s.registrations))
	}

	s.logger.Info("registered poll trigger",
		slog.String("trigger_id", reg.TriggerID),
		slog.String("integration", reg.Integration),
		slog.Int("interval", reg.Interval))

	return nil
}

// UnregisterTrigger removes a poll trigger.
func (s *Service) UnregisterTrigger(triggerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Unregister from scheduler
	s.scheduler.Unregister(triggerID)

	// Remove registration
	delete(s.registrations, triggerID)

	// Update metrics
	if s.metrics != nil {
		s.metrics.SetActiveTriggers(len(s.registrations))
	}

	// Optionally delete state (for now we keep it for debugging)
	// To fully clean up, you could call: s.stateManager.DeleteState(ctx, triggerID)

	s.logger.Info("unregistered poll trigger",
		slog.String("trigger_id", triggerID))

	return nil
}

// RegisterWorkflowTriggers scans a workflow and registers any poll triggers.
func (s *Service) RegisterWorkflowTriggers(workflowPath string, wf *workflow.Definition) error {
	if wf.Trigger == nil || wf.Trigger.Poll == nil {
		return nil
	}

	pollCfg := wf.Trigger.Poll

	// Generate trigger ID from workflow path and integration
	triggerID := fmt.Sprintf("%s:%s", workflowPath, pollCfg.Integration)

	// Convert query map
	query := make(map[string]interface{})
	for k, v := range pollCfg.Query {
		query[k] = v
	}

	// Convert input mapping
	inputMapping := make(map[string]string)
	for k, v := range pollCfg.InputMapping {
		inputMapping[k] = v
	}

	// Parse interval with default
	interval := 30 // default 30 seconds
	if pollCfg.Interval != "" {
		duration, err := time.ParseDuration(pollCfg.Interval)
		if err != nil {
			return fmt.Errorf("invalid interval %s: %w", pollCfg.Interval, err)
		}
		interval = int(duration.Seconds())
	}

	// Parse startup behavior
	startup := pollCfg.Startup
	if startup == "" {
		startup = "since_last"
	}

	// Parse backfill duration
	backfill := 0
	if pollCfg.Backfill != "" {
		duration, err := time.ParseDuration(pollCfg.Backfill)
		if err != nil {
			return fmt.Errorf("invalid backfill duration %s: %w", pollCfg.Backfill, err)
		}
		backfill = int(duration.Seconds())
	}

	reg := &PollTriggerRegistration{
		TriggerID:    triggerID,
		WorkflowPath: workflowPath,
		Integration:  pollCfg.Integration,
		Query:        query,
		Interval:     interval,
		Startup:      startup,
		Backfill:     backfill,
		InputMapping: inputMapping,
	}

	return s.RegisterTrigger(reg)
}

// handlePoll is called by the scheduler when a poll timer fires.
func (s *Service) handlePoll(ctx context.Context, triggerID string) error {
	s.wg.Add(1)
	defer s.wg.Done()

	// Add timeout to poll execution
	pollCtx, cancel := context.WithTimeout(ctx, s.cfg.PollTimeout)
	defer cancel()

	s.mu.RLock()
	reg, exists := s.registrations[triggerID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("trigger %s not registered", triggerID)
	}

	poller, exists := s.pollers[reg.Integration]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("integration %s not registered", reg.Integration)
	}
	s.mu.RUnlock()

	// Check rate limiter
	if err := s.rateLimiter.WaitIfNeeded(pollCtx, reg.Integration); err != nil {
		return fmt.Errorf("rate limit wait cancelled: %w", err)
	}

	// Get or create poll state
	state, err := s.stateManager.GetState(pollCtx, triggerID)
	if err != nil {
		return fmt.Errorf("failed to get state: %w", err)
	}

	if state == nil {
		// First poll - initialize state
		state = &PollState{
			TriggerID:    triggerID,
			WorkflowPath: reg.WorkflowPath,
			Integration:  reg.Integration,
			SeenEvents:   make(map[string]int64),
			CreatedAt:    time.Now(),
		}

		// Handle startup behavior
		switch reg.Startup {
		case "ignore_historical":
			// Start from now
			state.LastPollTime = time.Now()
		case "backfill":
			// Start from backfill duration ago
			if reg.Backfill > 0 {
				state.LastPollTime = time.Now().Add(-time.Duration(reg.Backfill) * time.Second)
			} else {
				state.LastPollTime = time.Now()
			}
		default: // "since_last" or empty
			// On first run, use current time
			state.LastPollTime = time.Now()
		}

		s.logger.Info("initialized poll state",
			slog.String("trigger_id", triggerID),
			slog.String("startup", reg.Startup),
			slog.Time("last_poll_time", state.LastPollTime))
	}

	pollTime := time.Now()
	pollStart := time.Now()

	// Execute the poll
	events, cursor, err := poller.Poll(pollCtx, state, reg.Query)
	pollDuration := time.Since(pollStart)

	if err != nil {
		// Record error in rate limiter and metrics
		s.rateLimiter.RecordError(reg.Integration)
		if s.metrics != nil {
			s.metrics.RecordPollComplete(pollCtx, reg.Integration, false, pollDuration)
			s.metrics.RecordError(pollCtx, reg.Integration, "poll_failed")
		}

		// Update error count
		state.ErrorCount++
		state.LastError = err.Error()

		// Circuit breaker logic: pause after 10 consecutive errors
		if state.ErrorCount >= 10 {
			s.logger.Error("poll trigger paused after 10 consecutive errors - manual reset required",
				slog.String("trigger_id", triggerID),
				slog.String("integration", reg.Integration),
				slog.String("error", err.Error()))
			// Unregister from scheduler to pause polling
			s.scheduler.Unregister(triggerID)
		} else if state.ErrorCount >= 5 {
			s.logger.Error("poll trigger experiencing errors",
				slog.String("trigger_id", triggerID),
				slog.String("integration", reg.Integration),
				slog.Int("error_count", state.ErrorCount),
				slog.String("error", err.Error()))
		} else {
			s.logger.Warn("poll failed",
				slog.String("trigger_id", triggerID),
				slog.String("integration", reg.Integration),
				slog.Int("error_count", state.ErrorCount),
				slog.String("error", err.Error()))
		}

		// Save state with error
		if saveErr := s.stateManager.SaveState(pollCtx, state); saveErr != nil {
			s.logger.Error("failed to save error state",
				slog.String("trigger_id", triggerID),
				slog.String("error", saveErr.Error()))
		}

		return err
	}

	// Poll succeeded - clear error count and record success
	state.ErrorCount = 0
	state.LastError = ""
	s.rateLimiter.RecordSuccess(reg.Integration)

	if s.metrics != nil {
		s.metrics.RecordPollComplete(pollCtx, reg.Integration, true, pollDuration)
	}

	// Process events
	newEventCount := 0
	for _, event := range events {
		// Extract event ID for deduplication
		eventID, ok := event["id"].(string)
		if !ok {
			s.logger.Warn("event missing id field, skipping",
				slog.String("trigger_id", triggerID))
			continue
		}

		// Check if we've seen this event before
		if _, seen := state.SeenEvents[eventID]; seen {
			continue
		}

		// Mark as seen
		state.SeenEvents[eventID] = time.Now().Unix()
		newEventCount++

		// Update high water mark if this event is newer
		if eventTime, ok := extractEventTimestamp(event); ok {
			if eventTime.After(state.HighWaterMark) {
				state.HighWaterMark = eventTime
			}
		}

		// Strip sensitive fields from event before passing to workflow
		cleanedEvent := StripSensitiveFields(event, reg.Integration)

		// Fire the workflow
		triggerContext := &PollTriggerContext{
			Integration: reg.Integration,
			TriggerTime: time.Now(),
			PollTime:    pollTime,
			Event:       cleanedEvent,
			Query:       reg.Query,
		}

		if err := s.workflowFirer(pollCtx, reg.WorkflowPath, triggerContext); err != nil {
			s.logger.Error("failed to fire workflow",
				slog.String("trigger_id", triggerID),
				slog.String("workflow", reg.WorkflowPath),
				slog.String("event_id", eventID),
				slog.String("error", err.Error()))
			// Continue processing other events
		}
	}

	// Update last poll time on success
	state.LastPollTime = pollTime
	state.Cursor = cursor
	state.UpdatedAt = time.Now()

	// Prune old seen events
	s.stateManager.PruneSeenEvents(state, 86400, 10000) // 24h TTL, 10k max

	// Save updated state
	if err := s.stateManager.SaveState(pollCtx, state); err != nil {
		s.logger.Error("failed to save state",
			slog.String("trigger_id", triggerID),
			slog.String("error", err.Error()))
		return err
	}

	// Record event metrics
	if s.metrics != nil && newEventCount > 0 {
		s.metrics.RecordEvents(pollCtx, reg.Integration, "poll_event", newEventCount)
	}

	if newEventCount > 0 {
		s.logger.Info("poll trigger fired",
			slog.String("trigger_id", triggerID),
			slog.String("integration", reg.Integration),
			slog.Int("new_events", newEventCount))
	}

	return nil
}

// extractEventTimestamp attempts to extract a timestamp from an event.
// Returns zero time if not found or invalid.
func extractEventTimestamp(event map[string]interface{}) (time.Time, bool) {
	// Try common timestamp field names
	for _, field := range []string{"created_at", "timestamp", "updated_at", "time"} {
		if val, ok := event[field]; ok {
			switch v := val.(type) {
			case string:
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					return t, true
				}
			case time.Time:
				return v, true
			}
		}
	}
	return time.Time{}, false
}
