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

// Package audit provides multi-destination audit logging for security events.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/syslog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event represents a security audit event.
type Event struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	WorkflowID   string                 `json:"workflow_id,omitempty"`
	StepID       string                 `json:"step_id,omitempty"`
	ToolName     string                 `json:"tool_name,omitempty"`
	Resource     string                 `json:"resource,omitempty"`
	ResourceType string                 `json:"resource_type,omitempty"`
	Action       string                 `json:"action,omitempty"`
	Decision     string                 `json:"decision"`
	Reason       string                 `json:"reason,omitempty"`
	Profile      string                 `json:"profile,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Destination represents an audit log destination.
type Destination interface {
	// Write writes an audit event to the destination
	Write(event Event) error

	// Close closes the destination
	Close() error
}

// Logger manages multiple audit destinations.
type Logger struct {
	mu           sync.RWMutex
	destinations []Destination
	buffer       chan Event
	bufferSize   int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// Config configures the audit logger.
type Config struct {
	Destinations []DestinationConfig `yaml:"destinations" json:"destinations"`
	BufferSize   int                 `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
}

// DestinationConfig configures a single destination.
type DestinationConfig struct {
	Type     string            `yaml:"type" json:"type"`
	Path     string            `yaml:"path,omitempty" json:"path,omitempty"`
	Format   string            `yaml:"format,omitempty" json:"format,omitempty"`
	Facility string            `yaml:"facility,omitempty" json:"facility,omitempty"`
	Severity string            `yaml:"severity,omitempty" json:"severity,omitempty"`
	URL      string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Rotation settings (for type=rotating-file)
	MaxSize     int64         `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	MaxAge      time.Duration `yaml:"max_age,omitempty" json:"max_age,omitempty"`
	RotateDaily bool          `yaml:"rotate_daily,omitempty" json:"rotate_daily,omitempty"`
	Compress    bool          `yaml:"compress,omitempty" json:"compress,omitempty"`
}

const (
	// DefaultBufferSize is the default size of the event buffer
	DefaultBufferSize = 1000
)

// NewLogger creates a new audit logger with the given configuration.
func NewLogger(config Config) (*Logger, error) {
	if config.BufferSize == 0 {
		config.BufferSize = DefaultBufferSize
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := &Logger{
		destinations: make([]Destination, 0, len(config.Destinations)),
		buffer:       make(chan Event, config.BufferSize),
		bufferSize:   config.BufferSize,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Create destinations
	for _, destConfig := range config.Destinations {
		dest, err := createDestination(destConfig)
		if err != nil {
			// Clean up already created destinations
			logger.Close()
			return nil, fmt.Errorf("failed to create %s destination: %w", destConfig.Type, err)
		}
		logger.destinations = append(logger.destinations, dest)
	}

	// Start background writer
	logger.wg.Add(1)
	go logger.writeLoop()

	return logger, nil
}

// Log logs an audit event to all destinations.
func (l *Logger) Log(event Event) {
	select {
	case l.buffer <- event:
		// Event buffered successfully
	default:
		// Buffer full - log to stderr as fallback
		fmt.Fprintf(os.Stderr, "AUDIT WARNING: Buffer full, dropping event: %+v\n", event)
	}
}

// Close closes all destinations and stops the logger.
func (l *Logger) Close() error {
	l.cancel()
	l.wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()

	var firstErr error
	for _, dest := range l.destinations {
		if err := dest.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// BufferUtilization returns the percentage of buffer usage (0.0 to 1.0).
func (l *Logger) BufferUtilization() float64 {
	return float64(len(l.buffer)) / float64(l.bufferSize)
}

// writeLoop processes events from the buffer and writes to destinations.
func (l *Logger) writeLoop() {
	defer l.wg.Done()

	for {
		select {
		case event := <-l.buffer:
			l.writeToDestinations(event)
		case <-l.ctx.Done():
			// Drain remaining events
			for {
				select {
				case event := <-l.buffer:
					l.writeToDestinations(event)
				default:
					return
				}
			}
		}
	}
}

// writeToDestinations writes an event to all configured destinations.
func (l *Logger) writeToDestinations(event Event) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, dest := range l.destinations {
		if err := dest.Write(event); err != nil {
			// Log to stderr if destination write fails
			fmt.Fprintf(os.Stderr, "AUDIT ERROR: Failed to write to destination: %v\n", err)
		}
	}
}

// createDestination creates a destination from configuration.
func createDestination(config DestinationConfig) (Destination, error) {
	switch config.Type {
	case "file":
		return NewFileDestination(config)
	case "rotating-file":
		// For rotating files, convert DestinationConfig to RotationConfig
		rotationCfg := RotationConfig{
			Path:        config.Path,
			Format:      config.Format,
			MaxSize:     config.MaxSize,
			MaxAge:      config.MaxAge,
			RotateDaily: config.RotateDaily,
			Compress:    config.Compress,
		}
		// Apply defaults if not set
		if rotationCfg.MaxSize == 0 {
			rotationCfg.MaxSize = DefaultMaxSize
		}
		if rotationCfg.MaxAge == 0 {
			rotationCfg.MaxAge = DefaultMaxAge
		}
		return NewRotatingFileDestination(rotationCfg)
	case "syslog":
		return NewSyslogDestination(config)
	case "webhook":
		return NewWebhookDestination(config)
	default:
		return nil, fmt.Errorf("unknown destination type: %s", config.Type)
	}
}

// FileDestination writes audit events to a file.
type FileDestination struct {
	mu     sync.Mutex
	file   *os.File
	format string
}

// NewFileDestination creates a new file destination.
func NewFileDestination(config DestinationConfig) (*FileDestination, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("file destination requires path")
	}

	// Expand home directory
	path := config.Path
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file for append
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	format := config.Format
	if format == "" {
		format = "json"
	}

	return &FileDestination{
		file:   file,
		format: format,
	}, nil
}

// Write writes an event to the file.
func (d *FileDestination) Write(event Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var line []byte
	var err error

	switch d.format {
	case "json":
		line, err = json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		line = append(line, '\n')
	case "text":
		line = []byte(fmt.Sprintf("[%s] %s %s %s decision=%s reason=%s\n",
			event.Timestamp.Format(time.RFC3339),
			event.EventType,
			event.ResourceType,
			event.Resource,
			event.Decision,
			event.Reason,
		))
	default:
		return fmt.Errorf("unknown format: %s", d.format)
	}

	_, err = d.file.Write(line)
	return err
}

// Close closes the file.
func (d *FileDestination) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.file.Close()
}

// SyslogDestination writes audit events to syslog.
type SyslogDestination struct {
	writer *syslog.Writer
}

// NewSyslogDestination creates a new syslog destination.
func NewSyslogDestination(config DestinationConfig) (*SyslogDestination, error) {
	priority := syslog.LOG_INFO | syslog.LOG_USER

	// Parse facility
	if config.Facility != "" {
		switch config.Facility {
		case "user":
			priority |= syslog.LOG_USER
		case "daemon":
			priority |= syslog.LOG_DAEMON
		case "auth":
			priority |= syslog.LOG_AUTH
		case "local0":
			priority |= syslog.LOG_LOCAL0
		case "local1":
			priority |= syslog.LOG_LOCAL1
		default:
			return nil, fmt.Errorf("unknown syslog facility: %s", config.Facility)
		}
	}

	// Parse severity
	if config.Severity != "" {
		switch config.Severity {
		case "emerg":
			priority |= syslog.LOG_EMERG
		case "alert":
			priority |= syslog.LOG_ALERT
		case "crit":
			priority |= syslog.LOG_CRIT
		case "err":
			priority |= syslog.LOG_ERR
		case "warning":
			priority |= syslog.LOG_WARNING
		case "notice":
			priority |= syslog.LOG_NOTICE
		case "info":
			priority |= syslog.LOG_INFO
		case "debug":
			priority |= syslog.LOG_DEBUG
		default:
			return nil, fmt.Errorf("unknown syslog severity: %s", config.Severity)
		}
	}

	writer, err := syslog.New(priority, "conductor")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to syslog: %w", err)
	}

	return &SyslogDestination{writer: writer}, nil
}

// Write writes an event to syslog.
func (d *SyslogDestination) Write(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return d.writer.Info(string(data))
}

// Close closes the syslog connection.
func (d *SyslogDestination) Close() error {
	return d.writer.Close()
}

// WebhookDestination sends audit events to a webhook.
type WebhookDestination struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewWebhookDestination creates a new webhook destination.
func NewWebhookDestination(config DestinationConfig) (*WebhookDestination, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("webhook destination requires url")
	}

	return &WebhookDestination{
		url:     config.URL,
		headers: config.Headers,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Write sends an event to the webhook.
func (d *WebhookDestination) Write(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	req, err := http.NewRequest("POST", d.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range d.headers {
		req.Header.Set(key, value)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the HTTP client.
func (d *WebhookDestination) Close() error {
	d.client.CloseIdleConnections()
	return nil
}
