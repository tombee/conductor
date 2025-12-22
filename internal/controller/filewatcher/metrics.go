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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// fileWatcherEvents tracks total file events received by watcher
	fileWatcherEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_filewatcher_events_total",
			Help: "Total file watcher events by watcher name and event type",
		},
		[]string{"watcher", "event_type"},
	)

	// fileWatcherTriggers tracks total workflow triggers
	fileWatcherTriggers = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_filewatcher_triggers_total",
			Help: "Total workflow triggers by watcher name",
		},
		[]string{"watcher"},
	)

	// fileWatcherErrors tracks errors during event processing
	fileWatcherErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_filewatcher_errors_total",
			Help: "Total file watcher errors by watcher name and error type",
		},
		[]string{"watcher", "error_type"},
	)

	// fileWatcherActive tracks number of active watchers
	fileWatcherActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "conductor_filewatcher_active_watchers",
			Help: "Number of currently active file watchers",
		},
	)

	// fileWatcherRateLimited tracks rate-limited events
	fileWatcherRateLimited = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_filewatcher_rate_limited_total",
			Help: "Total rate-limited events by watcher name",
		},
		[]string{"watcher"},
	)

	// fileWatcherPatternExcluded tracks pattern-excluded events
	fileWatcherPatternExcluded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_filewatcher_pattern_excluded_total",
			Help: "Total pattern-excluded events by watcher name",
		},
		[]string{"watcher"},
	)
)

// recordEvent increments the event counter
func recordEvent(watcher, eventType string) {
	fileWatcherEvents.WithLabelValues(watcher, eventType).Inc()
}

// recordTrigger increments the trigger counter
func recordTrigger(watcher string) {
	fileWatcherTriggers.WithLabelValues(watcher).Inc()
}

// recordError increments the error counter
func recordError(watcher, errorType string) {
	fileWatcherErrors.WithLabelValues(watcher, errorType).Inc()
}

// recordRateLimited increments the rate-limited counter
func recordRateLimited(watcher string) {
	fileWatcherRateLimited.WithLabelValues(watcher).Inc()
}

// recordPatternExcluded increments the pattern-excluded counter
func recordPatternExcluded(watcher string) {
	fileWatcherPatternExcluded.WithLabelValues(watcher).Inc()
}
