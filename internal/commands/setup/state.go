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

// SetupState tracks the wizard's working configuration and dirty state.
type SetupState struct {
	// Original holds the configuration loaded from disk.
	// This is used for rollback and dirty detection.
	Original *config.Config

	// Working is the current working copy being edited in the wizard.
	// All modifications are made to this copy.
	Working *config.Config

	// Dirty indicates whether Working differs from Original.
	// Used to prompt users about unsaved changes on exit.
	Dirty bool

	// ConfigPath is the path to the configuration file.
	ConfigPath string

	// SecretsBackend is the user's selected default secrets backend.
	// Options: "keychain", "env", "file"
	SecretsBackend string

	// CredentialStore temporarily holds credentials during wizard flow
	// before they are persisted via secrets backend.
	// Keys are in format: "provider:<name>:api_key" or "integration:<name>:<field>"
	// This map is zeroed on exit for security.
	CredentialStore map[string]string
}

// NewSetupState creates a new setup state with the given config.
// If cfg is nil, creates a minimal default config.
func NewSetupState(cfg *config.Config, configPath string) *SetupState {
	if cfg == nil {
		cfg = &config.Config{
			Providers: make(config.ProvidersMap),
		}
	}

	// Deep copy the config for working copy
	working := copyConfig(cfg)

	return &SetupState{
		Original:        cfg,
		Working:         working,
		Dirty:           false,
		ConfigPath:      configPath,
		CredentialStore: make(map[string]string),
	}
}

// MarkDirty marks the state as dirty (has unsaved changes).
func (s *SetupState) MarkDirty() {
	s.Dirty = true
}

// IsDirty returns whether there are unsaved changes.
func (s *SetupState) IsDirty() bool {
	return s.Dirty
}

// Reset resets the working config to match the original.
func (s *SetupState) Reset() {
	s.Working = copyConfig(s.Original)
	s.Dirty = false
	s.clearCredentials()
}

// clearCredentials zeros all credentials in the credential store.
// This is called on exit to prevent credentials from remaining in memory.
func (s *SetupState) clearCredentials() {
	for key := range s.CredentialStore {
		// Zero the string by replacing with empty string
		// Note: Go strings are immutable, but this removes the reference
		s.CredentialStore[key] = ""
	}
	s.CredentialStore = make(map[string]string)
}

// copyConfig creates a deep copy of a config.Config.
func copyConfig(cfg *config.Config) *config.Config {
	if cfg == nil {
		return nil
	}

	// Create a new config with copied fields
	copied := &config.Config{
		Server:                   cfg.Server,
		Auth:                     cfg.Auth,
		Log:                      cfg.Log,
		LLM:                      cfg.LLM,
		Controller:               cfg.Controller,
		Security:                 cfg.Security,
		DefaultProvider:          cfg.DefaultProvider,
		SuppressUnmappedWarnings: cfg.SuppressUnmappedWarnings,
	}

	// Deep copy maps
	if cfg.Providers != nil {
		copied.Providers = make(config.ProvidersMap, len(cfg.Providers))
		for k, v := range cfg.Providers {
			copied.Providers[k] = v
		}
	}

	if cfg.AgentMappings != nil {
		copied.AgentMappings = make(config.AgentMappings, len(cfg.AgentMappings))
		for k, v := range cfg.AgentMappings {
			copied.AgentMappings[k] = v
		}
	}

	if cfg.AcknowledgedDefaults != nil {
		copied.AcknowledgedDefaults = make([]string, len(cfg.AcknowledgedDefaults))
		copy(copied.AcknowledgedDefaults, cfg.AcknowledgedDefaults)
	}

	if cfg.Workspaces != nil {
		copied.Workspaces = make(map[string]config.Workspace, len(cfg.Workspaces))
		for k, v := range cfg.Workspaces {
			copied.Workspaces[k] = v
		}
	}

	return copied
}
