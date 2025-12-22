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
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"gopkg.in/yaml.v3"
)

// NewConfigCommand creates the config command with subcommands
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and manage configuration",
		Long: `View and manage Conductor configuration.

Subcommands:
  show - Display current configuration
  path - Show config file location`,
	}

	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigPathCommand())

	// If no subcommand provided, default to 'show'
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newConfigShowCommand().RunE(cmd, args)
	}

	return cmd
}

// newConfigShowCommand creates the 'config show' subcommand
func newConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display current configuration",
		Long: `Display the current effective configuration.

Sensitive values (API keys) are masked for security.
Use --json for machine-readable output.`,
		RunE: runConfigShow,
	}

	return cmd
}

// newConfigPathCommand creates the 'config path' subcommand
func newConfigPathCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show config file location",
		Long:  `Display the path to the configuration file.`,
		RunE:  runConfigPath,
	}

	return cmd
}

// runConfigShow displays the current configuration
func runConfigShow(cmd *cobra.Command, args []string) error {
	// Get config path
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
		if shared.GetJSON() {
			// Output empty config in JSON
			fmt.Println("{}")
			return nil
		}
		return fmt.Errorf("no configuration file found at %s\nRun 'conductor init' to create one", cfgPath)
	}

	// Load config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Mask sensitive values
	maskedCfg := maskSensitiveConfig(cfg)

	// Output based on format
	if shared.GetJSON() {
		return outputConfigJSON(maskedCfg)
	}
	return outputConfigYAML(cfgPath, maskedCfg)
}

// runConfigPath displays the config file path
func runConfigPath(cmd *cobra.Command, args []string) error {
	cfgPath := shared.GetConfigPath()
	if cfgPath == "" {
		var err error
		cfgPath, err = config.ConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine config path: %w", err)
		}
	}

	fmt.Println(cfgPath)
	return nil
}

// maskSensitiveConfig creates a copy of config with sensitive values masked
func maskSensitiveConfig(cfg *config.Config) *config.Config {
	masked := *cfg

	// Mask API keys in providers
	maskedProviders := make(config.ProvidersMap)
	for name, provider := range cfg.Providers {
		maskedProvider := provider
		if maskedProvider.APIKey != "" {
			maskedProvider.APIKey = maskAPIKey(maskedProvider.APIKey)
		}
		maskedProviders[name] = maskedProvider
	}
	masked.Providers = maskedProviders

	return &masked
}

// maskAPIKey masks an API key for display
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}

	// If it's an environment variable reference, don't mask
	if strings.HasPrefix(key, "${") && strings.HasSuffix(key, "}") {
		return key
	}

	// Show first 4 and last 4 characters
	if len(key) <= 8 {
		return "****"
	}

	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// outputConfigJSON outputs config in JSON format
func outputConfigJSON(cfg *config.Config) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cfg)
}

// outputConfigYAML outputs config in YAML format
func outputConfigYAML(path string, cfg *config.Config) error {
	fmt.Printf("Configuration: %s\n", path)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)

	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return encoder.Close()
}
