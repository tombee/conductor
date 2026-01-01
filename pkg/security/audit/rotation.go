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

package audit

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultMaxSize is the default maximum file size before rotation (1GB)
	DefaultMaxSize = 1024 * 1024 * 1024

	// DefaultMaxAge is the default retention period for rotated logs (90 days)
	DefaultMaxAge = 90 * 24 * time.Hour

	// DefaultRotateDaily enables daily rotation
	DefaultRotateDaily = true
)

// RotatingFileDestination is a file destination that rotates logs.
type RotatingFileDestination struct {
	mu          sync.Mutex
	basePath    string
	currentPath string
	file        *os.File
	format      string
	maxSize     int64
	maxAge      time.Duration
	rotateDaily bool
	currentSize int64
	currentDate string
	compress    bool
}

// RotationConfig configures log rotation.
type RotationConfig struct {
	Path        string        `yaml:"path" json:"path"`
	Format      string        `yaml:"format,omitempty" json:"format,omitempty"`
	MaxSize     int64         `yaml:"max_size,omitempty" json:"max_size,omitempty"`
	MaxAge      time.Duration `yaml:"max_age,omitempty" json:"max_age,omitempty"`
	RotateDaily bool          `yaml:"rotate_daily,omitempty" json:"rotate_daily,omitempty"`
	Compress    bool          `yaml:"compress,omitempty" json:"compress,omitempty"`
}

// NewRotatingFileDestination creates a new rotating file destination.
func NewRotatingFileDestination(config RotationConfig) (*RotatingFileDestination, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("rotating file destination requires path")
	}

	// Set defaults
	if config.MaxSize == 0 {
		config.MaxSize = DefaultMaxSize
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultMaxAge
	}
	if config.Format == "" {
		config.Format = "json"
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

	dest := &RotatingFileDestination{
		basePath:    path,
		currentPath: path,
		format:      config.Format,
		maxSize:     config.MaxSize,
		maxAge:      config.MaxAge,
		rotateDaily: config.RotateDaily,
		compress:    config.Compress,
		currentDate: time.Now().Format("2006-01-02"),
	}

	// Open initial file
	if err := dest.openFile(); err != nil {
		return nil, err
	}

	// Clean up old logs
	if err := dest.cleanupOldLogs(); err != nil {
		// Log warning but don't fail
		fmt.Fprintf(os.Stderr, "WARNING: Failed to cleanup old logs: %v\n", err)
	}

	return dest, nil
}

// Write writes an event and rotates if necessary.
func (d *RotatingFileDestination) Write(event Event) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if rotation is needed
	if d.shouldRotate() {
		if err := d.rotate(); err != nil {
			return fmt.Errorf("failed to rotate log: %w", err)
		}
	}

	// Format event
	var line []byte
	var err error

	switch d.format {
	case "json":
		line, err = event.MarshalJSON()
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

	// Write to file
	n, err := d.file.Write(line)
	if err != nil {
		return err
	}

	d.currentSize += int64(n)
	return nil
}

// Close closes the current log file.
func (d *RotatingFileDestination) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// shouldRotate checks if the log should be rotated.
func (d *RotatingFileDestination) shouldRotate() bool {
	// Check size limit
	if d.currentSize >= d.maxSize {
		return true
	}

	// Check daily rotation
	if d.rotateDaily {
		currentDate := time.Now().Format("2006-01-02")
		if currentDate != d.currentDate {
			return true
		}
	}

	return false
}

// rotate closes the current file and opens a new one.
func (d *RotatingFileDestination) rotate() error {
	// Close current file
	if d.file != nil {
		if err := d.file.Close(); err != nil {
			return fmt.Errorf("failed to close current log: %w", err)
		}
	}

	// Generate rotated filename
	timestamp := time.Now().Format("2006-01-02-150405")
	ext := filepath.Ext(d.basePath)
	base := strings.TrimSuffix(d.basePath, ext)
	rotatedPath := fmt.Sprintf("%s.%s%s", base, timestamp, ext)

	// Rename current file to rotated name
	if err := os.Rename(d.currentPath, rotatedPath); err != nil {
		// File might not exist yet, that's okay
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to rename log file: %w", err)
		}
	} else {
		// Compress rotated file if configured
		if d.compress {
			if err := d.compressFile(rotatedPath); err != nil {
				// Log warning but don't fail rotation
				fmt.Fprintf(os.Stderr, "WARNING: Failed to compress rotated log: %v\n", err)
			}
		}
	}

	// Open new file
	if err := d.openFile(); err != nil {
		return err
	}

	// Update current date
	d.currentDate = time.Now().Format("2006-01-02")

	// Cleanup old logs
	if err := d.cleanupOldLogs(); err != nil {
		// Log warning but don't fail rotation
		fmt.Fprintf(os.Stderr, "WARNING: Failed to cleanup old logs: %v\n", err)
	}

	return nil
}

// openFile opens the current log file.
func (d *RotatingFileDestination) openFile() error {
	file, err := os.OpenFile(d.currentPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	d.file = file
	d.currentSize = info.Size()

	return nil
}

// compressFile compresses a rotated log file with gzip.
func (d *RotatingFileDestination) compressFile(path string) error {
	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create compressed file
	dstPath := path + ".gz"
	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create compressed file: %w", err)
	}
	defer dst.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(dst)
	defer gzWriter.Close()

	// Copy data
	if _, err := io.Copy(gzWriter, src); err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}

	// Close gzip writer to flush
	if err := gzWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize compression: %w", err)
	}

	// Close destination file
	if err := dst.Close(); err != nil {
		return fmt.Errorf("failed to close compressed file: %w", err)
	}

	// Remove original file
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove uncompressed file: %w", err)
	}

	return nil
}

// cleanupOldLogs removes logs older than maxAge.
func (d *RotatingFileDestination) cleanupOldLogs() error {
	dir := filepath.Dir(d.basePath)
	base := filepath.Base(d.basePath)

	// Find all rotated log files
	pattern := strings.TrimSuffix(base, filepath.Ext(base)) + ".*"
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return fmt.Errorf("failed to find rotated logs: %w", err)
	}

	cutoff := time.Now().Add(-d.maxAge)

	for _, match := range matches {
		// Skip current log file
		if match == d.currentPath {
			continue
		}

		// Get file info
		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		// Check if file is too old
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(match); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Failed to remove old log %s: %v\n", match, err)
			}
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler for Event.
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return []byte(fmt.Sprintf(`{"timestamp":"%s","event_type":"%s","workflow_id":"%s","step_id":"%s","tool_name":"%s","resource":"%s","resource_type":"%s","action":"%s","decision":"%s","reason":"%s","profile":"%s","user_id":"%s"}`,
		e.Timestamp.Format(time.RFC3339),
		e.EventType,
		e.WorkflowID,
		e.StepID,
		e.ToolName,
		e.Resource,
		e.ResourceType,
		e.Action,
		e.Decision,
		e.Reason,
		e.Profile,
		e.UserID,
	)), nil
}

// ListRotatedLogs returns information about rotated log files.
func ListRotatedLogs(basePath string) ([]RotatedLogInfo, error) {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)

	// Find all rotated log files
	pattern := strings.TrimSuffix(base, filepath.Ext(base)) + ".*"
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to find rotated logs: %w", err)
	}

	var logs []RotatedLogInfo
	for _, match := range matches {
		// Skip current log file
		if match == basePath {
			continue
		}

		info, err := os.Stat(match)
		if err != nil {
			continue
		}

		logs = append(logs, RotatedLogInfo{
			Path:      match,
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsGzipped: strings.HasSuffix(match, ".gz"),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].ModTime.After(logs[j].ModTime)
	})

	return logs, nil
}

// RotatedLogInfo contains information about a rotated log file.
type RotatedLogInfo struct {
	Path      string
	Size      int64
	ModTime   time.Time
	IsGzipped bool
}
