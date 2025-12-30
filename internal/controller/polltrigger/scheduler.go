package polltrigger

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// MinPollInterval is the minimum allowed polling interval in seconds.
const MinPollInterval = 10

// Scheduler manages poll timers for registered triggers.
// It creates per-trigger timers with jitter to avoid thundering herd issues.
type Scheduler struct {
	mu      sync.RWMutex
	timers  map[string]*pollTimer
	handler PollHandler
	stopped bool
}

// PollHandler is called when a poll timer fires.
type PollHandler func(ctx context.Context, triggerID string) error

// pollTimer tracks the timer and configuration for a single poll trigger.
type pollTimer struct {
	triggerID string
	interval  time.Duration
	timer     *time.Timer
	cancel    context.CancelFunc
	stopped   bool
}

// NewScheduler creates a new poll trigger scheduler.
func NewScheduler(handler PollHandler) *Scheduler {
	return &Scheduler{
		timers:  make(map[string]*pollTimer),
		handler: handler,
	}
}

// Register adds or updates a poll trigger with the given interval.
// The interval is enforced to be at least MinPollInterval seconds.
// Jitter (±10%) is added to the interval to avoid thundering herd.
func (s *Scheduler) Register(ctx context.Context, triggerID string, intervalSeconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return fmt.Errorf("scheduler is stopped")
	}

	// Enforce minimum interval
	if intervalSeconds < MinPollInterval {
		intervalSeconds = MinPollInterval
	}

	interval := time.Duration(intervalSeconds) * time.Second

	// Check if timer already exists
	if existing, exists := s.timers[triggerID]; exists {
		// Update interval if changed
		if existing.interval != interval {
			existing.cancel()
			delete(s.timers, triggerID)
		} else {
			// Same interval, no change needed
			return nil
		}
	}

	// Create new timer with jitter
	jitteredInterval := addJitter(interval)

	timerCtx, cancel := context.WithCancel(ctx)
	timer := time.NewTimer(jitteredInterval)

	pt := &pollTimer{
		triggerID: triggerID,
		interval:  interval,
		timer:     timer,
		cancel:    cancel,
	}

	s.timers[triggerID] = pt

	// Start goroutine to handle timer fires
	go s.runTimer(timerCtx, pt)

	return nil
}

// Unregister removes a poll trigger from the scheduler.
func (s *Scheduler) Unregister(triggerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pt, exists := s.timers[triggerID]; exists {
		pt.stopped = true
		pt.cancel()
		pt.timer.Stop()
		delete(s.timers, triggerID)
	}
}

// runTimer handles timer fires and reschedules for a single poll trigger.
func (s *Scheduler) runTimer(ctx context.Context, pt *pollTimer) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pt.timer.C:
			if pt.stopped {
				return
			}

			// Call the poll handler
			if s.handler != nil {
				if err := s.handler(ctx, pt.triggerID); err != nil {
					// Error handling is done by the handler itself
					// We just log and continue
				}
			}

			// Reschedule with jitter
			jitteredInterval := addJitter(pt.interval)
			pt.timer.Reset(jitteredInterval)
		}
	}
}

// Stop stops all timers and shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopped = true

	for _, pt := range s.timers {
		pt.stopped = true
		pt.cancel()
		pt.timer.Stop()
	}

	s.timers = make(map[string]*pollTimer)
}

// GetInterval returns the configured interval for a trigger in seconds.
// Returns 0 if the trigger is not registered.
func (s *Scheduler) GetInterval(triggerID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pt, exists := s.timers[triggerID]; exists {
		return int(pt.interval.Seconds())
	}
	return 0
}

// ListTriggers returns a list of all registered trigger IDs.
func (s *Scheduler) ListTriggers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	triggers := make([]string, 0, len(s.timers))
	for triggerID := range s.timers {
		triggers = append(triggers, triggerID)
	}
	return triggers
}

// addJitter adds ±10% jitter to a duration to avoid thundering herd.
func addJitter(d time.Duration) time.Duration {
	// Calculate 10% of the duration
	jitterRange := float64(d) * 0.1

	// Random value between -10% and +10%
	jitter := (rand.Float64()*2 - 1) * jitterRange

	return d + time.Duration(jitter)
}
