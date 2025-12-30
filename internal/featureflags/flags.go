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

// Package featureflags provides runtime feature flag management for Conductor.
package featureflags

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Flags holds all feature flags with thread-safe access.
type Flags struct {
	mu sync.RWMutex

	// Debug feature flags
	DebugTimelineEnabled  bool
	DebugDryRunDeepEnabled bool
	DebugReplayEnabled    bool
	DebugSSEEnabled       bool
}

var (
	// globalFlags is the singleton instance of feature flags
	globalFlags *Flags
	once        sync.Once
)

// Get returns the global feature flags instance.
func Get() *Flags {
	once.Do(func() {
		globalFlags = &Flags{
			// All debug features enabled by default
			DebugTimelineEnabled:   true,
			DebugDryRunDeepEnabled: true,
			DebugReplayEnabled:     true,
			DebugSSEEnabled:        true,
		}
		globalFlags.loadFromEnv()
	})
	return globalFlags
}

// loadFromEnv loads feature flags from environment variables.
// Environment variables override default values.
func (f *Flags) loadFromEnv() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if val := os.Getenv("DEBUG_TIMELINE_ENABLED"); val != "" {
		f.DebugTimelineEnabled = parseBool(val)
	}
	if val := os.Getenv("DEBUG_DRYRUN_DEEP_ENABLED"); val != "" {
		f.DebugDryRunDeepEnabled = parseBool(val)
	}
	if val := os.Getenv("DEBUG_REPLAY_ENABLED"); val != "" {
		f.DebugReplayEnabled = parseBool(val)
	}
	if val := os.Getenv("DEBUG_SSE_ENABLED"); val != "" {
		f.DebugSSEEnabled = parseBool(val)
	}
}

// IsTimelineEnabled returns whether timeline visualization is enabled.
func (f *Flags) IsTimelineEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.DebugTimelineEnabled
}

// IsDryRunDeepEnabled returns whether deep dry-run mode is enabled.
func (f *Flags) IsDryRunDeepEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.DebugDryRunDeepEnabled
}

// IsReplayEnabled returns whether workflow replay is enabled.
func (f *Flags) IsReplayEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.DebugReplayEnabled
}

// IsSSEEnabled returns whether SSE debugging is enabled.
func (f *Flags) IsSSEEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.DebugSSEEnabled
}

// SetTimelineEnabled sets the timeline enabled flag (for testing).
func (f *Flags) SetTimelineEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DebugTimelineEnabled = enabled
}

// SetDryRunDeepEnabled sets the dry-run deep enabled flag (for testing).
func (f *Flags) SetDryRunDeepEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DebugDryRunDeepEnabled = enabled
}

// SetReplayEnabled sets the replay enabled flag (for testing).
func (f *Flags) SetReplayEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DebugReplayEnabled = enabled
}

// SetSSEEnabled sets the SSE enabled flag (for testing).
func (f *Flags) SetSSEEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.DebugSSEEnabled = enabled
}

// parseBool converts a string to a boolean value.
// Accepts: "1", "t", "T", "true", "TRUE", "True"
func parseBool(val string) bool {
	val = strings.TrimSpace(val)
	if b, err := strconv.ParseBool(val); err == nil {
		return b
	}
	return false
}
