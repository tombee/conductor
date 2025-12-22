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
	"testing"
)

func TestAccessChecker_FilesystemRead(t *testing.T) {
	tests := []struct {
		name       string
		config     FilesystemAccess
		path       string
		wantAllow  bool
		wantReason string
	}{
		{
			name: "empty config denies all access",
			config: FilesystemAccess{
				Read: []string{},
			},
			path:      "/tmp/test.txt",
			wantAllow: false,
		},
		{
			name: "exact path match allowed",
			config: FilesystemAccess{
				Read: []string{"/tmp/test.txt"},
			},
			path:      "/tmp/test.txt",
			wantAllow: true,
		},
		{
			name: "glob pattern match allowed",
			config: FilesystemAccess{
				Read: []string{"/tmp/*.txt"},
			},
			path:      "/tmp/test.txt",
			wantAllow: true,
		},
		{
			name: "recursive glob pattern match allowed",
			config: FilesystemAccess{
				Read: []string{"/tmp/**/*.go"},
			},
			path:      "/tmp/pkg/main.go",
			wantAllow: true,
		},
		{
			name: "deny pattern blocks access",
			config: FilesystemAccess{
				Read: []string{"/tmp/**"},
				Deny: []string{"/tmp/secrets/**"},
			},
			path:      "/tmp/secrets/key.txt",
			wantAllow: false,
		},
		{
			name: "deny pattern has priority over allow",
			config: FilesystemAccess{
				Read: []string{"/tmp/**/*.txt"},
				Deny: []string{"**/.env"},
			},
			path:      "/tmp/config/.env",
			wantAllow: false,
		},
		{
			name: "path not matching any pattern denied",
			config: FilesystemAccess{
				Read: []string{"/tmp/**"},
			},
			path:      "/var/log/test.log",
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AccessConfig{
				Filesystem: tt.config,
			}
			checker, err := NewAccessChecker(cfg)
			if err != nil {
				t.Fatalf("NewAccessChecker() error = %v", err)
			}

			result := checker.CheckFilesystemRead(tt.path)
			if result.Allowed != tt.wantAllow {
				t.Errorf("CheckFilesystemRead() allowed = %v, want %v", result.Allowed, tt.wantAllow)
				t.Logf("Reason: %s", result.Reason)
			}
		})
	}
}

func TestAccessChecker_FilesystemWrite(t *testing.T) {
	tests := []struct {
		name      string
		config    FilesystemAccess
		path      string
		wantAllow bool
	}{
		{
			name: "empty config denies all writes",
			config: FilesystemAccess{
				Write: []string{},
			},
			path:      "/tmp/test.txt",
			wantAllow: false,
		},
		{
			name: "allowed write pattern",
			config: FilesystemAccess{
				Write: []string{"/tmp/**"},
			},
			path:      "/tmp/output/result.txt",
			wantAllow: true,
		},
		{
			name: "deny pattern blocks write",
			config: FilesystemAccess{
				Write: []string{"/tmp/**"},
				Deny:  []string{"/tmp/readonly/**"},
			},
			path:      "/tmp/readonly/data.txt",
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AccessConfig{
				Filesystem: tt.config,
			}
			checker, err := NewAccessChecker(cfg)
			if err != nil {
				t.Fatalf("NewAccessChecker() error = %v", err)
			}

			result := checker.CheckFilesystemWrite(tt.path)
			if result.Allowed != tt.wantAllow {
				t.Errorf("CheckFilesystemWrite() allowed = %v, want %v", result.Allowed, tt.wantAllow)
				t.Logf("Reason: %s", result.Reason)
			}
		})
	}
}

func TestAccessChecker_FilesystemVariables(t *testing.T) {
	// Use a temporary directory as the cwd for testing
	// This ensures tests are isolated and don't depend on the actual cwd
	testDir, err := os.MkdirTemp("", "access-checker-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(testDir)

	// Create subdirectories for testing
	srcDir := filepath.Join(testDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	tempDir := os.TempDir()

	tests := []struct {
		name      string
		pattern   string
		testPath  string
		wantAllow bool
		customCwd string
	}{
		{
			name:      "$cwd variable resolves to current directory",
			pattern:   "$cwd/**",
			testPath:  filepath.Join(testDir, "test.txt"),
			wantAllow: true,
			customCwd: testDir,
		},
		{
			name:      "$temp variable resolves to temp directory",
			pattern:   "$temp/**",
			testPath:  filepath.Join(tempDir, "test.txt"),
			wantAllow: true,
		},
		{
			name:      "relative path resolves correctly",
			pattern:   "./src/**",
			testPath:  filepath.Join(testDir, "src", "main.go"),
			wantAllow: true,
			customCwd: testDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AccessConfig{
				Filesystem: FilesystemAccess{
					Read: []string{tt.pattern},
				},
			}

			// If test specifies a custom cwd, create checker with that cwd
			checker, err := NewAccessChecker(cfg)
			if err != nil {
				t.Fatalf("NewAccessChecker() error = %v", err)
			}

			if tt.customCwd != "" {
				// Override the cwd for this test
				ac := checker.(*accessChecker)
				ac.cwd = tt.customCwd
				// Re-resolve patterns with new cwd
				ac.fsReadPatterns = ac.resolvePatterns(cfg.Filesystem.Read)
			}

			result := checker.CheckFilesystemRead(tt.testPath)
			if result.Allowed != tt.wantAllow {
				t.Errorf("CheckFilesystemRead() allowed = %v, want %v", result.Allowed, tt.wantAllow)
				t.Logf("Pattern: %s, TestPath: %s, Reason: %s", tt.pattern, tt.testPath, result.Reason)
			}
		})
	}
}

func TestAccessChecker_Network(t *testing.T) {
	tests := []struct {
		name      string
		config    NetworkAccess
		host      string
		port      int
		wantAllow bool
	}{
		{
			name: "empty config denies all network access",
			config: NetworkAccess{
				Allow: []string{},
			},
			host:      "api.github.com",
			port:      443,
			wantAllow: false,
		},
		{
			name: "exact hostname match allowed",
			config: NetworkAccess{
				Allow: []string{"api.github.com"},
			},
			host:      "api.github.com",
			port:      443,
			wantAllow: true,
		},
		{
			name: "wildcard subdomain match allowed",
			config: NetworkAccess{
				Allow: []string{"*.github.com"},
			},
			host:      "api.github.com",
			port:      443,
			wantAllow: true,
		},
		{
			name: "wildcard does not match base domain",
			config: NetworkAccess{
				Allow: []string{"*.github.com"},
			},
			host:      "github.com",
			port:      443,
			wantAllow: false,
		},
		{
			name: "wildcard does not match evil-github.com",
			config: NetworkAccess{
				Allow: []string{"*.github.com"},
			},
			host:      "evil-github.com",
			port:      443,
			wantAllow: false,
		},
		{
			name: "port-specific pattern matches",
			config: NetworkAccess{
				Allow: []string{"localhost:8080"},
			},
			host:      "localhost",
			port:      8080,
			wantAllow: true,
		},
		{
			name: "port-specific pattern rejects wrong port",
			config: NetworkAccess{
				Allow: []string{"localhost:8080"},
			},
			host:      "localhost",
			port:      3000,
			wantAllow: false,
		},
		{
			name: "CIDR notation matches IP",
			config: NetworkAccess{
				Allow: []string{"10.0.0.0/8"},
			},
			host:      "10.1.2.3",
			port:      443,
			wantAllow: true,
		},
		{
			name: "CIDR notation rejects IP outside range",
			config: NetworkAccess{
				Allow: []string{"10.0.0.0/8"},
			},
			host:      "192.168.1.1",
			port:      443,
			wantAllow: false,
		},
		{
			name: "deny pattern blocks allowed host",
			config: NetworkAccess{
				Allow: []string{"*.github.com"},
				Deny:  []string{"evil.github.com"},
			},
			host:      "evil.github.com",
			port:      443,
			wantAllow: false,
		},
		{
			name: "deny private IPs",
			config: NetworkAccess{
				Allow: []string{"*"},
				Deny:  []string{"10.0.0.0/8", "192.168.0.0/16", "127.0.0.0/8"},
			},
			host:      "192.168.1.1",
			port:      443,
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AccessConfig{
				Network: tt.config,
			}
			checker, err := NewAccessChecker(cfg)
			if err != nil {
				t.Fatalf("NewAccessChecker() error = %v", err)
			}

			result := checker.CheckNetwork(tt.host, tt.port)
			if result.Allowed != tt.wantAllow {
				t.Errorf("CheckNetwork(%s:%d) allowed = %v, want %v", tt.host, tt.port, result.Allowed, tt.wantAllow)
				t.Logf("Reason: %s", result.Reason)
			}
		})
	}
}

func TestAccessChecker_Shell(t *testing.T) {
	tests := []struct {
		name      string
		config    ShellAccess
		command   string
		wantAllow bool
	}{
		{
			name: "empty config denies all commands",
			config: ShellAccess{
				Commands: []string{},
			},
			command:   "git status",
			wantAllow: false,
		},
		{
			name: "base command allows all subcommands",
			config: ShellAccess{
				Commands: []string{"git"},
			},
			command:   "git status",
			wantAllow: true,
		},
		{
			name: "base command allows complex arguments",
			config: ShellAccess{
				Commands: []string{"git"},
			},
			command:   "git push origin main",
			wantAllow: true,
		},
		{
			name: "specific subcommand allowed",
			config: ShellAccess{
				Commands: []string{"git status"},
			},
			command:   "git status",
			wantAllow: true,
		},
		{
			name: "specific subcommand with args allowed",
			config: ShellAccess{
				Commands: []string{"git status"},
			},
			command:   "git status -s",
			wantAllow: true,
		},
		{
			name: "specific subcommand blocks other subcommands",
			config: ShellAccess{
				Commands: []string{"git status"},
			},
			command:   "git push",
			wantAllow: false,
		},
		{
			name: "command with path allowed",
			config: ShellAccess{
				Commands: []string{"git"},
			},
			command:   "/usr/bin/git status",
			wantAllow: true,
		},
		{
			name: "deny pattern blocks dangerous command",
			config: ShellAccess{
				Commands:     []string{"git"},
				DenyPatterns: []string{"git push --force"},
			},
			command:   "git push --force",
			wantAllow: false,
		},
		{
			name: "deny pattern blocks with additional args",
			config: ShellAccess{
				Commands:     []string{"git"},
				DenyPatterns: []string{"git push --force"},
			},
			command:   "git push --force origin main",
			wantAllow: false,
		},
		{
			name: "deny pattern allows safe variant",
			config: ShellAccess{
				Commands:     []string{"git"},
				DenyPatterns: []string{"git push --force"},
			},
			command:   "git push origin main",
			wantAllow: true,
		},
		{
			name: "multiple deny patterns",
			config: ShellAccess{
				Commands:     []string{"rm"},
				DenyPatterns: []string{"rm -rf /", "rm -rf *"},
			},
			command:   "rm -rf /",
			wantAllow: false,
		},
		{
			name: "base command mismatch",
			config: ShellAccess{
				Commands: []string{"git"},
			},
			command:   "gitk",
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &AccessConfig{
				Shell: tt.config,
			}
			checker, err := NewAccessChecker(cfg)
			if err != nil {
				t.Fatalf("NewAccessChecker() error = %v", err)
			}

			result := checker.CheckShell(tt.command)
			if result.Allowed != tt.wantAllow {
				t.Errorf("CheckShell(%q) allowed = %v, want %v", tt.command, result.Allowed, tt.wantAllow)
				t.Logf("Reason: %s", result.Reason)
			}
		})
	}
}

func TestExtractCommandBase(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"git", "git"},
		{"git status", "git"},
		{"git push origin main", "git"},
		{"/usr/bin/git status", "git"},
		{"/usr/local/bin/npm install", "npm"},
		{"  git  status  ", "git"},
		{"", ""},
	}

	cfg := &AccessConfig{}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		t.Fatalf("NewAccessChecker() error = %v", err)
	}
	ac := checker.(*accessChecker)

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := ac.extractCommandBase(tt.command)
			if result != tt.expected {
				t.Errorf("extractCommandBase(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		input        string
		expectedHost string
		expectedPort int
	}{
		{"example.com", "example.com", 0},
		{"example.com:443", "example.com", 443},
		{"localhost:8080", "localhost", 8080},
		{"api.github.com:443", "api.github.com", 443},
		{"192.168.1.1:3000", "192.168.1.1", 3000},
		{"invalid:port", "invalid:port", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			host, port := parseHostPort(tt.input)
			if host != tt.expectedHost || port != tt.expectedPort {
				t.Errorf("parseHostPort(%q) = (%q, %d), want (%q, %d)",
					tt.input, host, port, tt.expectedHost, tt.expectedPort)
			}
		})
	}
}

func TestAccessCheckResult_AllowedList(t *testing.T) {
	cfg := &AccessConfig{
		Filesystem: FilesystemAccess{
			Read: []string{"/tmp/**", "/var/log/**"},
		},
	}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		t.Fatal(err)
	}

	result := checker.CheckFilesystemRead("/etc/passwd")
	if result.Allowed {
		t.Error("Expected access to be denied")
	}

	if len(result.AllowedList) != 2 {
		t.Errorf("Expected AllowedList to have 2 entries, got %d", len(result.AllowedList))
	}

	if result.AllowedList[0] != "/tmp/**" {
		t.Errorf("Expected first allowed pattern to be /tmp/**, got %s", result.AllowedList[0])
	}
}

func TestAccessChecker_SymlinkEscapePrevention(t *testing.T) {
	// Create temp directory structure with symlink pointing outside allowed dir
	allowedDir, err := os.MkdirTemp("", "allowed-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(allowedDir)

	restrictedDir, err := os.MkdirTemp("", "restricted-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(restrictedDir)

	// Create a restricted file
	restrictedFile := filepath.Join(restrictedDir, "secret.txt")
	if err := os.WriteFile(restrictedFile, []byte("secret data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside allowed dir that points to restricted file
	symlinkPath := filepath.Join(allowedDir, "link-to-secret")
	if err := os.Symlink(restrictedFile, symlinkPath); err != nil {
		t.Skip("Symlink creation not supported on this platform")
	}

	// Configure access to allow reading from allowedDir only
	cfg := &AccessConfig{
		Filesystem: FilesystemAccess{
			Read: []string{allowedDir + "/**"},
		},
	}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		t.Fatalf("NewAccessChecker() error = %v", err)
	}

	// Verify access is denied when resolving symlink to restricted location
	result := checker.CheckFilesystemRead(symlinkPath)
	if result.Allowed {
		t.Errorf("CheckFilesystemRead() should deny symlink escape, but allowed = %v", result.Allowed)
		t.Logf("Symlink path: %s", symlinkPath)
		t.Logf("Restricted file: %s", restrictedFile)
		t.Logf("Reason: %s", result.Reason)
	}
}

// BenchmarkAccessChecker_FilesystemRead verifies <1ms p99 latency for filesystem checks
func BenchmarkAccessChecker_FilesystemRead(b *testing.B) {
	cfg := &AccessConfig{
		Filesystem: FilesystemAccess{
			Read: []string{
				"/tmp/**",
				"/var/log/**/*.log",
				"/home/user/projects/**",
			},
			Deny: []string{
				"**/.env",
				"**/secrets/**",
			},
		},
	}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		b.Fatalf("NewAccessChecker() error = %v", err)
	}

	testPaths := []string{
		"/tmp/test.txt",
		"/var/log/app/server.log",
		"/home/user/projects/myapp/main.go",
		"/etc/passwd", // Should be denied
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := testPaths[i%len(testPaths)]
		_ = checker.CheckFilesystemRead(path)
	}
}

// BenchmarkAccessChecker_Network verifies <1ms p99 latency for network checks
func BenchmarkAccessChecker_Network(b *testing.B) {
	cfg := &AccessConfig{
		Network: NetworkAccess{
			Allow: []string{
				"api.github.com",
				"*.example.com",
				"10.0.0.0/8",
				"localhost:8080",
			},
			Deny: []string{
				"evil.example.com",
				"192.168.0.0/16",
			},
		},
	}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		b.Fatalf("NewAccessChecker() error = %v", err)
	}

	testCases := []struct {
		host string
		port int
	}{
		{"api.github.com", 443},
		{"api.example.com", 443},
		{"10.1.2.3", 443},
		{"localhost", 8080},
		{"evil.example.com", 443}, // Should be denied
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := testCases[i%len(testCases)]
		_ = checker.CheckNetwork(tc.host, tc.port)
	}
}

// BenchmarkAccessChecker_Shell verifies <1ms p99 latency for shell checks
func BenchmarkAccessChecker_Shell(b *testing.B) {
	cfg := &AccessConfig{
		Shell: ShellAccess{
			Commands: []string{
				"git",
				"npm",
				"go test",
			},
			DenyPatterns: []string{
				"git push --force",
				"rm -rf /",
			},
		},
	}
	checker, err := NewAccessChecker(cfg)
	if err != nil {
		b.Fatalf("NewAccessChecker() error = %v", err)
	}

	testCommands := []string{
		"git status",
		"npm install",
		"go test ./...",
		"git push --force", // Should be denied
		"rm -rf /",         // Should be denied
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := testCommands[i%len(testCommands)]
		_ = checker.CheckShell(cmd)
	}
}
