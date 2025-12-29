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

package endpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/controller/auth"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary workflow directory
	tmpDir := t.TempDir()

	// Create test workflow files
	testWorkflow := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testWorkflow, []byte("name: test\nsteps: []"), 0644); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	anotherWorkflow := filepath.Join(tmpDir, "another.yml")
	if err := os.WriteFile(anotherWorkflow, []byte("name: another\nsteps: []"), 0644); err != nil {
		t.Fatalf("Failed to create another workflow: %v", err)
	}

	tests := []struct {
		name        string
		cfg         config.EndpointsConfig
		workflowDir string
		wantCount   int
		wantErr     bool
		errSubstr   string
	}{
		{
			name: "disabled config returns empty registry",
			cfg: config.EndpointsConfig{
				Enabled: false,
				Endpoints: []config.EndpointEntry{
					{Name: "test", Workflow: "test.yaml"},
				},
			},
			workflowDir: tmpDir,
			wantCount:   0,
			wantErr:     false,
		},
		{
			name: "valid single endpoint",
			cfg: config.EndpointsConfig{
				Enabled: true,
				Endpoints: []config.EndpointEntry{
					{
						Name:        "test-endpoint",
						Description: "Test endpoint",
						Workflow:    "test",
						Inputs: map[string]any{
							"key": "value",
						},
						Scopes:    []string{"scope1"},
						RateLimit: "100/hour",
						Timeout:   5 * time.Minute,
					},
				},
			},
			workflowDir: tmpDir,
			wantCount:   1,
			wantErr:     false,
		},
		{
			name: "multiple endpoints",
			cfg: config.EndpointsConfig{
				Enabled: true,
				Endpoints: []config.EndpointEntry{
					{Name: "ep1", Workflow: "test"},
					{Name: "ep2", Workflow: "another"},
				},
			},
			workflowDir: tmpDir,
			wantCount:   2,
			wantErr:     false,
		},
		{
			name: "workflow not found",
			cfg: config.EndpointsConfig{
				Enabled: true,
				Endpoints: []config.EndpointEntry{
					{Name: "missing", Workflow: "nonexistent.yaml"},
				},
			},
			workflowDir: tmpDir,
			wantErr:     true,
			errSubstr:   "not found",
		},
		{
			name: "duplicate endpoint names",
			cfg: config.EndpointsConfig{
				Enabled: true,
				Endpoints: []config.EndpointEntry{
					{Name: "duplicate", Workflow: "test"},
					{Name: "duplicate", Workflow: "another"},
				},
			},
			workflowDir: tmpDir,
			wantErr:     true,
			errSubstr:   "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, err := LoadConfig(tt.cfg, tt.workflowDir, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("LoadConfig() error = %v, want substring %q", err, tt.errSubstr)
				}
				return
			}
			if registry == nil {
				t.Fatal("LoadConfig() returned nil registry")
			}
			if registry.Count() != tt.wantCount {
				t.Errorf("LoadConfig() registry count = %d, want %d", registry.Count(), tt.wantCount)
			}
		})
	}
}

func TestLoadConfig_WithRateLimiter(t *testing.T) {
	// Create temporary workflow directory
	tmpDir := t.TempDir()

	// Create test workflow file
	testWorkflow := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testWorkflow, []byte("name: test\nsteps: []"), 0644); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	rateLimiter := auth.NewNamedRateLimiter()

	cfg := config.EndpointsConfig{
		Enabled: true,
		Endpoints: []config.EndpointEntry{
			{
				Name:      "test-endpoint",
				Workflow:  "test",
				RateLimit: "100/hour",
			},
		},
	}

	registry, err := LoadConfig(cfg, tmpDir, rateLimiter)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("LoadConfig() registry count = %d, want 1", registry.Count())
	}

	// Verify rate limiter was configured
	allowed := rateLimiter.Allow("test-endpoint")
	if !allowed {
		t.Error("Rate limiter should allow initial request")
	}

	// Verify limit exists
	_, _, _, exists := rateLimiter.GetStatus("test-endpoint")
	if !exists {
		t.Error("Rate limit should be configured for test-endpoint")
	}
}

func TestLoadConfig_InvalidRateLimit(t *testing.T) {
	// Create temporary workflow directory
	tmpDir := t.TempDir()

	// Create test workflow file
	testWorkflow := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testWorkflow, []byte("name: test\nsteps: []"), 0644); err != nil {
		t.Fatalf("Failed to create test workflow: %v", err)
	}

	rateLimiter := auth.NewNamedRateLimiter()

	cfg := config.EndpointsConfig{
		Enabled: true,
		Endpoints: []config.EndpointEntry{
			{
				Name:      "test-endpoint",
				Workflow:  "test",
				RateLimit: "invalid-rate-limit",
			},
		},
	}

	_, err := LoadConfig(cfg, tmpDir, rateLimiter)
	if err == nil {
		t.Error("LoadConfig() should fail with invalid rate limit")
	}
	if !strings.Contains(err.Error(), "invalid rate limit") {
		t.Errorf("LoadConfig() error = %v, want 'invalid rate limit'", err)
	}
}

func TestFindWorkflow(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("Failed to create workflows dir: %v", err)
	}

	// Create test files
	files := map[string]string{
		"test.yaml":            "name: test",
		"another.yml":          "name: another",
		"noext":                "name: noext",
		"subdir/nested.yaml":   "name: nested",
	}

	for path, content := range files {
		fullPath := filepath.Join(workflowsDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", path, err)
		}
	}

	tests := []struct {
		name         string
		workflowName string
		workflowsDir string
		wantErr      bool
		wantPath     string
	}{
		{
			name:         "find .yaml file",
			workflowName: "test",
			workflowsDir: workflowsDir,
			wantErr:      false,
			wantPath:     filepath.Join(workflowsDir, "test.yaml"),
		},
		{
			name:         "find .yml file",
			workflowName: "another",
			workflowsDir: workflowsDir,
			wantErr:      false,
			wantPath:     filepath.Join(workflowsDir, "another.yml"),
		},
		{
			name:         "find file without extension",
			workflowName: "noext",
			workflowsDir: workflowsDir,
			wantErr:      false,
			wantPath:     filepath.Join(workflowsDir, "noext"),
		},
		{
			name:         "find nested file",
			workflowName: "subdir/nested",
			workflowsDir: workflowsDir,
			wantErr:      false,
			wantPath:     filepath.Join(workflowsDir, "subdir/nested.yaml"),
		},
		{
			name:         "file not found",
			workflowName: "nonexistent",
			workflowsDir: workflowsDir,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := findWorkflow(tt.workflowName, tt.workflowsDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("findWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotPath != tt.wantPath {
				t.Errorf("findWorkflow() path = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		rateLimit     string
		wantCount     int
		wantDuration  time.Duration
		wantErr       bool
		errSubstr     string
	}{
		{
			name:         "valid per second",
			rateLimit:    "10/second",
			wantCount:    10,
			wantDuration: time.Second,
			wantErr:      false,
		},
		{
			name:         "valid per minute",
			rateLimit:    "100/minute",
			wantCount:    100,
			wantDuration: time.Minute,
			wantErr:      false,
		},
		{
			name:         "valid per hour",
			rateLimit:    "1000/hour",
			wantCount:    1000,
			wantDuration: time.Hour,
			wantErr:      false,
		},
		{
			name:         "valid per day",
			rateLimit:    "10000/day",
			wantCount:    10000,
			wantDuration: 24 * time.Hour,
			wantErr:      false,
		},
		{
			name:      "empty string",
			rateLimit: "",
			wantErr:   true,
			errSubstr: "empty",
		},
		{
			name:      "invalid format no slash",
			rateLimit: "100",
			wantErr:   true,
			errSubstr: "invalid rate limit format",
		},
		{
			name:      "invalid format too many parts",
			rateLimit: "100/minute/extra",
			wantErr:   true,
			errSubstr: "invalid rate limit format",
		},
		{
			name:      "invalid count non-numeric",
			rateLimit: "abc/minute",
			wantErr:   true,
			errSubstr: "invalid rate limit count",
		},
		{
			name:      "invalid count zero",
			rateLimit: "0/minute",
			wantErr:   true,
			errSubstr: "invalid rate limit count",
		},
		{
			name:      "invalid count negative",
			rateLimit: "-10/minute",
			wantErr:   true,
			errSubstr: "invalid rate limit count",
		},
		{
			name:      "invalid unit",
			rateLimit: "100/week",
			wantErr:   true,
			errSubstr: "invalid rate limit unit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, duration, err := ParseRateLimit(tt.rateLimit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRateLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errSubstr != "" {
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("ParseRateLimit() error = %v, want substring %q", err, tt.errSubstr)
				}
				return
			}
			if !tt.wantErr {
				if count != tt.wantCount {
					t.Errorf("ParseRateLimit() count = %d, want %d", count, tt.wantCount)
				}
				if duration != tt.wantDuration {
					t.Errorf("ParseRateLimit() duration = %v, want %v", duration, tt.wantDuration)
				}
			}
		})
	}
}
