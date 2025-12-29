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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/controller/auth"
)

// LoadConfig loads endpoints from configuration into a registry.
// It validates that workflow files exist and parses rate limit strings.
// If a rateLimiter is provided, it configures rate limits for each endpoint.
func LoadConfig(cfg config.EndpointsConfig, workflowsDir string, rateLimiter *auth.NamedRateLimiter) (*Registry, error) {
	registry := NewRegistry()

	if !cfg.Enabled {
		return registry, nil
	}

	for i, entry := range cfg.Endpoints {
		// Convert config entry to endpoint
		ep := &Endpoint{
			Name:        entry.Name,
			Description: entry.Description,
			Workflow:    entry.Workflow,
			Inputs:      entry.Inputs,
			Scopes:      entry.Scopes,
			RateLimit:   entry.RateLimit,
			Timeout:     entry.Timeout,
			Public:      entry.Public,
		}

		// Validate workflow file exists
		workflowPath, err := findWorkflow(entry.Workflow, workflowsDir)
		if err != nil {
			return nil, fmt.Errorf("endpoint %d (%s): workflow %q not found: %w", i, entry.Name, entry.Workflow, err)
		}

		// Verify file is readable
		if _, err := os.Stat(workflowPath); err != nil {
			return nil, fmt.Errorf("endpoint %d (%s): workflow %q not accessible: %w", i, entry.Name, entry.Workflow, err)
		}

		// Configure rate limit if specified
		if rateLimiter != nil && entry.RateLimit != "" {
			if err := rateLimiter.AddLimit(entry.Name, entry.RateLimit); err != nil {
				return nil, fmt.Errorf("endpoint %d (%s): invalid rate limit %q: %w", i, entry.Name, entry.RateLimit, err)
			}
		}

		// Add to registry
		if err := registry.Add(ep); err != nil {
			return nil, fmt.Errorf("endpoint %d (%s): %w", i, entry.Name, err)
		}
	}

	return registry, nil
}

// findWorkflow locates a workflow file by name.
// It searches in the workflows directory with various extensions (.yaml, .yml, or no extension).
// Returns the full path if found, error otherwise.
func findWorkflow(name string, workflowsDir string) (string, error) {
	// Search in workflows directory and current directory
	extensions := []string{".yaml", ".yml", ""}
	baseDirs := []string{workflowsDir, "."}

	// If the name is an absolute path, try it directly with extensions
	if filepath.IsAbs(name) {
		for _, ext := range extensions {
			path := name + ext
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
		// If name already has extension, try exact match
		if filepath.Ext(name) != "" {
			if info, err := os.Stat(name); err == nil && !info.IsDir() {
				return name, nil
			}
		}
		return "", fmt.Errorf("workflow file not found: %s", name)
	}

	// For relative paths (including those with subdirectories), search in base dirs
	for _, baseDir := range baseDirs {
		if baseDir == "" {
			continue
		}
		for _, ext := range extensions {
			path := filepath.Join(baseDir, name+ext)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("workflow file not found: %s (searched in %s and current directory)", name, workflowsDir)
}

// ParseRateLimit parses a rate limit string (e.g., "100/hour") into tokens and duration.
// Returns (tokens per period, duration) or error if invalid format.
func ParseRateLimit(rateLimit string) (int, time.Duration, error) {
	if rateLimit == "" {
		return 0, 0, fmt.Errorf("empty rate limit")
	}

	parts := strings.Split(rateLimit, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid rate limit format %q, expected <count>/<unit>", rateLimit)
	}

	// Parse count
	count, err := strconv.Atoi(parts[0])
	if err != nil || count <= 0 {
		return 0, 0, fmt.Errorf("invalid rate limit count %q, must be positive integer", parts[0])
	}

	// Parse unit to duration
	unit := parts[1]
	var duration time.Duration
	switch unit {
	case "second":
		duration = time.Second
	case "minute":
		duration = time.Minute
	case "hour":
		duration = time.Hour
	case "day":
		duration = 24 * time.Hour
	default:
		return 0, 0, fmt.Errorf("invalid rate limit unit %q, must be one of: second, minute, hour, day", unit)
	}

	return count, duration, nil
}
