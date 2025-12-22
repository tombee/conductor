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
	"github.com/tombee/conductor/internal/config"
)

// TierDefaults defines the default model tier mappings for a provider type.
// These are used for auto-assignment when adding the first provider.
type TierDefaults struct {
	Fast      string // Model ID for fast tier
	Balanced  string // Model ID for balanced tier
	Strategic string // Model ID for strategic tier
}

// HasDefaults returns true if this provider type has known tier defaults.
func (t TierDefaults) HasDefaults() bool {
	return t.Fast != "" || t.Balanced != "" || t.Strategic != ""
}

// ToModelTierMap converts TierDefaults to config.ModelTierMap.
func (t TierDefaults) ToModelTierMap() config.ModelTierMap {
	return config.ModelTierMap{
		Fast:      t.Fast,
		Balanced:  t.Balanced,
		Strategic: t.Strategic,
	}
}

// providerTierDefaults maps provider types to their default tier assignments.
// Only claude-code has defaults because it supports simple aliases (haiku, sonnet, opus)
// that automatically resolve to the latest model versions.
// Other providers require users to specify models since version numbers change frequently.
var providerTierDefaults = map[string]TierDefaults{
	// Claude Code supports simple model aliases that auto-update to latest versions
	// See: https://code.claude.com/docs/en/model-config
	"claude-code": {
		Fast:      "haiku",  // Fast, efficient model for simple tasks
		Balanced:  "sonnet", // Balanced model for daily coding
		Strategic: "opus",   // Complex reasoning model
	},
	// Note: anthropic, openai, ollama, openai-compatible don't have defaults
	// because their APIs require specific model version IDs that change frequently.
	// Users must specify models via workflow steps or manual config editing.
}

// GetTierDefaults returns the default tier mappings for a provider type.
// Returns an empty TierDefaults if the provider type has no known defaults.
func GetTierDefaults(providerType string) TierDefaults {
	if defaults, ok := providerTierDefaults[providerType]; ok {
		return defaults
	}
	return TierDefaults{}
}

// HasTierDefaults returns true if a provider type has known tier defaults.
func HasTierDefaults(providerType string) bool {
	_, ok := providerTierDefaults[providerType]
	return ok
}
