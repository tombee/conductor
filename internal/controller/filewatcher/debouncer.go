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

package filewatcher

import (
	"sync"
	"time"
)

// Debouncer manages per-file debounce timers to prevent duplicate triggers
// for rapid file changes (e.g., multiple editor saves).
//
// Debouncing works by delaying event delivery until no new events arrive
// for the configured window duration. Each file has its own timer.
//
// Batch mode accumulates all events during the debounce window and delivers
// them together when the timer expires.
type Debouncer struct {
	mu        sync.Mutex
	window    time.Duration
	batch     bool
	timers    map[string]*debounceTimer
	onFlush   func([]*Context)
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// debounceTimer tracks a pending timer for a specific file path.
type debounceTimer struct {
	timer  *time.Timer
	events []*Context // For batch mode
}

// NewDebouncer creates a new debouncer with the specified window duration.
// If batch is true, all events during the window are delivered together.
// If batch is false, only the last event is delivered.
// The onFlush callback is called when events are ready to be delivered.
func NewDebouncer(window time.Duration, batch bool, onFlush func([]*Context)) *Debouncer {
	return &Debouncer{
		window:    window,
		batch:     batch,
		timers:    make(map[string]*debounceTimer),
		onFlush:   onFlush,
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
}

// Add adds an event to the debouncer.
// It resets the timer for the file if one exists, or creates a new timer.
func (d *Debouncer) Add(ctx *Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.stopCh:
		// Debouncer is stopped, don't accept new events
		return
	default:
	}

	path := ctx.Path
	dt, exists := d.timers[path]

	if exists {
		// Stop existing timer
		dt.timer.Stop()

		if d.batch {
			// Accumulate event in batch mode
			dt.events = append(dt.events, ctx)
		} else {
			// Replace with latest event in non-batch mode
			dt.events = []*Context{ctx}
		}
	} else {
		// Create new timer entry
		dt = &debounceTimer{
			events: []*Context{ctx},
		}
		d.timers[path] = dt
	}

	// Create new timer
	dt.timer = time.AfterFunc(d.window, func() {
		d.flush(path)
	})
}

// flush delivers accumulated events for a path and cleans up the timer.
func (d *Debouncer) flush(path string) {
	d.mu.Lock()

	dt, exists := d.timers[path]
	if !exists {
		d.mu.Unlock()
		return
	}

	events := dt.events
	delete(d.timers, path)
	d.mu.Unlock()

	// Call onFlush outside of lock to prevent deadlocks
	if d.onFlush != nil && len(events) > 0 {
		d.onFlush(events)
	}
}

// Stop stops the debouncer and flushes all pending events.
// It blocks until all timers have been cleaned up.
func (d *Debouncer) Stop() {
	d.mu.Lock()

	// Signal that we're stopping
	select {
	case <-d.stopCh:
		// Already stopped
		d.mu.Unlock()
		return
	default:
		close(d.stopCh)
	}

	// Stop all timers and collect events
	var allEvents []*Context
	for path, dt := range d.timers {
		dt.timer.Stop()
		if d.batch {
			allEvents = append(allEvents, dt.events...)
		} else if len(dt.events) > 0 {
			allEvents = append(allEvents, dt.events[len(dt.events)-1])
		}
		delete(d.timers, path)
	}

	d.mu.Unlock()

	// Flush accumulated events
	if d.onFlush != nil && len(allEvents) > 0 {
		d.onFlush(allEvents)
	}

	close(d.stoppedCh)
}

// Wait blocks until the debouncer has fully stopped.
func (d *Debouncer) Wait() {
	<-d.stoppedCh
}

// Pending returns the number of files with pending timers.
func (d *Debouncer) Pending() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.timers)
}
