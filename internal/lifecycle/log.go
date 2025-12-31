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

package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LifecycleEvent represents a lifecycle event (start, stop, etc.).
type LifecycleEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Event      string            `json:"event"`       // "start", "stop", "health_check_failed", etc.
	PID        int               `json:"pid,omitempty"`
	Version    string            `json:"version,omitempty"`
	ExitCode   int               `json:"exit_code,omitempty"`
	Success    bool              `json:"success"`
	Message    string            `json:"message,omitempty"`
	Flags      map[string]string `json:"flags,omitempty"`
	ConfigFile string            `json:"config_file,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// LifecycleLogger logs controller lifecycle events to a file.
type LifecycleLogger struct {
	logPath string
}

// NewLifecycleLogger creates a new lifecycle logger.
func NewLifecycleLogger(logPath string) *LifecycleLogger {
	return &LifecycleLogger{
		logPath: logPath,
	}
}

// LogStart logs a controller start event.
func (l *LifecycleLogger) LogStart(version string, args []string, configFile string) error {
	event := LifecycleEvent{
		Timestamp:  time.Now(),
		Event:      "start",
		Version:    version,
		Success:    true,
		Message:    "Controller start initiated",
		Flags:      parseFlags(args),
		ConfigFile: configFile,
	}
	return l.writeEvent(event)
}

// LogStartSuccess logs successful controller startup with PID.
func (l *LifecycleLogger) LogStartSuccess(pid int, healthCheckAttempts int, duration time.Duration) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "start_success",
		PID:       pid,
		Success:   true,
		Message:   fmt.Sprintf("Controller started successfully (health checks: %d, duration: %v)", healthCheckAttempts, duration),
	}
	return l.writeEvent(event)
}

// LogStartFailure logs failed controller startup.
func (l *LifecycleLogger) LogStartFailure(err error) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "start_failure",
		Success:   false,
		Message:   "Controller failed to start",
		Error:     err.Error(),
	}
	return l.writeEvent(event)
}

// LogStop logs a controller stop event.
func (l *LifecycleLogger) LogStop(pid int, force bool) error {
	message := "Controller stop initiated"
	if force {
		message = "Controller force stop initiated"
	}

	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "stop",
		PID:       pid,
		Success:   true,
		Message:   message,
	}
	return l.writeEvent(event)
}

// LogStopSuccess logs successful controller shutdown.
func (l *LifecycleLogger) LogStopSuccess(pid int, duration time.Duration) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "stop_success",
		PID:       pid,
		Success:   true,
		Message:   fmt.Sprintf("Controller stopped successfully (duration: %v)", duration),
	}
	return l.writeEvent(event)
}

// LogStopFailure logs failed controller shutdown.
func (l *LifecycleLogger) LogStopFailure(pid int, err error) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "stop_failure",
		PID:       pid,
		Success:   false,
		Message:   "Failed to stop controller",
		Error:     err.Error(),
	}
	return l.writeEvent(event)
}

// LogHealthCheckFailed logs a failed health check during startup.
func (l *LifecycleLogger) LogHealthCheckFailed(endpoint string, attempts int, responseTime time.Duration, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "health_check_failed",
		Success:   false,
		Message:   fmt.Sprintf("Health check failed (endpoint: %s, attempts: %d, response time: %v)", endpoint, attempts, responseTime),
		Error:     errMsg,
	}
	return l.writeEvent(event)
}

// LogStalePID logs detection of a stale PID file.
func (l *LifecycleLogger) LogStalePID(pid int, reason string) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "stale_pid_detected",
		PID:       pid,
		Success:   true,
		Message:   fmt.Sprintf("Stale PID file detected and removed: %s", reason),
	}
	return l.writeEvent(event)
}

// LogAlreadyRunning logs that the controller is already running.
func (l *LifecycleLogger) LogAlreadyRunning(pid int) error {
	event := LifecycleEvent{
		Timestamp: time.Now(),
		Event:     "already_running",
		PID:       pid,
		Success:   true,
		Message:   "Controller already running",
	}
	return l.writeEvent(event)
}

// writeEvent appends a lifecycle event to the log file.
func (l *LifecycleLogger) writeEvent(event LifecycleEvent) error {
	// Ensure log directory exists
	logDir := filepath.Dir(l.logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file in append mode
	f, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lifecycle log: %w", err)
	}
	defer f.Close()

	// Write as JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// parseFlags converts command-line arguments to a map of flags.
// This is a simple parser for logging purposes.
func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Skip non-flag arguments
		if !strings.HasPrefix(arg, "-") {
			continue
		}

		// Remove leading dashes
		key := strings.TrimLeft(arg, "-")

		// Check if next arg is the value (not another flag)
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags[key] = args[i+1]
			i++ // Skip value in next iteration
		} else {
			// Boolean flag
			flags[key] = "true"
		}
	}

	return flags
}
