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

package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateServerName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myserver", false},
		{"valid with hyphen", "my-server", false},
		{"valid with underscore", "my_server", false},
		{"valid with numbers", "server123", false},
		{"valid mixed", "my-server_v2", false},
		{"empty", "", true},
		{"starts with number", "123server", true},
		{"starts with hyphen", "-server", true},
		{"starts with underscore", "_server", true},
		{"contains space", "my server", true},
		{"contains dot", "my.server", true},
		{"too long", "a" + strings.Repeat("b", 64), true},
		{"max length", "a" + strings.Repeat("b", 63), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServerName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServerName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateArg(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple arg", "value", false},
		{"path arg", "/path/to/file", false},
		{"flag arg", "--verbose", false},
		{"contains semicolon", "cmd;rm -rf", true},
		{"contains pipe", "cmd|cat", true},
		{"contains and", "cmd&&echo", true},
		{"contains or", "cmd||echo", true},
		{"contains backtick", "cmd`echo`", true},
		{"contains subshell", "$(rm -rf)", true},
		{"contains var expansion", "${HOME}", true},
		{"contains newline", "cmd\nrm", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArg(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArg(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEnv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "KEY=value", false},
		{"with underscore", "MY_KEY=value", false},
		{"empty value", "KEY=", false},
		{"with var substitution", "KEY=${OTHER}", false},
		{"no equals", "KEY", true},
		{"empty key", "=value", true},
		{"key starts with number", "1KEY=value", true},
		{"key contains hyphen", "MY-KEY=value", true},
		{"value contains semicolon", "KEY=value;cmd", true},
		{"value contains pipe", "KEY=value|cmd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnv(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnv(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestIsSensitiveEnvKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"GITHUB_TOKEN", true},
		{"API_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"DATABASE_PASSWORD", true},
		{"AUTH_TOKEN", true},
		{"MY_CREDENTIAL", true},
		{"HOSTNAME", false},
		{"PORT", false},
		{"DEBUG", false},
		{"LOG_LEVEL", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsSensitiveEnvKey(tt.key); got != tt.expected {
				t.Errorf("IsSensitiveEnvKey(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestRedactEnv(t *testing.T) {
	input := []string{
		"GITHUB_TOKEN=secret123",
		"PORT=8080",
		"API_KEY=mykey",
		"DEBUG=true",
	}

	expected := []string{
		"GITHUB_TOKEN=***REDACTED***",
		"PORT=8080",
		"API_KEY=***REDACTED***",
		"DEBUG=true",
	}

	result := RedactEnv(input)

	if len(result) != len(expected) {
		t.Fatalf("RedactEnv() returned %d items, want %d", len(result), len(expected))
	}

	for i, want := range expected {
		if result[i] != want {
			t.Errorf("RedactEnv()[%d] = %q, want %q", i, result[i], want)
		}
	}
}

func TestMCPServerEntryValidate(t *testing.T) {
	tests := []struct {
		name    string
		entry   MCPServerEntry
		wantErr bool
	}{
		{
			name: "valid with command",
			entry: MCPServerEntry{
				Command: "echo", // Should be in PATH
				Args:    []string{"hello"},
			},
			wantErr: false,
		},
		{
			name: "missing command and source",
			entry: MCPServerEntry{
				Args: []string{"hello"},
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			entry: MCPServerEntry{
				Command: "echo",
				Timeout: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid restart policy",
			entry: MCPServerEntry{
				Command:       "echo",
				RestartPolicy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid restart policies",
			entry: MCPServerEntry{
				Command:       "echo",
				RestartPolicy: RestartAlways,
			},
			wantErr: false,
		},
		{
			name: "invalid arg",
			entry: MCPServerEntry{
				Command: "echo",
				Args:    []string{"$(rm -rf)"},
			},
			wantErr: true,
		},
		{
			name: "invalid env",
			entry: MCPServerEntry{
				Command: "echo",
				Env:     []string{"INVALID"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadSaveMCPConfig(t *testing.T) {
	// Create a temp directory for testing
	tmpDir := t.TempDir()
	oldHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", oldHome)

	// Test loading non-existent config (should return defaults)
	cfg, err := LoadMCPConfig()
	if err != nil {
		t.Fatalf("LoadMCPConfig() error = %v", err)
	}
	if cfg.Servers == nil {
		t.Error("LoadMCPConfig() Servers should not be nil")
	}
	if cfg.Defaults.Timeout != 30 {
		t.Errorf("LoadMCPConfig() Defaults.Timeout = %d, want 30", cfg.Defaults.Timeout)
	}

	// Add a server and save
	cfg.Servers["test"] = &MCPServerEntry{
		Command:   "echo",
		Args:      []string{"hello"},
		AutoStart: true,
	}

	if err := SaveMCPConfig(cfg); err != nil {
		t.Fatalf("SaveMCPConfig() error = %v", err)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, "conductor", "mcp.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("SaveMCPConfig() did not create file at %s", path)
	}

	// Load again and verify
	cfg2, err := LoadMCPConfig()
	if err != nil {
		t.Fatalf("LoadMCPConfig() after save error = %v", err)
	}

	if _, ok := cfg2.Servers["test"]; !ok {
		t.Error("LoadMCPConfig() did not preserve server 'test'")
	}

	entry := cfg2.Servers["test"]
	if entry.Command != "echo" {
		t.Errorf("Server command = %q, want %q", entry.Command, "echo")
	}
	if !entry.AutoStart {
		t.Error("Server AutoStart should be true")
	}
}

func TestMCPGlobalConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  MCPGlobalConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: MCPGlobalConfig{
				Servers: map[string]*MCPServerEntry{
					"valid": {Command: "echo"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid server name",
			config: MCPGlobalConfig{
				Servers: map[string]*MCPServerEntry{
					"invalid name": {Command: "echo"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid server entry",
			config: MCPGlobalConfig{
				Servers: map[string]*MCPServerEntry{
					"valid": {}, // Missing command and source
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToServerConfig(t *testing.T) {
	entry := &MCPServerEntry{
		Command: "python",
		Args:    []string{"server.py"},
		Env:     []string{"DEBUG=true"},
		Timeout: 60,
	}

	cfg := entry.ToServerConfig("myserver")

	if cfg.Name != "myserver" {
		t.Errorf("Name = %q, want %q", cfg.Name, "myserver")
	}
	if cfg.Command != "python" {
		t.Errorf("Command = %q, want %q", cfg.Command, "python")
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "server.py" {
		t.Errorf("Args = %v, want [server.py]", cfg.Args)
	}
	if cfg.Timeout.Seconds() != 60 {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
}
