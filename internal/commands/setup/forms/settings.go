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

package forms

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
)

// SettingsMenuChoice represents a selection in the settings menu
type SettingsMenuChoice string

const (
	SettingsChangeBackend    SettingsMenuChoice = "change_backend"
	SettingsAddBackend       SettingsMenuChoice = "add_backend"
	SettingsViewCredentials  SettingsMenuChoice = "view_credentials"
	SettingsMigratePlaintext SettingsMenuChoice = "migrate_plaintext"
	SettingsBack             SettingsMenuChoice = "back"
)

// ShowSettingsMenu displays the settings management screen.
func ShowSettingsMenu(state *setup.SetupState) (SettingsMenuChoice, error) {
	var choice string

	// Build backend list summary
	backendList := buildBackendListSummary(state)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Settings\n\n"+backendList),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Change default backend", string(SettingsChangeBackend)),
					huh.NewOption("Add secrets backend", string(SettingsAddBackend)),
					huh.NewOption("View stored credentials", string(SettingsViewCredentials)),
					huh.NewOption("Migrate plaintext credentials", string(SettingsMigratePlaintext)),
					huh.NewOption("Back", string(SettingsBack)),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return SettingsMenuChoice(choice), nil
}

// buildBackendListSummary builds a formatted list of available secrets backends
func buildBackendListSummary(state *setup.SetupState) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Secrets Backend: %s (default)", state.SecretsBackend))
	lines = append(lines, "")
	lines = append(lines, "Available backends:")

	backends := setup.GetBackendTypes()
	for _, backend := range backends {
		marker := "○"
		suffix := ""
		if backend.ID() == state.SecretsBackend {
			marker = "●"
			suffix = " (current default)"
		}

		available := ""
		if backend.IsAvailable() {
			available = " ✓"
		} else {
			available = " ✗"
		}

		lines = append(lines, fmt.Sprintf("  %s %s - %s%s%s",
			marker, backend.ID(), backend.Name(), available, suffix))
	}

	return strings.Join(lines, "\n")
}

// ChangeDefaultBackend allows user to select a new default secrets backend
func ChangeDefaultBackend(state *setup.SetupState) error {
	backends := setup.GetBackendTypes()
	if len(backends) == 0 {
		return fmt.Errorf("no backends available")
	}

	// Build options for available backends only
	options := make([]huh.Option[string], 0)
	for _, backend := range backends {
		if backend.IsAvailable() {
			label := fmt.Sprintf("%s - %s", backend.ID(), backend.Name())
			if backend.ID() == state.SecretsBackend {
				label += " [current]"
			}
			options = append(options, huh.NewOption(label, backend.ID()))
		}
	}

	if len(options) == 0 {
		return fmt.Errorf("no available backends found")
	}

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select default secrets backend:").
				Description("This backend will be used for new credentials").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	if selected != state.SecretsBackend {
		state.SecretsBackend = selected
		state.MarkDirty()
	}

	return nil
}

// ViewStoredCredentials shows a read-only view of stored credentials
func ViewStoredCredentials(state *setup.SetupState) error {
	// Collect credentials from config
	var lines []string

	// Group by backend
	backendCreds := make(map[string][]string)

	// Scan provider credentials
	for name, provider := range state.Working.Providers {
		if provider.APIKey != "" {
			backend, key := parseCredentialReference(provider.APIKey)
			if backend != "" {
				masked := maskCredentialValue(key)
				entry := fmt.Sprintf("  🔐 %s (provider: %s) %s", key, name, masked)
				backendCreds[backend] = append(backendCreds[backend], entry)
			}
		}
	}

	// TODO: Add integration credentials once integration support is added

	if len(backendCreds) == 0 {
		lines = append(lines, "No credentials stored yet.")
	} else {
		for backend, creds := range backendCreds {
			lines = append(lines, fmt.Sprintf("%s:", backend))
			lines = append(lines, creds...)
			lines = append(lines, "")
		}
	}

	message := "Stored Credentials\n\n" + strings.Join(lines, "\n")

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to go back").
				Affirmative("Back").
				Negative(""),
		),
	)

	return form.Run()
}

// parseCredentialReference parses a credential reference like "$secret:KEY" or "$env:VAR"
// and returns (backend, key)
func parseCredentialReference(ref string) (string, string) {
	if !strings.HasPrefix(ref, "$") {
		return "", ""
	}

	parts := strings.SplitN(ref[1:], ":", 2)
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}

// maskCredentialValue masks a credential value for display
// Shows first 4 and last 4 chars with dots in between
func maskCredentialValue(value string) string {
	if len(value) <= 8 {
		return "••••••••"
	}
	return fmt.Sprintf("%s•••••%s", value[:4], value[len(value)-4:])
}

// DetectPlaintextCredentials scans the config for plaintext credentials
func DetectPlaintextCredentials(state *setup.SetupState) []PlaintextCredential {
	var creds []PlaintextCredential

	// Scan provider API keys
	for name, provider := range state.Working.Providers {
		if provider.APIKey != "" && !strings.HasPrefix(provider.APIKey, "$") {
			// This looks like a plaintext credential
			credType := detectCredentialType(provider.APIKey)
			creds = append(creds, PlaintextCredential{
				Source: fmt.Sprintf("provider:%s", name),
				Field:  "api_key",
				Value:  provider.APIKey,
				Type:   credType,
			})
		}
	}

	// TODO: Scan integration credentials once integration support is added

	return creds
}

// PlaintextCredential represents a detected plaintext credential
type PlaintextCredential struct {
	Source string // e.g., "provider:anthropic" or "integration:github"
	Field  string // e.g., "api_key", "token"
	Value  string // The plaintext value
	Type   string // Detected type (e.g., "anthropic", "github-pat")
}

// detectCredentialType attempts to identify the credential type from its value
func detectCredentialType(value string) string {
	patterns := map[string]*regexp.Regexp{
		"anthropic":    regexp.MustCompile(`^sk-ant-api\d{2}-[A-Za-z0-9_-]{93}$`),
		"openai":       regexp.MustCompile(`^sk-[A-Za-z0-9]{48}$`),
		"openai-proj":  regexp.MustCompile(`^sk-proj-[A-Za-z0-9_-]{48,}$`),
		"github-pat":   regexp.MustCompile(`^ghp_[A-Za-z0-9]{36}$`),
		"github-fine":  regexp.MustCompile(`^github_pat_[A-Za-z0-9]{22}_[A-Za-z0-9]{59}$`),
		"github-oauth": regexp.MustCompile(`^gho_[A-Za-z0-9]{36}$`),
		"github-user":  regexp.MustCompile(`^ghu_[A-Za-z0-9]{36}$`),
		"github-srv":   regexp.MustCompile(`^ghs_[A-Za-z0-9]{36}$`),
		"github-ref":   regexp.MustCompile(`^ghr_[A-Za-z0-9]{36}$`),
		"slack-bot":    regexp.MustCompile(`^xoxb-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
		"slack-user":   regexp.MustCompile(`^xoxp-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
		"slack-app":    regexp.MustCompile(`^xoxa-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
		"slack-ref":    regexp.MustCompile(`^xoxr-[0-9]+-[0-9]+-[A-Za-z0-9]+$`),
		"google-api":   regexp.MustCompile(`^AIza[A-Za-z0-9_-]{35}$`),
		"aws-access":   regexp.MustCompile(`^AKIA[A-Z0-9]{16}$`),
		"gitlab-pat":   regexp.MustCompile(`^glpat-[A-Za-z0-9_-]{20}$`),
	}

	for credType, pattern := range patterns {
		if pattern.MatchString(value) {
			return credType
		}
	}

	// Check for Bearer/Basic tokens
	if strings.HasPrefix(value, "Bearer ") {
		return "bearer-token"
	}
	if strings.HasPrefix(value, "Basic ") {
		return "basic-auth"
	}

	return "unknown"
}

// MigratePlaintextCredentials guides user through migrating plaintext credentials
func MigratePlaintextCredentials(state *setup.SetupState) error {
	creds := DetectPlaintextCredentials(state)

	if len(creds) == 0 {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("✓ No plaintext credentials detected\n\nAll credentials are using secure storage."),
				huh.NewConfirm().
					Title("Press Enter to go back").
					Affirmative("Back").
					Negative(""),
			),
		)
		return form.Run()
	}

	// Build list of detected credentials
	var lines []string
	lines = append(lines, "⚠ Plaintext credentials detected")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Found %d credential(s) in config:", len(creds)))
	for _, cred := range creds {
		lines = append(lines, fmt.Sprintf("  • %s (%s)", cred.Source, cred.Type))
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(strings.Join(lines, "\n")),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Migrate all to default backend", "migrate_all"),
					huh.NewOption("Migrate individually", "migrate_individual"),
					huh.NewOption("Skip (not recommended)", "skip"),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	switch choice {
	case "migrate_all":
		return migrateAllCredentials(state, creds)
	case "migrate_individual":
		return migrateCredentialsIndividually(state, creds)
	case "skip":
		return nil
	}

	return nil
}

// migrateAllCredentials migrates all detected plaintext credentials to the default backend
func migrateAllCredentials(state *setup.SetupState, creds []PlaintextCredential) error {
	// TODO: Implement actual migration to secrets backend
	// For now, just update references to use default backend

	for _, cred := range creds {
		// Parse source to determine what to update
		parts := strings.SplitN(cred.Source, ":", 2)
		if len(parts) != 2 {
			continue
		}

		sourceType := parts[0]
		sourceName := parts[1]

		// Generate credential key name
		keyName := fmt.Sprintf("%s_%s", strings.ToUpper(sourceName), strings.ToUpper(cred.Field))

		// Store value in credential store
		storeKey := fmt.Sprintf("%s:%s:%s", sourceType, sourceName, cred.Field)
		state.CredentialStore[storeKey] = cred.Value

		// Update config reference
		credRef := fmt.Sprintf("$%s:%s", state.SecretsBackend, keyName)

		if sourceType == "provider" {
			if provider, ok := state.Working.Providers[sourceName]; ok {
				provider.APIKey = credRef
				state.Working.Providers[sourceName] = provider
			}
		}
		// TODO: Handle integration credentials
	}

	state.MarkDirty()

	// Show success message
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("✓ Migrated %d credential(s) to %s backend", len(creds), state.SecretsBackend)),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}

// migrateCredentialsIndividually guides user through migrating each credential
func migrateCredentialsIndividually(state *setup.SetupState, creds []PlaintextCredential) error {
	// TODO: Implement individual migration flow
	// For now, just call migrate all
	return migrateAllCredentials(state, creds)
}
