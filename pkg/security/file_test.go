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

package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeterminePermissions(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		expectedFile os.FileMode
		expectedDir  os.FileMode
		description  string
	}{
		// Config patterns
		{
			name:         "config.yaml",
			path:         "/etc/conductor/config.yaml",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "config pattern should get 0600/0700",
		},
		{
			name:         "CONFIG.json",
			path:         "/tmp/CONFIG.json",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "case insensitive config pattern",
		},
		{
			name:         "my-config-file.yml",
			path:         "/var/my-config-file.yml",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "substring config matching",
		},
		{
			name:         "settings.ini",
			path:         "settings.ini",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "settings pattern",
		},
		{
			name:         "app.conf",
			path:         "/etc/app.conf",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "conf pattern",
		},
		{
			name:         "database.cfg",
			path:         "database.cfg",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  ".cfg extension",
		},

		// Secret patterns
		{
			name:         "secrets.json",
			path:         "/var/secrets.json",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "secret pattern",
		},
		{
			name:         "my-secret-key.txt",
			path:         "/tmp/my-secret-key.txt",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "substring secret matching",
		},
		{
			name:         "credentials.json",
			path:         "credentials.json",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "credential pattern",
		},
		{
			name:         "PASSWORD.txt",
			path:         "PASSWORD.txt",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "case insensitive password",
		},
		{
			name:         "auth.token",
			path:         "/var/auth.token",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "auth pattern",
		},

		// Key patterns
		{
			name:         "encryption_key.pem",
			path:         "/etc/certs/encryption_key.pem",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "key pattern with .pem",
		},
		{
			name:         "private.key",
			path:         "private.key",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "private pattern",
		},
		{
			name:         "keystore.p12",
			path:         "/var/keystore.p12",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  ".p12 extension",
		},
		{
			name:         "truststore.jks",
			path:         "truststore.jks",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  ".jks extension",
		},

		// Environment files
		{
			name:         ".env",
			path:         "/app/.env",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  ".env file",
		},
		{
			name:         ".env.production",
			path:         ".env.production",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  ".env with suffix",
		},

		// Token patterns
		{
			name:         "api_token.txt",
			path:         "/var/api_token.txt",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "token pattern",
		},
		{
			name:         "bearer_token",
			path:         "bearer_token",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "bearer pattern",
		},
		{
			name:         "api_key.json",
			path:         "/etc/api_key.json",
			expectedFile: 0600,
			expectedDir:  0700,
			description:  "api_key pattern",
		},

		// Non-matching files (should get 0640/0750)
		{
			name:         "output.txt",
			path:         "/tmp/output.txt",
			expectedFile: 0640,
			expectedDir:  0750,
			description:  "general file should get 0640/0750",
		},
		{
			name:         "data.json",
			path:         "/var/data.json",
			expectedFile: 0640,
			expectedDir:  0750,
			description:  "general json file",
		},
		{
			name:         "report.csv",
			path:         "report.csv",
			expectedFile: 0640,
			expectedDir:  0750,
			description:  "general csv file",
		},
		{
			name:         "log.txt",
			path:         "/var/log/log.txt",
			expectedFile: 0640,
			expectedDir:  0750,
			description:  "log file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileMode, dirMode := DeterminePermissions(tt.path)

			if fileMode != tt.expectedFile {
				t.Errorf("DeterminePermissions(%q) file mode = %o, want %o (%s)",
					tt.path, fileMode, tt.expectedFile, tt.description)
			}

			if dirMode != tt.expectedDir {
				t.Errorf("DeterminePermissions(%q) dir mode = %o, want %o (%s)",
					tt.path, dirMode, tt.expectedDir, tt.description)
			}
		})
	}
}

func TestVerifyPermissions(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		// Create a file with specific permissions
		testFile := filepath.Join(tmpDir, "test-verify.txt")
		f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		defer f.Close()

		// Verify permissions match
		if err := VerifyPermissions(f, 0600); err != nil {
			t.Errorf("VerifyPermissions() unexpected error: %v", err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		// Create a file with 0640 permissions
		testFile := filepath.Join(tmpDir, "test-mismatch.txt")
		f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0640)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		defer f.Close()

		// Verify should fail when expecting different permissions
		err = VerifyPermissions(f, 0600)
		if err == nil {
			t.Error("VerifyPermissions() expected error for permission mismatch, got nil")
		}

		// Check that error message contains expected and actual permissions
		expectedMsg := "permissions mismatch"
		if err != nil && !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("VerifyPermissions() error = %v, want error containing %q", err, expectedMsg)
		}
	})

	t.Run("invalid file descriptor", func(t *testing.T) {
		// Create and close a file to get an invalid descriptor
		testFile := filepath.Join(tmpDir, "test-closed.txt")
		f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		f.Close() // Close it to make the descriptor invalid

		// Verify should fail on stat
		err = VerifyPermissions(f, 0600)
		if err == nil {
			t.Error("VerifyPermissions() expected error for closed file, got nil")
		}
	})
}

func TestWriteFileAtomic_TempFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	config := DefaultFileSecurityConfig()
	// Allow writes to temp directory
	config.AllowedWritePaths = []string{tmpDir}

	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")

	// Write file with 0640 permissions (final permissions)
	err := config.WriteFileAtomic(testFile, content, 0640)
	if err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}

	// Verify final file has correct permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0640 {
		t.Errorf("final file permissions = %o, want %o", info.Mode().Perm(), 0640)
	}

	// Read content to verify file was written correctly
	readContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("file content = %q, want %q", readContent, content)
	}
}

func TestCheckConfigPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("non-existent path", func(t *testing.T) {
		// Non-existent paths should not generate warnings
		warnings := CheckConfigPermissions(filepath.Join(tmpDir, "nonexistent"))
		if len(warnings) != 0 {
			t.Errorf("CheckConfigPermissions() for non-existent path returned %d warnings, want 0", len(warnings))
		}
	})

	t.Run("secure file permissions", func(t *testing.T) {
		// Create a file with secure permissions (0600)
		testFile := filepath.Join(tmpDir, "secure-config.yaml")
		if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		warnings := CheckConfigPermissions(testFile)
		if len(warnings) != 0 {
			t.Errorf("CheckConfigPermissions() for secure file returned warnings: %v", warnings)
		}
	})

	t.Run("world-readable file", func(t *testing.T) {
		// Create a file with world-readable permissions (0644)
		testFile := filepath.Join(tmpDir, "world-readable.yaml")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		warnings := CheckConfigPermissions(testFile)
		if len(warnings) == 0 {
			t.Error("CheckConfigPermissions() expected warnings for world-readable file, got none")
		}

		// Check that warning mentions world-readable
		foundWarning := false
		for _, w := range warnings {
			if strings.Contains(w, "world-readable") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("CheckConfigPermissions() warnings did not mention world-readable: %v", warnings)
		}
	})

	t.Run("world-writable file", func(t *testing.T) {
		// Create a file with world-writable permissions (0666)
		testFile := filepath.Join(tmpDir, "world-writable.yaml")
		if err := os.WriteFile(testFile, []byte("test"), 0666); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Check actual permissions (umask might affect them)
		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		actualPerm := info.Mode().Perm()
		t.Logf("Requested 0666, actual permissions: %o", actualPerm)

		warnings := CheckConfigPermissions(testFile)
		if len(warnings) == 0 {
			t.Error("CheckConfigPermissions() expected warnings for world-writable file, got none")
		}

		t.Logf("Got %d warnings: %v", len(warnings), warnings)

		// Should have at least one warning (might not have world-writable if umask blocked it)
		if len(warnings) < 1 {
			t.Errorf("CheckConfigPermissions() got %d warnings, want at least 1 for 0666 file", len(warnings))
		}
	})

	t.Run("group-writable sensitive file", func(t *testing.T) {
		// Create a sensitive file with group-writable permissions (0620)
		testFile := filepath.Join(tmpDir, "secret.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		// Use chmod to ensure exact permissions (bypass umask)
		if err := os.Chmod(testFile, 0620); err != nil {
			t.Fatalf("failed to chmod file: %v", err)
		}

		warnings := CheckConfigPermissions(testFile)
		if len(warnings) == 0 {
			t.Error("CheckConfigPermissions() expected warnings for group-writable sensitive file, got none")
		}

		// Check that warning mentions group-writable
		foundWarning := false
		for _, w := range warnings {
			if strings.Contains(w, "group-writable") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("CheckConfigPermissions() warnings did not mention group-writable: %v", warnings)
		}
	})

	t.Run("secure directory permissions", func(t *testing.T) {
		// Create a directory with secure permissions (0700)
		testDir := filepath.Join(tmpDir, "secure-dir")
		if err := os.Mkdir(testDir, 0700); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		warnings := CheckConfigPermissions(testDir)
		if len(warnings) != 0 {
			t.Errorf("CheckConfigPermissions() for secure directory returned warnings: %v", warnings)
		}
	})

	t.Run("world-readable directory", func(t *testing.T) {
		// Create a directory with world-readable permissions (0755)
		testDir := filepath.Join(tmpDir, "world-readable-dir")
		if err := os.Mkdir(testDir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}

		warnings := CheckConfigPermissions(testDir)
		if len(warnings) == 0 {
			t.Error("CheckConfigPermissions() expected warnings for world-readable directory, got none")
		}

		// Check that warning mentions directory
		foundWarning := false
		for _, w := range warnings {
			if strings.Contains(w, "directory") && strings.Contains(w, "world-readable") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("CheckConfigPermissions() warnings did not mention world-readable directory: %v", warnings)
		}
	})

	t.Run("group-writable directory", func(t *testing.T) {
		// Create a directory with group-writable permissions (0770)
		testDir := filepath.Join(tmpDir, "group-writable-dir")
		if err := os.Mkdir(testDir, 0700); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
		// Use chmod to ensure exact permissions (bypass umask)
		if err := os.Chmod(testDir, 0770); err != nil {
			t.Fatalf("failed to chmod directory: %v", err)
		}

		warnings := CheckConfigPermissions(testDir)
		if len(warnings) == 0 {
			t.Error("CheckConfigPermissions() expected warnings for group-writable directory, got none")
		}

		// Check that warning mentions group-writable
		foundWarning := false
		for _, w := range warnings {
			if strings.Contains(w, "group-writable") {
				foundWarning = true
				break
			}
		}
		if !foundWarning {
			t.Errorf("CheckConfigPermissions() warnings did not mention group-writable: %v", warnings)
		}
	})

	t.Run("world-writable directory", func(t *testing.T) {
		// Create a directory with world-writable permissions (0777)
		testDir := filepath.Join(tmpDir, "world-writable-dir")
		if err := os.Mkdir(testDir, 0700); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
		// Use chmod to ensure exact permissions (bypass umask)
		if err := os.Chmod(testDir, 0777); err != nil {
			t.Fatalf("failed to chmod directory: %v", err)
		}

		warnings := CheckConfigPermissions(testDir)
		if len(warnings) < 2 {
			t.Errorf("CheckConfigPermissions() got %d warnings for 0777 directory, want at least 2", len(warnings))
		}
	})
}
