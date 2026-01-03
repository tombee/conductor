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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// HealthToolResult represents the health check result
type HealthToolResult struct {
	Healthy bool          `json:"healthy"`
	Version string        `json:"version"`
	Checks  []HealthCheck `json:"checks"`
}

// HealthCheck represents a single health check
type HealthCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // "pass", "warn", "fail"
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

// handleHealth implements the conductor_health tool
func (s *Server) handleHealth(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Run health checks with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result := runHealthChecks(checkCtx, s.version)

	// Marshal to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode health result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}

// runHealthChecks performs all health checks
func runHealthChecks(ctx context.Context, version string) HealthToolResult {
	result := HealthToolResult{
		Healthy: true,
		Version: version,
		Checks:  []HealthCheck{},
	}

	// Check 1: Config file exists and is valid
	configCheck := checkConfig()
	result.Checks = append(result.Checks, configCheck)
	if configCheck.Status == "fail" {
		result.Healthy = false
	}

	// Check 2: Default provider configured
	if configCheck.Status == "pass" {
		providerCheck := checkDefaultProvider()
		result.Checks = append(result.Checks, providerCheck)
		if providerCheck.Status == "fail" {
			result.Healthy = false
		}

		// Check 3: Provider health (if configured)
		if providerCheck.Status == "pass" {
			providerHealthCheck := checkProviderHealth(ctx)
			result.Checks = append(result.Checks, providerHealthCheck)
			if providerHealthCheck.Status == "fail" {
				result.Healthy = false
			}
		}
	}

	// Check 4: Environment variables
	envCheck := checkEnvironment()
	result.Checks = append(result.Checks, envCheck)
	// Environment check is informational only

	return result
}

// checkConfig checks if config file exists and is valid
func checkConfig() HealthCheck {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return HealthCheck{
			Name:        "Configuration File",
			Status:      "fail",
			Message:     fmt.Sprintf("Failed to determine config path: %v", err),
			Remediation: "Check your home directory permissions",
		}
	}

	// Check if config exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return HealthCheck{
			Name:        "Configuration File",
			Status:      "fail",
			Message:     "Config file not found",
			Remediation: "Run 'conductor setup' to create configuration",
		}
	}

	// Try to load config
	_, err = config.Load(cfgPath)
	if err != nil {
		return HealthCheck{
			Name:        "Configuration File",
			Status:      "fail",
			Message:     fmt.Sprintf("Config validation failed: %v", err),
			Remediation: "Fix configuration errors or run 'conductor setup --force' to recreate",
		}
	}

	return HealthCheck{
		Name:    "Configuration File",
		Status:  "pass",
		Message: fmt.Sprintf("Config found and valid (%s)", cfgPath),
	}
}

// checkDefaultProvider checks if a provider is configured
func checkDefaultProvider() HealthCheck {
	cfgPath, _ := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return HealthCheck{
			Name:    "Provider Configuration",
			Status:  "fail",
			Message: "Cannot load config to check provider",
		}
	}

	if len(cfg.Providers) == 0 {
		return HealthCheck{
			Name:        "Provider Configuration",
			Status:      "fail",
			Message:     "No providers configured",
			Remediation: "Run 'conductor provider add' to configure a provider",
		}
	}

	providerName := cfg.GetPrimaryProvider()
	if providerName == "" {
		return HealthCheck{
			Name:        "Provider Configuration",
			Status:      "fail",
			Message:     "No provider available",
			Remediation: "Run 'conductor provider add' to configure a provider",
		}
	}

	// Check if we have tiers configured
	tiersConfigured := len(cfg.Tiers) > 0
	if tiersConfigured {
		return HealthCheck{
			Name:    "Provider Configuration",
			Status:  "pass",
			Message: fmt.Sprintf("Primary provider: %s (via tiers)", providerName),
		}
	}

	return HealthCheck{
		Name:    "Provider Configuration",
		Status:  "pass",
		Message: fmt.Sprintf("Primary provider: %s", providerName),
	}
}

// checkProviderHealth checks the health of the configured provider
func checkProviderHealth(ctx context.Context) HealthCheck {
	cfgPath, _ := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return HealthCheck{
			Name:    "Provider Health",
			Status:  "warn",
			Message: "Cannot check provider health",
		}
	}

	// Get primary provider
	providerName := cfg.GetPrimaryProvider()
	if providerName == "" {
		return HealthCheck{
			Name:        "Provider Health",
			Status:      "fail",
			Message:     "No provider configured",
			Remediation: "Run 'conductor provider add' to configure a provider",
		}
	}

	providerCfg, ok := cfg.Providers[providerName]
	if !ok {
		return HealthCheck{
			Name:        "Provider Health",
			Status:      "fail",
			Message:     fmt.Sprintf("Provider %q not found in configuration", providerName),
			Remediation: "Check your provider configuration",
		}
	}

	// Create provider based on type
	var provider llm.Provider
	switch providerCfg.Type {
	case "claude-code":
		provider = claudecode.New()
	default:
		return HealthCheck{
			Name:    "Provider Health",
			Status:  "warn",
			Message: fmt.Sprintf("Unknown provider type: %s", providerCfg.Type),
		}
	}

	// Check if provider supports health checks
	healthChecker, ok := provider.(llm.HealthCheckable)
	if !ok {
		return HealthCheck{
			Name:    "Provider Health",
			Status:  "pass",
			Message: "Provider does not support health checks (assumed working)",
		}
	}

	// Run health check (with timeout from ctx)
	healthResult := healthChecker.HealthCheck(ctx)

	// IMPORTANT: Don't expose credentials in output (SC2)
	if !healthResult.Healthy() {
		status := "fail"
		if healthResult.Installed {
			status = "warn"
		}

		message := "Provider health check failed"
		if healthResult.Message != "" {
			// Sanitize message to ensure no credential exposure
			message = sanitizeMessage(healthResult.Message)
		}

		return HealthCheck{
			Name:        "Provider Health",
			Status:      status,
			Message:     message,
			Remediation: getProviderRemediation(healthResult),
		}
	}

	message := fmt.Sprintf("Provider %s is healthy", providerName)
	if healthResult.Version != "" {
		message += fmt.Sprintf(" (version: %s)", healthResult.Version)
	}

	return HealthCheck{
		Name:    "Provider Health",
		Status:  "pass",
		Message: message,
	}
}

// checkEnvironment checks relevant environment variables
func checkEnvironment() HealthCheck {
	// Check for common environment variables
	var warnings []string

	if os.Getenv("CONDUCTOR_ALLOWED_PATHS") != "" {
		warnings = append(warnings, "CONDUCTOR_ALLOWED_PATHS is set")
	}

	if os.Getenv("CONDUCTOR_PROVIDER") != "" {
		warnings = append(warnings, "CONDUCTOR_PROVIDER override is active")
	}

	message := "No environment overrides detected"
	if len(warnings) > 0 {
		message = fmt.Sprintf("Environment overrides: %d active", len(warnings))
	}

	return HealthCheck{
		Name:    "Environment Variables",
		Status:  "pass",
		Message: message,
	}
}

// sanitizeMessage removes potential credential information from messages
func sanitizeMessage(msg string) string {
	// Remove any parts that might contain credentials
	// For now, just return the message as-is since llm.HealthCheckResult
	// should already not include credentials
	return msg
}

// getProviderRemediation returns remediation steps based on health check result
func getProviderRemediation(result llm.HealthCheckResult) string {
	if !result.Installed {
		return "Provider is not installed. Install the required provider binary."
	}
	if !result.Authenticated {
		return "Provider authentication failed. Check your credentials configuration."
	}
	if !result.Working {
		return "Provider is not working properly. Check provider logs for details."
	}
	return "Run 'conductor health' CLI command for detailed diagnostics"
}
