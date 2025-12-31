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
	"strings"
	"testing"
	"time"
)

func TestWriteConfig(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create a minimal config
	cfg := &Config{
		DefaultProvider: "test",
		Providers: ProvidersMap{
			"test": ProviderConfig{
				Type: "anthropic",
			},
		},
	}

	// Write config
	if err := WriteConfig(cfg, configPath); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Verify file permissions (0600)
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "# Conductor Configuration") {
		t.Error("Config file missing header comment")
	}
	if !strings.Contains(content, "default_provider: test") {
		t.Error("Config file missing default_provider")
	}
	if !strings.Contains(content, "type: anthropic") {
		t.Error("Config file missing provider type")
	}
}

func TestWriteConfigBackup(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create initial config
	cfg1 := &Config{
		DefaultProvider: "old",
		Providers: ProvidersMap{
			"old": ProviderConfig{Type: "anthropic"},
		},
	}

	if err := WriteConfig(cfg1, configPath); err != nil {
		t.Fatalf("First WriteConfig failed: %v", err)
	}

	// Wait a moment to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Write updated config
	cfg2 := &Config{
		DefaultProvider: "new",
		Providers: ProvidersMap{
			"new": ProviderConfig{Type: "openai-compatible"},
		},
	}

	if err := WriteConfig(cfg2, configPath); err != nil {
		t.Fatalf("Second WriteConfig failed: %v", err)
	}

	// Verify backup was created
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	backupFound := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config.yaml.bak.") {
			backupFound = true

			// Verify backup contains old config
			backupPath := filepath.Join(tmpDir, entry.Name())
			data, err := os.ReadFile(backupPath)
			if err != nil {
				t.Fatalf("Failed to read backup file: %v", err)
			}
			if !strings.Contains(string(data), "default_provider: old") {
				t.Error("Backup doesn't contain old config")
			}
		}
	}

	if !backupFound {
		t.Error("No backup file was created")
	}

	// Verify current config has new data
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read current config: %v", err)
	}
	if !strings.Contains(string(data), "default_provider: new") {
		t.Error("Current config doesn't contain new data")
	}
}

func TestRotateBackups(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create initial config
	cfg := &Config{
		DefaultProvider: "test",
		Providers:       ProvidersMap{"test": ProviderConfig{Type: "anthropic"}},
	}

	// Write config 6 times to generate backups
	// First write creates no backup (no existing file)
	// Subsequent writes create backups
	for i := 0; i < 6; i++ {
		cfg.DefaultProvider = "test" + string(rune('0'+i))
		if err := WriteConfig(cfg, configPath); err != nil {
			t.Fatalf("WriteConfig %d failed: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Count backup files
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	backupCount := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config.yaml.bak.") {
			backupCount++
		}
	}

	// Should have exactly 3 backups (oldest ones rotated out)
	// Write 1: no backup (file doesn't exist)
	// Write 2: 1 backup
	// Write 3: 2 backups
	// Write 4: 3 backups
	// Write 5: 3 backups (oldest rotated)
	// Write 6: 3 backups (oldest rotated)
	if backupCount != 3 {
		t.Errorf("Expected 3 backup files, got %d", backupCount)
	}
}

func TestWriteAtomicFailureCleanup(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Try to write to a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0500); err != nil {
		t.Fatalf("Failed to create read-only dir: %v", err)
	}
	defer os.Chmod(readOnlyDir, 0700) // Restore permissions for cleanup

	configPath := filepath.Join(readOnlyDir, "config.yaml")

	// This should fail due to permissions
	err := writeAtomic(configPath, []byte("test"), 0600)
	if err == nil {
		t.Error("Expected writeAtomic to fail with read-only directory")
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(readOnlyDir)
	if err != nil && !os.IsPermission(err) {
		t.Fatalf("Failed to read directory: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Errorf("Found leftover temp file: %s", entry.Name())
		}
	}
}

func TestWriteConfigExpandsHomedir(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Mock home directory for test
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tmpDir)

	// Write to ~/config.yaml
	cfg := &Config{
		DefaultProvider: "test",
		Providers:       ProvidersMap{"test": ProviderConfig{Type: "anthropic"}},
	}

	configPath := "~/config.yaml"
	if err := WriteConfig(cfg, configPath); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Verify file was created in home directory
	expectedPath := filepath.Join(tmpDir, "config.yaml")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Config file not created at expected path: %s", expectedPath)
	}
}
