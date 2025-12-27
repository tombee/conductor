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

package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// DoctorResult contains the overall health check results
type DoctorResult struct {
	ConfigPath         string                    `json:"config_path"`
	ConfigExists       bool                      `json:"config_exists"`
	ConfigValid        bool                      `json:"config_valid"`
	ConfigError        string                    `json:"config_error,omitempty"`
	DefaultProvider    string                    `json:"default_provider"`
	ProviderResults    map[string]ProviderHealth `json:"provider_results"`
	Recommendations    []string                  `json:"recommendations"`
	OverallHealthy     bool                      `json:"overall_healthy"`
}

// ProviderHealth contains health check results for a single provider
type ProviderHealth struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Installed     bool   `json:"installed"`
	Authenticated bool   `json:"authenticated"`
	Working       bool   `json:"working"`
	Healthy       bool   `json:"healthy"`
	Error         string `json:"error,omitempty"`
	ErrorStep     string `json:"error_step,omitempty"`
	Message       string `json:"message,omitempty"`
	Version       string `json:"version,omitempty"`
}

// NewDoctorCommand creates the doctor command
func NewDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Annotations: map[string]string{
			"group": "diagnostics",
		},
		Short: "Check system health and configuration",
		Long: `Perform a comprehensive health check of Conductor configuration and providers.

This command checks:
  - Config file exists and is valid
  - Default provider is configured
  - All providers are healthy and can connect
  - Common configuration issues

Provides actionable recommendations for fixing any issues found.`,
		RunE: runDoctor,
	}

	return cmd
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result := DoctorResult{
		ProviderResults: make(map[string]ProviderHealth),
		Recommendations: []string{},
		OverallHealthy:  true,
	}

	// Step 1: Check config file
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		// Use default path
		var err error
		cfgPath, err = config.ConfigPath()
		if err != nil {
			result.ConfigPath = "unknown"
			result.ConfigError = fmt.Sprintf("Failed to determine config path: %v", err)
			result.OverallHealthy = false
		} else {
			result.ConfigPath = cfgPath
		}
	} else {
		result.ConfigPath = cfgPath
	}

	// Check if config file exists
	if _, err := os.Stat(result.ConfigPath); err == nil {
		result.ConfigExists = true
	} else if os.IsNotExist(err) {
		result.ConfigExists = false
		result.OverallHealthy = false
		result.Recommendations = append(result.Recommendations,
			"No configuration file found. Run 'conductor init' to create one.")
	} else {
		result.ConfigError = fmt.Sprintf("Failed to check config file: %v", err)
		result.OverallHealthy = false
	}

	// Step 2: Try to load and validate config
	if result.ConfigExists {
		cfg, err := config.Load(result.ConfigPath)
		if err != nil {
			result.ConfigValid = false
			result.ConfigError = fmt.Sprintf("Config validation failed: %v", err)
			result.OverallHealthy = false
			result.Recommendations = append(result.Recommendations,
				"Fix configuration errors or run 'conductor init --force' to recreate config.")
		} else {
			result.ConfigValid = true
			result.DefaultProvider = cfg.DefaultProvider

			// Check if default provider is set
			if cfg.DefaultProvider == "" {
				result.OverallHealthy = false
				result.Recommendations = append(result.Recommendations,
					"No default provider configured. Add 'default_provider' to config or run 'conductor init'.")
			}

			// Step 3: Test all configured providers
			if len(cfg.Providers) == 0 {
				result.OverallHealthy = false
				result.Recommendations = append(result.Recommendations,
					"No providers configured. Run 'conductor init' to set up a provider.")
			} else {
				// Test each provider
				for name, providerCfg := range cfg.Providers {
					health := testProvider(ctx, name, providerCfg)
					result.ProviderResults[name] = health

					if !health.Healthy {
						result.OverallHealthy = false
						if health.Message != "" {
							result.Recommendations = append(result.Recommendations,
								fmt.Sprintf("Provider '%s': %s", name, health.Message))
						}
					}
				}
			}
		}
	}

	// Step 4: Check for Claude Code CLI as fallback if no config
	if !result.ConfigExists || (result.ConfigValid && len(result.ProviderResults) == 0) {
		provider := claudecode.New()
		health := testClaudeCodeProvider(ctx, provider)

		if health.Installed {
			result.Recommendations = append(result.Recommendations,
				"Claude Code CLI detected. Run 'conductor init' to configure it.")
		} else {
			result.Recommendations = append(result.Recommendations,
				"Install Claude Code CLI for easy setup: https://claude.ai/download")
		}
	}

	// Output results
	if shared.GetJSON() {
		return outputDoctorJSON(result)
	}
	return outputDoctorText(result)
}

// testProvider tests a configured provider's health
func testProvider(ctx context.Context, name string, cfg config.ProviderConfig) ProviderHealth {
	health := ProviderHealth{
		Name:    name,
		Type:    cfg.Type,
		Healthy: false,
	}

	// Create provider based on type
	var provider llm.Provider
	switch cfg.Type {
	case "claude-code":
		provider = claudecode.New()
		if cfg.ConfigPath != "" {
			// TODO: Support custom config path when provider supports it
		}
	default:
		health.Error = fmt.Sprintf("Unknown provider type: %s", cfg.Type)
		health.ErrorStep = "installed"
		return health
	}

	// Check if provider supports health checks
	healthChecker, ok := provider.(llm.HealthCheckable)
	if !ok {
		// Provider doesn't support health checks, assume it's working
		health.Installed = true
		health.Authenticated = true
		health.Working = true
		health.Healthy = true
		health.Message = "Provider type does not support health checks"
		return health
	}

	// Run health check
	checkResult := healthChecker.HealthCheck(ctx)
	health.Installed = checkResult.Installed
	health.Authenticated = checkResult.Authenticated
	health.Working = checkResult.Working
	health.Healthy = checkResult.Healthy()
	health.Version = checkResult.Version

	if checkResult.Error != nil {
		health.Error = checkResult.Error.Error()
		health.ErrorStep = string(checkResult.ErrorStep)
	}

	health.Message = checkResult.Message

	return health
}

// testClaudeCodeProvider tests Claude Code CLI availability
func testClaudeCodeProvider(ctx context.Context, provider *claudecode.Provider) ProviderHealth {
	health := ProviderHealth{
		Name:    "claude-code",
		Type:    "claude-code",
		Healthy: false,
	}

	checkResult := provider.HealthCheck(ctx)
	health.Installed = checkResult.Installed
	health.Authenticated = checkResult.Authenticated
	health.Working = checkResult.Working
	health.Healthy = checkResult.Healthy()
	health.Version = checkResult.Version

	if checkResult.Error != nil {
		health.Error = checkResult.Error.Error()
		health.ErrorStep = string(checkResult.ErrorStep)
	}

	health.Message = checkResult.Message

	return health
}

// outputDoctorJSON outputs results in JSON format
func outputDoctorJSON(result DoctorResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// outputDoctorText outputs results in human-readable format
func outputDoctorText(result DoctorResult) error {
	fmt.Println("Conductor Health Check")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Config status
	fmt.Println("Configuration:")
	fmt.Printf("  Path: %s\n", result.ConfigPath)

	if result.ConfigExists {
		fmt.Println("  Status: Found")
		if result.ConfigValid {
			fmt.Println("  Valid: Yes")
			if result.DefaultProvider != "" {
				fmt.Printf("  Default Provider: %s\n", result.DefaultProvider)
			} else {
				fmt.Println("  Default Provider: Not set")
			}
		} else {
			fmt.Println("  Valid: No")
			if result.ConfigError != "" {
				fmt.Printf("  Error: %s\n", result.ConfigError)
			}
		}
	} else {
		fmt.Println("  Status: Not found")
	}
	fmt.Println()

	// Provider health
	if len(result.ProviderResults) > 0 {
		fmt.Println("Provider Health:")
		for name, health := range result.ProviderResults {
			status := "OK"
			if !health.Healthy {
				status = "FAILED"
			}

			fmt.Printf("  %s (%s): [%s]\n", name, health.Type, status)
			fmt.Printf("    Installed: %s\n", checkMark(health.Installed))
			fmt.Printf("    Authenticated: %s\n", checkMark(health.Authenticated))
			fmt.Printf("    Working: %s\n", checkMark(health.Working))

			if health.Version != "" {
				fmt.Printf("    Version: %s\n", health.Version)
			}

			if health.Error != "" {
				fmt.Printf("    Error: %s\n", health.Error)
			}

			if !health.Healthy && health.Message != "" {
				// Message includes newlines, indent each line
				lines := strings.Split(health.Message, "\n")
				for _, line := range lines {
					if line != "" {
						fmt.Printf("    %s\n", line)
					}
				}
			}
			fmt.Println()
		}
	}

	// Recommendations
	if len(result.Recommendations) > 0 {
		fmt.Println("Recommendations:")
		for _, rec := range result.Recommendations {
			fmt.Printf("  - %s\n", rec)
		}
		fmt.Println()
	}

	// Overall status
	if result.OverallHealthy {
		fmt.Println("Overall Status: Healthy")
		return nil
	} else {
		fmt.Println("Overall Status: Issues Found")
		return fmt.Errorf("health check found issues")
	}
}
