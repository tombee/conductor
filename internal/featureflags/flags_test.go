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

package featureflags

import (
	"os"
	"testing"
)

func TestFlags_Defaults(t *testing.T) {
	// Create a fresh instance to test defaults
	f := &Flags{}
	f.loadFromEnv()

	// All flags should be false when no env vars are set
	// (since we don't set defaults in loadFromEnv, only in Get())
	if f.DebugTimelineEnabled {
		t.Error("expected DebugTimelineEnabled to be false by default in fresh instance")
	}
	if f.DebugDryRunDeepEnabled {
		t.Error("expected DebugDryRunDeepEnabled to be false by default in fresh instance")
	}
	if f.DebugReplayEnabled {
		t.Error("expected DebugReplayEnabled to be false by default in fresh instance")
	}
	if f.DebugSSEEnabled {
		t.Error("expected DebugSSEEnabled to be false by default in fresh instance")
	}
}

func TestFlags_LoadFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		check    func(*Flags) bool
	}{
		{
			name:     "timeline enabled true",
			envKey:   "DEBUG_TIMELINE_ENABLED",
			envValue: "true",
			check:    func(f *Flags) bool { return f.DebugTimelineEnabled },
		},
		{
			name:     "timeline enabled 1",
			envKey:   "DEBUG_TIMELINE_ENABLED",
			envValue: "1",
			check:    func(f *Flags) bool { return f.DebugTimelineEnabled },
		},
		{
			name:     "timeline disabled false",
			envKey:   "DEBUG_TIMELINE_ENABLED",
			envValue: "false",
			check:    func(f *Flags) bool { return !f.DebugTimelineEnabled },
		},
		{
			name:     "timeline disabled 0",
			envKey:   "DEBUG_TIMELINE_ENABLED",
			envValue: "0",
			check:    func(f *Flags) bool { return !f.DebugTimelineEnabled },
		},
		{
			name:     "dryrun deep enabled",
			envKey:   "DEBUG_DRYRUN_DEEP_ENABLED",
			envValue: "true",
			check:    func(f *Flags) bool { return f.DebugDryRunDeepEnabled },
		},
		{
			name:     "replay enabled",
			envKey:   "DEBUG_REPLAY_ENABLED",
			envValue: "true",
			check:    func(f *Flags) bool { return f.DebugReplayEnabled },
		},
		{
			name:     "sse enabled",
			envKey:   "DEBUG_SSE_ENABLED",
			envValue: "true",
			check:    func(f *Flags) bool { return f.DebugSSEEnabled },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			os.Setenv(tt.envKey, tt.envValue)
			defer os.Unsetenv(tt.envKey)

			// Create fresh instance and load
			f := &Flags{}
			f.loadFromEnv()

			// Check result
			if !tt.check(f) {
				t.Errorf("flag check failed for %s=%s", tt.envKey, tt.envValue)
			}
		})
	}
}

func TestFlags_Getters(t *testing.T) {
	f := &Flags{
		DebugTimelineEnabled:   true,
		DebugDryRunDeepEnabled: false,
		DebugReplayEnabled:     true,
		DebugSSEEnabled:        false,
	}

	if !f.IsTimelineEnabled() {
		t.Error("expected IsTimelineEnabled to return true")
	}
	if f.IsDryRunDeepEnabled() {
		t.Error("expected IsDryRunDeepEnabled to return false")
	}
	if !f.IsReplayEnabled() {
		t.Error("expected IsReplayEnabled to return true")
	}
	if f.IsSSEEnabled() {
		t.Error("expected IsSSEEnabled to return false")
	}
}

func TestFlags_Setters(t *testing.T) {
	f := &Flags{}

	f.SetTimelineEnabled(true)
	if !f.DebugTimelineEnabled {
		t.Error("SetTimelineEnabled failed")
	}

	f.SetDryRunDeepEnabled(true)
	if !f.DebugDryRunDeepEnabled {
		t.Error("SetDryRunDeepEnabled failed")
	}

	f.SetReplayEnabled(true)
	if !f.DebugReplayEnabled {
		t.Error("SetReplayEnabled failed")
	}

	f.SetSSEEnabled(true)
	if !f.DebugSSEEnabled {
		t.Error("SetSSEEnabled failed")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"t", true},
		{"T", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
		{"f", false},
		{"F", false},
		{"", false},
		{"invalid", false},
		{" true ", true},
		{" false ", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
