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

package setup

import (
	"context"
	"os/exec"
	"time"

	"github.com/tombee/conductor/internal/config"
)

// ProviderType defines the interface for provider type definitions.
// Each provider type knows how to configure and test itself.
type ProviderType interface {
	// Name returns the provider type name (e.g., "claude-code", "ollama")
	Name() string

	// DisplayName returns the human-readable name
	DisplayName() string

	// Description returns a short description
	Description() string

	// IsCLI returns true if this provider uses a CLI tool
	IsCLI() bool

	// RequiresAPIKey returns true if this provider requires an API key
	RequiresAPIKey() bool

	// RequiresBaseURL returns true if this provider requires a base URL
	RequiresBaseURL() bool

	// DefaultBaseURL returns the default base URL (empty if not applicable)
	DefaultBaseURL() string

	// DetectCLI attempts to detect the CLI tool on the system
	// Returns true if detected, false otherwise
	DetectCLI(ctx context.Context) (bool, string, error)

	// ValidateConfig validates provider-specific configuration
	ValidateConfig(cfg config.ProviderConfig) error

	// CreateConfig creates a default provider configuration
	CreateConfig() config.ProviderConfig
}

// providerRegistry holds all registered provider types
var providerRegistry = map[string]ProviderType{
	"claude-code":       &ClaudeCodeProviderType{},
	"ollama":            &OllamaProviderType{},
	"anthropic":         &AnthropicProviderType{},
	"openai-compatible": &OpenAICompatibleProviderType{},
}

// GetProviderTypes returns all registered provider types
func GetProviderTypes() []ProviderType {
	types := make([]ProviderType, 0, len(providerRegistry))
	for _, pt := range providerRegistry {
		types = append(types, pt)
	}
	return types
}

// GetProviderType returns a provider type by name
func GetProviderType(name string) (ProviderType, bool) {
	pt, ok := providerRegistry[name]
	return pt, ok
}

// ClaudeCodeProviderType implements ProviderType for Claude Code CLI
type ClaudeCodeProviderType struct{}

func (p *ClaudeCodeProviderType) Name() string           { return "claude-code" }
func (p *ClaudeCodeProviderType) DisplayName() string    { return "Claude Code" }
func (p *ClaudeCodeProviderType) Description() string    { return "Uses the Claude Code CLI" }
func (p *ClaudeCodeProviderType) IsCLI() bool            { return true }
func (p *ClaudeCodeProviderType) RequiresAPIKey() bool   { return false }
func (p *ClaudeCodeProviderType) RequiresBaseURL() bool  { return false }
func (p *ClaudeCodeProviderType) DefaultBaseURL() string { return "" }
func (p *ClaudeCodeProviderType) ValidateConfig(cfg config.ProviderConfig) error {
	return nil
}
func (p *ClaudeCodeProviderType) CreateConfig() config.ProviderConfig {
	return config.ProviderConfig{
		Type: "claude-code",
	}
}

func (p *ClaudeCodeProviderType) DetectCLI(ctx context.Context) (bool, string, error) {
	_, err := exec.LookPath("claude")
	if err != nil {
		return false, "", nil
	}

	// Try to get version
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--version")
	output, err := cmd.Output()
	if err != nil {
		return true, "", nil // Found but version check failed
	}

	return true, string(output), nil
}

// OllamaProviderType implements ProviderType for Ollama CLI
type OllamaProviderType struct{}

func (p *OllamaProviderType) Name() string           { return "ollama" }
func (p *OllamaProviderType) DisplayName() string    { return "Ollama" }
func (p *OllamaProviderType) Description() string    { return "Uses local Ollama models" }
func (p *OllamaProviderType) IsCLI() bool            { return true }
func (p *OllamaProviderType) RequiresAPIKey() bool   { return false }
func (p *OllamaProviderType) RequiresBaseURL() bool  { return false }
func (p *OllamaProviderType) DefaultBaseURL() string { return "http://localhost:11434" }
func (p *OllamaProviderType) ValidateConfig(cfg config.ProviderConfig) error {
	return nil
}
func (p *OllamaProviderType) CreateConfig() config.ProviderConfig {
	return config.ProviderConfig{
		Type: "ollama",
	}
}

func (p *OllamaProviderType) DetectCLI(ctx context.Context) (bool, string, error) {
	_, err := exec.LookPath("ollama")
	if err != nil {
		return false, "", nil
	}

	// Try to get version
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ollama", "--version")
	output, err := cmd.Output()
	if err != nil {
		return true, "", nil // Found but version check failed
	}

	return true, string(output), nil
}

// AnthropicProviderType implements ProviderType for Anthropic API
type AnthropicProviderType struct{}

func (p *AnthropicProviderType) Name() string           { return "anthropic" }
func (p *AnthropicProviderType) DisplayName() string    { return "Anthropic API" }
func (p *AnthropicProviderType) Description() string    { return "Uses Anthropic's Claude API directly" }
func (p *AnthropicProviderType) IsCLI() bool            { return false }
func (p *AnthropicProviderType) RequiresAPIKey() bool   { return true }
func (p *AnthropicProviderType) RequiresBaseURL() bool  { return false }
func (p *AnthropicProviderType) DefaultBaseURL() string { return "https://api.anthropic.com" }
func (p *AnthropicProviderType) ValidateConfig(cfg config.ProviderConfig) error {
	return nil
}
func (p *AnthropicProviderType) CreateConfig() config.ProviderConfig {
	return config.ProviderConfig{
		Type: "anthropic",
	}
}

func (p *AnthropicProviderType) DetectCLI(ctx context.Context) (bool, string, error) {
	return false, "", nil // Not a CLI provider
}

// OpenAICompatibleProviderType implements ProviderType for OpenAI-compatible APIs
type OpenAICompatibleProviderType struct{}

func (p *OpenAICompatibleProviderType) Name() string           { return "openai-compatible" }
func (p *OpenAICompatibleProviderType) DisplayName() string    { return "OpenAI-Compatible API" }
func (p *OpenAICompatibleProviderType) Description() string    { return "Uses any OpenAI-compatible API" }
func (p *OpenAICompatibleProviderType) IsCLI() bool            { return false }
func (p *OpenAICompatibleProviderType) RequiresAPIKey() bool   { return true }
func (p *OpenAICompatibleProviderType) RequiresBaseURL() bool  { return true }
func (p *OpenAICompatibleProviderType) DefaultBaseURL() string { return "https://api.openai.com/v1" }
func (p *OpenAICompatibleProviderType) ValidateConfig(cfg config.ProviderConfig) error {
	return nil
}
func (p *OpenAICompatibleProviderType) CreateConfig() config.ProviderConfig {
	return config.ProviderConfig{
		Type: "openai-compatible",
	}
}

func (p *OpenAICompatibleProviderType) DetectCLI(ctx context.Context) (bool, string, error) {
	return false, "", nil // Not a CLI provider
}
