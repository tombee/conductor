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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSettingsFile_LockUnlock(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.yaml")

	sf, err := NewSettingsFile(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsFile() error = %v", err)
	}

	// Test lock acquisition
	if err := sf.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Test unlock
	if err := sf.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
}

func TestSettingsFile_ConcurrentAccess(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.yaml")

	// Create two SettingsFile instances to simulate concurrent processes
	sf1, err := NewSettingsFile(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsFile() sf1 error = %v", err)
	}

	sf2, err := NewSettingsFile(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsFile() sf2 error = %v", err)
	}

	// First process acquires lock
	if err := sf1.Lock(); err != nil {
		t.Fatalf("sf1.Lock() error = %v", err)
	}
	defer sf1.Unlock()

	// Second process should timeout trying to acquire lock
	errChan := make(chan error, 1)
	go func() {
		errChan <- sf2.Lock()
	}()

	// Wait for timeout (should be ~5 seconds)
	select {
	case err := <-errChan:
		if err != ErrLockTimeout {
			t.Errorf("Expected ErrLockTimeout, got %v", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("Lock timeout did not occur within expected time")
	}
}

func TestSettingsFile_SaveLoad(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.yaml")

	sf, err := NewSettingsFile(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsFile() error = %v", err)
	}

	// Create test config
	testCfg := &Config{
		Version: 1,
		Providers: ProvidersMap{
			"test-provider": ProviderConfig{
				Type: "anthropic",
				Models: map[string]ModelConfig{
					"test-model": {
						ContextWindow: 200000,
					},
				},
			},
		},
		Tiers: map[string]string{
			"fast": "test-provider/test-model",
		},
	}

	// Test save
	err = sf.WithLock(func() error {
		return sf.Save(testCfg)
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Fatal("Settings file was not created")
	}

	// Test load
	var loadedCfg *Config
	err = sf.WithLock(func() error {
		var loadErr error
		loadedCfg, loadErr = sf.Load()
		return loadErr
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded config matches saved config
	if loadedCfg.Version != testCfg.Version {
		t.Errorf("Version mismatch: got %d, want %d", loadedCfg.Version, testCfg.Version)
	}

	if len(loadedCfg.Providers) != len(testCfg.Providers) {
		t.Errorf("Providers count mismatch: got %d, want %d", len(loadedCfg.Providers), len(testCfg.Providers))
	}

	if len(loadedCfg.Tiers) != len(testCfg.Tiers) {
		t.Errorf("Tiers count mismatch: got %d, want %d", len(loadedCfg.Tiers), len(testCfg.Tiers))
	}
}

func TestSettingsFile_AtomicWrite(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.yaml")

	sf, err := NewSettingsFile(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsFile() error = %v", err)
	}

	// Write initial config
	initialCfg := &Config{
		Version: 1,
		Providers: ProvidersMap{
			"initial": ProviderConfig{Type: "anthropic"},
		},
	}

	err = sf.WithLock(func() error {
		return sf.Save(initialCfg)
	})
	if err != nil {
		t.Fatalf("Initial Save() error = %v", err)
	}

	// Simulate concurrent writes
	var wg sync.WaitGroup
	errors := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		providerName := "provider" + string(rune('A'+i))
		go func(name string) {
			defer wg.Done()

			sf2, err := NewSettingsFile(settingsPath)
			if err != nil {
				errors <- err
				return
			}

			cfg := &Config{
				Version: 1,
				Providers: ProvidersMap{
					name: ProviderConfig{Type: "anthropic"},
				},
			}

			err = sf2.WithLock(func() error {
				return sf2.Save(cfg)
			})
			if err != nil {
				errors <- err
			}
		}(providerName)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent write error: %v", err)
		}
	}

	// Verify final state is valid (one of the writes succeeded)
	finalCfg, err := LoadSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}

	if finalCfg.Version != 1 {
		t.Errorf("Final config version = %d, want 1", finalCfg.Version)
	}

	if len(finalCfg.Providers) != 1 {
		t.Errorf("Final config should have 1 provider, got %d", len(finalCfg.Providers))
	}
}

func TestLoadSettings_NonExistent(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "nonexistent.yaml")

	cfg, err := LoadSettings(settingsPath)
	if err != nil {
		t.Fatalf("LoadSettings() on non-existent file should not error, got %v", err)
	}

	// Should return default config with version 1
	if cfg.Version != 1 {
		t.Errorf("Default config version = %d, want 1", cfg.Version)
	}
}

func TestSaveSettings_CreatesDirectory(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "subdir", "settings.yaml")

	testCfg := &Config{
		Version: 1,
		Providers: ProvidersMap{
			"test": ProviderConfig{Type: "anthropic"},
		},
	}

	err := SaveSettings(settingsPath, testCfg)
	if err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(settingsPath)); os.IsNotExist(err) {
		t.Fatal("Directory was not created")
	}

	// Verify file was created
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Fatal("Settings file was not created")
	}

	// Verify file permissions are secure (0600)
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("File permissions = %o, want 0600", mode)
	}
}
