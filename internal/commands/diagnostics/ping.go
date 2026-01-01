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
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

var (
	pingProvider string
)

// PingResult contains the ping health check result
type PingResult struct {
	Provider      string `json:"provider"`
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

// NewPingCommand creates the ping command
func NewPingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "ping [provider]",
		Annotations: map[string]string{
			"group": "diagnostics",
		},
		Short: "Quick health check for LLM provider",
		Long: `Test connectivity and authentication with an LLM provider.

By default, tests the default provider from config.
You can specify a provider name to test a specific configured provider.

This performs a lightweight three-step check:
  1. Installed - Provider is available
  2. Authenticated - Provider has valid credentials
  3. Working - Provider can communicate with backend

Exit codes:
  0 - Provider is healthy
  1 - Provider has issues`,
		Args: cobra.MaximumNArgs(1),
		RunE: runPing,
	}

	cmd.Flags().StringVar(&pingProvider, "provider", "", "Provider name to test (defaults to default_provider)")

	return cmd
}

func runPing(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Determine which provider to test
	providerName := pingProvider
	if len(args) > 0 {
		providerName = args[0]
	}

	// Load config
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		var err error
		cfgPath, err = config.ConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine config path: %w", err)
		}
	}

	// Check if config exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// No config - try Claude Code CLI as fallback
		if providerName == "" || providerName == "claude-code" {
			return pingClaudeCodeFallback(ctx)
		}
		return fmt.Errorf("no configuration found. Run 'conductor init' to set up")
	}

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine provider to test
	if providerName == "" {
		providerName = cfg.DefaultProvider
		if providerName == "" {
			return fmt.Errorf("no default provider configured. Use --provider to specify one")
		}
	}

	// Get provider config
	providerCfg, exists := cfg.Providers[providerName]
	if !exists {
		return fmt.Errorf("provider %q not found in config. Available: %v", providerName, keysOf(cfg.Providers))
	}

	// Test the provider
	result := pingProviderConfig(ctx, providerName, providerCfg)

	// Output results
	if shared.GetJSON() {
		return outputPingJSON(result)
	}
	return outputPingText(result)
}

// pingProviderConfig tests a configured provider
func pingProviderConfig(ctx context.Context, name string, cfg config.ProviderConfig) PingResult {
	result := PingResult{
		Provider: name,
		Type:     cfg.Type,
	}

	// Create provider based on type
	var provider llm.Provider
	switch cfg.Type {
	case "claude-code":
		provider = claudecode.New()
	default:
		result.Error = fmt.Sprintf("Unknown provider type: %s", cfg.Type)
		result.ErrorStep = "installed"
		return result
	}

	// Check if provider supports health checks
	healthChecker, ok := provider.(llm.HealthCheckable)
	if !ok {
		// Provider doesn't support health checks
		result.Error = "Provider type does not support health checks"
		result.ErrorStep = "installed"
		return result
	}

	// Run health check
	checkResult := healthChecker.HealthCheck(ctx)
	result.Installed = checkResult.Installed
	result.Authenticated = checkResult.Authenticated
	result.Working = checkResult.Working
	result.Healthy = checkResult.Healthy()
	result.Version = checkResult.Version

	if checkResult.Error != nil {
		result.Error = checkResult.Error.Error()
		result.ErrorStep = string(checkResult.ErrorStep)
	}

	result.Message = checkResult.Message

	return result
}

// pingClaudeCodeFallback tests Claude Code CLI when no config exists
func pingClaudeCodeFallback(ctx context.Context) error {
	provider := claudecode.New()
	checkResult := provider.HealthCheck(ctx)

	result := PingResult{
		Provider:      "claude-code",
		Type:          "claude-code",
		Installed:     checkResult.Installed,
		Authenticated: checkResult.Authenticated,
		Working:       checkResult.Working,
		Healthy:       checkResult.Healthy(),
		Version:       checkResult.Version,
	}

	if checkResult.Error != nil {
		result.Error = checkResult.Error.Error()
		result.ErrorStep = string(checkResult.ErrorStep)
	}

	result.Message = checkResult.Message

	if shared.GetJSON() {
		return outputPingJSON(result)
	}
	return outputPingText(result)
}

// outputPingJSON outputs ping result in JSON format
func outputPingJSON(result PingResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return err
	}

	if !result.Healthy {
		os.Exit(1)
	}
	return nil
}

// outputPingText outputs ping result in human-readable format
func outputPingText(result PingResult) error {
	if !shared.GetQuiet() {
		fmt.Printf("Testing provider: %s (%s)\n", result.Provider, result.Type)
		fmt.Println()

		fmt.Printf("  Installed:     %s\n", checkMark(result.Installed))
		fmt.Printf("  Authenticated: %s\n", checkMark(result.Authenticated))
		fmt.Printf("  Working:       %s\n", checkMark(result.Working))

		if result.Version != "" {
			fmt.Printf("  Version:       %s\n", result.Version)
		}

		fmt.Println()

		if result.Healthy {
			fmt.Println("Status: Healthy")
		} else {
			fmt.Println("Status: Failed")
			if result.Error != "" {
				fmt.Printf("Error: %s\n", result.Error)
			}
			if result.Message != "" {
				fmt.Println()
				fmt.Println(result.Message)
			}
		}
	}

	if !result.Healthy {
		os.Exit(1)
	}

	return nil
}
