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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"gopkg.in/yaml.v3"
)

// ValidationResult represents the result of config validation.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// NewValidateCommand creates the 'config validate' subcommand.
func NewValidateCommand() *cobra.Command {
	var strict bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long: `Validate the configuration file structure and references.

Checks performed:
  - YAML syntax and structure
  - Provider configurations are valid
  - Model registrations are complete
  - Tier mappings reference existing providers and models
  - No orphaned tier references

With --strict, warnings are treated as errors.`,
		Example: `  # Validate configuration
  conductor config validate

  # Validate with warnings as errors
  conductor config validate --strict

  # Get validation result as JSON
  conductor config validate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(strict)
		},
	}

	cmd.Flags().BoolVar(&strict, "strict", false, "Treat warnings as errors")

	return cmd
}

// runValidate performs configuration validation.
func runValidate(strict bool) error {
	// Get config path (settings.yaml)
	settingsPath, err := config.SettingsPath()
	if err != nil {
		return fmt.Errorf("failed to determine settings path: %w", err)
	}

	// Check if settings file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		result := ValidationResult{
			Valid:    false,
			Errors:   []string{"No settings file found. Run 'conductor provider add' to configure providers."},
			Warnings: nil,
		}
		return outputValidationResult(result, strict)
	}

	// Load the settings file
	cfg, err := config.LoadSettings("")
	if err != nil {
		// Check if it's a YAML parsing error
		if yamlErr, ok := err.(*yaml.TypeError); ok {
			result := ValidationResult{
				Valid:  false,
				Errors: []string{fmt.Sprintf("YAML parsing error: %v", yamlErr)},
			}
			return outputValidationResult(result, strict)
		}

		result := ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to load settings: %v", err)},
		}
		return outputValidationResult(result, strict)
	}

	// Perform validation
	result := validateConfig(cfg)

	// Output results
	return outputValidationResult(result, strict)
}

// validateConfig performs comprehensive validation on the loaded config.
func validateConfig(cfg *config.Config) ValidationResult {
	var errors []string
	var warnings []string

	// Check YAML structure version
	if cfg.Version == 0 {
		warnings = append(warnings, "Missing version field. Consider setting 'version: 1' in settings.yaml.")
	} else if cfg.Version != 1 {
		warnings = append(warnings, fmt.Sprintf("Unknown config version: %d. Expected version 1.", cfg.Version))
	}

	// Validate providers
	if len(cfg.Providers) == 0 {
		warnings = append(warnings, "No providers configured. Run 'conductor provider add' to add a provider.")
	} else {
		for providerName, providerCfg := range cfg.Providers {
			if providerCfg.Type == "" {
				errors = append(errors, fmt.Sprintf("Provider %q is missing required 'type' field", providerName))
			}

			// Check if provider has models
			if len(providerCfg.Models) == 0 {
				warnings = append(warnings, fmt.Sprintf("Provider %q has no models registered", providerName))
			}
		}
	}

	// Validate tier mappings
	if len(cfg.Tiers) == 0 {
		warnings = append(warnings, "No tier mappings configured. Workflows using tiers will fail.")
	} else {
		tierErrs := cfg.ValidateTiers()
		for _, tierErr := range tierErrs {
			errors = append(errors, tierErr.Error())
		}

		// Check that all standard tiers are mapped
		standardTiers := []string{"fast", "balanced", "strategic"}
		for _, tier := range standardTiers {
			if _, exists := cfg.Tiers[tier]; !exists {
				warnings = append(warnings, fmt.Sprintf("Standard tier %q is not mapped", tier))
			}
		}
	}

	// Check for orphaned tier mappings (tier references non-existent provider/model)
	// This is already handled by ValidateTiers above

	// Determine overall validity
	valid := len(errors) == 0

	return ValidationResult{
		Valid:    valid,
		Errors:   errors,
		Warnings: warnings,
	}
}

// outputValidationResult outputs the validation result and returns appropriate exit code.
func outputValidationResult(result ValidationResult, strict bool) error {
	// Output based on format
	if shared.GetJSON() {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	} else {
		// Human-readable output
		if result.Valid {
			fmt.Println(shared.RenderOK("Configuration is valid"))
		} else {
			fmt.Println(shared.RenderError("Configuration validation failed"))
		}
		fmt.Println()

		if len(result.Errors) > 0 {
			fmt.Println(shared.Header.Render("Errors:"))
			for _, err := range result.Errors {
				fmt.Printf("  %s %s\n", shared.StatusError.Render(shared.SymbolError), err)
			}
			fmt.Println()
		}

		if len(result.Warnings) > 0 {
			fmt.Println(shared.Header.Render("Warnings:"))
			for _, warn := range result.Warnings {
				fmt.Printf("  %s %s\n", shared.StatusWarn.Render(shared.SymbolWarn), warn)
			}
			fmt.Println()
		}

		if result.Valid && len(result.Warnings) == 0 {
			fmt.Println("No issues found.")
		}
	}

	// Determine exit code
	if !result.Valid {
		os.Exit(1)
	}

	// In strict mode, warnings become errors
	if strict && len(result.Warnings) > 0 {
		if !shared.GetJSON() {
			fmt.Println("Validation failed (strict mode: warnings treated as errors)")
		}
		os.Exit(1)
	}

	return nil
}
