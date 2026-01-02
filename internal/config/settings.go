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

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// ErrLockTimeout is returned when file lock acquisition times out.
	ErrLockTimeout = errors.New("configuration locked by another process")
)

const (
	// lockTimeout is the maximum duration to wait for lock acquisition.
	lockTimeout = 5 * time.Second
)

// SettingsFile manages the settings.yaml file with file locking for concurrent access protection.
type SettingsFile struct {
	path     string
	lockFile *os.File
}

// SettingsPath returns the full path to the settings.yaml file.
func SettingsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.yaml"), nil
}

// NewSettingsFile creates a new SettingsFile instance for the given path.
// If path is empty, uses the default settings path.
func NewSettingsFile(path string) (*SettingsFile, error) {
	if path == "" {
		var err error
		path, err = SettingsPath()
		if err != nil {
			return nil, fmt.Errorf("failed to get settings path: %w", err)
		}
	}

	return &SettingsFile{
		path: path,
	}, nil
}

// Lock acquires an exclusive lock on the settings file.
// Returns ErrLockTimeout if the lock cannot be acquired within the timeout period.
func (s *SettingsFile) Lock() error {
	lockPath := s.path + ".lock"

	// Ensure the directory exists
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Open or create the lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire the lock with timeout
	deadline := time.Now().Add(lockTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Attempt to acquire exclusive lock
		err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Lock acquired
			s.lockFile = lockFile
			return nil
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			lockFile.Close()
			return ErrLockTimeout
		}

		// Wait before retrying
		<-ticker.C
	}
}

// Unlock releases the file lock.
func (s *SettingsFile) Unlock() error {
	if s.lockFile == nil {
		return nil
	}

	// Release the lock
	if err := syscall.Flock(int(s.lockFile.Fd()), syscall.LOCK_UN); err != nil {
		s.lockFile.Close()
		s.lockFile = nil
		return fmt.Errorf("failed to unlock: %w", err)
	}

	// Close the lock file
	if err := s.lockFile.Close(); err != nil {
		s.lockFile = nil
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	s.lockFile = nil
	return nil
}

// Load loads the configuration from the settings file.
// The file must be locked before calling this method.
func (s *SettingsFile) Load() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return default config
			cfg := Default()
			cfg.Version = 1
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse settings YAML: %w", err)
	}

	// Apply defaults to fill in any missing values
	cfg.applyDefaults()

	return &cfg, nil
}

// Save saves the configuration to the settings file using atomic writes.
// The file must be locked before calling this method.
func (s *SettingsFile) Save(cfg *Config) error {
	// Ensure the directory exists with secure permissions
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal the config to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to temporary file in the same directory (for atomic rename)
	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	// Atomically rename the temp file to the target file
	if err := os.Rename(tempPath, s.path); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temporary file: %w", err)
	}

	return nil
}

// WithLock executes a function while holding the file lock.
// The lock is automatically released when the function returns.
func (s *SettingsFile) WithLock(fn func() error) error {
	if err := s.Lock(); err != nil {
		return err
	}
	defer s.Unlock()

	return fn()
}

// LoadSettings is a convenience function that loads settings with automatic locking.
func LoadSettings(path string) (*Config, error) {
	sf, err := NewSettingsFile(path)
	if err != nil {
		return nil, err
	}

	var cfg *Config
	err = sf.WithLock(func() error {
		var loadErr error
		cfg, loadErr = sf.Load()
		return loadErr
	})
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveSettings is a convenience function that saves settings with automatic locking.
func SaveSettings(path string, cfg *Config) error {
	sf, err := NewSettingsFile(path)
	if err != nil {
		return err
	}

	return sf.WithLock(func() error {
		return sf.Save(cfg)
	})
}
