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
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
	"github.com/tombee/conductor/internal/commands/setup/actions"
)

// IntegrationsMenuChoice represents a selection in the integrations menu
type IntegrationsMenuChoice string

const (
	IntegrationAdd     IntegrationsMenuChoice = "add"
	IntegrationEdit    IntegrationsMenuChoice = "edit"
	IntegrationRemove  IntegrationsMenuChoice = "remove"
	IntegrationTestAll IntegrationsMenuChoice = "test_all"
	IntegrationDone    IntegrationsMenuChoice = "done"
)

// ShowIntegrationsMenu displays the integrations management screen.
func ShowIntegrationsMenu(state *setup.SetupState) (IntegrationsMenuChoice, error) {
	var choice string

	// Build integration list summary
	integrationList := buildIntegrationListSummary(state)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Integrations\n\n"+integrationList),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(
					huh.NewOption("Add integration", string(IntegrationAdd)),
					huh.NewOption("Edit integration", string(IntegrationEdit)),
					huh.NewOption("Remove integration", string(IntegrationRemove)),
					huh.NewOption("Test all integrations", string(IntegrationTestAll)),
					huh.NewOption("Done with integrations", string(IntegrationDone)),
				).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return IntegrationsMenuChoice(choice), nil
}

// buildIntegrationListSummary builds a formatted list of configured integrations
func buildIntegrationListSummary(state *setup.SetupState) string {
	// Find all integrations in credential store
	integrations := make(map[string]string)
	for k, v := range state.CredentialStore {
		if strings.HasPrefix(k, "integration:") && strings.Count(k, ":") == 1 {
			integrationName := strings.TrimPrefix(k, "integration:")
			integrations[integrationName] = v
		}
	}

	if len(integrations) == 0 {
		return "No integrations configured yet."
	}

	var lines []string
	lines = append(lines, "Configured integrations:")

	for name, integrationType := range integrations {
		// Get integration config
		integrationKey := fmt.Sprintf("integration:%s", name)
		var details []string

		for k, v := range state.CredentialStore {
			if strings.HasPrefix(k, integrationKey+":") {
				fieldName := strings.TrimPrefix(k, integrationKey+":")
				// Show non-secret fields
				if fieldName == "base_url" {
					details = append(details, v)
				}
			}
		}

		detailStr := ""
		if len(details) > 0 {
			detailStr = " - " + strings.Join(details, ", ")
		}

		lines = append(lines, fmt.Sprintf("  ✓ %s (%s)%s", name, integrationType, detailStr))
	}

	return strings.Join(lines, "\n")
}

// AddIntegrationFlow guides the user through adding a new integration.
func AddIntegrationFlow(ctx context.Context, state *setup.SetupState) error {
	// Get available integration types
	integrationTypes := setup.GetIntegrationTypes()

	if len(integrationTypes) == 0 {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("No integration types available yet.\n\nIntegration support is coming soon."),
				huh.NewConfirm().
					Title("Press Enter to go back").
					Affirmative("Back").
					Negative(""),
			),
		)
		return form.Run()
	}

	// Build selection options
	options := make([]huh.Option[string], 0, len(integrationTypes)+1)
	for _, it := range integrationTypes {
		options = append(options, huh.NewOption(
			it.DisplayName()+" - "+it.Description(),
			it.ID(),
		))
	}
	options = append(options, huh.NewOption("Back", "back"))

	var selectedType string
	typeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select integration type:").
				Options(options...).
				Value(&selectedType),
		),
	)

	if err := typeForm.Run(); err != nil {
		return err
	}

	if selectedType == "back" {
		return nil
	}

	integrationType, ok := setup.GetIntegrationType(selectedType)
	if !ok {
		return fmt.Errorf("unknown integration type: %s", selectedType)
	}

	// Step 2: Get instance name
	var instanceName string
	defaultName := selectedType

	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("%s integration name:", integrationType.DisplayName())).
				Description("Examples: github, github-work, slack, jira-cloud").
				Value(&instanceName).
				Placeholder(defaultName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("integration name is required")
					}
					// TODO: Check if integration name already exists once integrations are in config
					return nil
				}),
		),
	)

	if err := nameForm.Run(); err != nil {
		return err
	}

	if instanceName == "" {
		instanceName = defaultName
	}

	// Get integration type info
	integrationInfo, ok := GetIntegrationTypeInfo(selectedType)
	if !ok {
		return fmt.Errorf("integration type %q not found", selectedType)
	}

	// Collect field values
	fieldValues := make(map[string]string)

	// Collect each field
	for _, field := range integrationInfo.Fields {
		var value string

		if field.IsSecret {
			// Prompt for secret with password masking
			keyForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("%s:", field.DisplayName)).
						EchoMode(huh.EchoModePassword).
						Value(&value).
						Validate(func(s string) error {
							if field.Required && s == "" {
								return fmt.Errorf("%s is required", field.DisplayName)
							}
							return nil
						}),
				),
			)

			if err := keyForm.Run(); err != nil {
				return err
			}

			// Store in credential store
			credKey := fmt.Sprintf("integration:%s:%s", instanceName, field.Name)
			state.CredentialStore[credKey] = value

			// Store reference
			envVarName := field.DefaultEnv
			if envVarName == "" {
				envVarName = fmt.Sprintf("%s_%s", strings.ToUpper(instanceName), strings.ToUpper(field.Name))
			}
			fieldValues[field.Name] = fmt.Sprintf("$secret:%s", envVarName)

		} else {
			// Regular field
			inputForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title(fmt.Sprintf("%s:", field.DisplayName)).
						Value(&value).
						Validate(func(s string) error {
							if field.Required && s == "" {
								return fmt.Errorf("%s is required", field.DisplayName)
							}
							return nil
						}),
				),
			)

			if err := inputForm.Run(); err != nil {
				return err
			}

			if value != "" {
				fieldValues[field.Name] = value
			}
		}
	}

	// Store integration config (for now just mark as configured)
	integrationKey := fmt.Sprintf("integration:%s", instanceName)
	state.CredentialStore[integrationKey] = selectedType
	for k, v := range fieldValues {
		state.CredentialStore[fmt.Sprintf("%s:%s", integrationKey, k)] = v
	}

	state.MarkDirty()

	// Show success message
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("✓ Integration %q configured successfully", instanceName)),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}

// SelectIntegrationForEdit shows a list of integrations and returns the selected one
func SelectIntegrationForEdit(state *setup.SetupState) (string, error) {
	// Find all integrations in credential store
	integrations := make(map[string]string)
	for k, v := range state.CredentialStore {
		if strings.HasPrefix(k, "integration:") && strings.Count(k, ":") == 1 {
			integrationName := strings.TrimPrefix(k, "integration:")
			integrations[integrationName] = v
		}
	}

	if len(integrations) == 0 {
		return "", fmt.Errorf("no integrations configured")
	}

	options := make([]huh.Option[string], 0, len(integrations)+1)
	for name, integrationType := range integrations {
		label := fmt.Sprintf("%s (%s)", name, integrationType)
		options = append(options, huh.NewOption(label, name))
	}
	options = append(options, huh.NewOption("Back", ""))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select integration to edit:").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}

// SelectIntegrationForRemoval shows a list of integrations and returns the selected one
func SelectIntegrationForRemoval(state *setup.SetupState) (string, error) {
	// Find all integrations in credential store
	integrations := make(map[string]string)
	for k, v := range state.CredentialStore {
		if strings.HasPrefix(k, "integration:") && strings.Count(k, ":") == 1 {
			integrationName := strings.TrimPrefix(k, "integration:")
			integrations[integrationName] = v
		}
	}

	if len(integrations) == 0 {
		return "", fmt.Errorf("no integrations configured")
	}

	options := make([]huh.Option[string], 0, len(integrations)+1)
	for name, integrationType := range integrations {
		label := fmt.Sprintf("%s (%s)", name, integrationType)
		options = append(options, huh.NewOption(label, name))
	}
	options = append(options, huh.NewOption("Back", ""))

	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select integration to remove:").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}

	return selected, nil
}

// ConfirmRemoveIntegration confirms removal of an integration
func ConfirmRemoveIntegration(name string) (bool, error) {
	var confirm bool
	message := fmt.Sprintf("Remove integration %q?", name)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return false, err
	}

	return confirm, nil
}

// RemoveIntegration removes an integration
func RemoveIntegration(state *setup.SetupState, name string) error {
	// Confirm removal
	confirmed, err := ConfirmRemoveIntegration(name)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	// Remove integration and all its fields from credential store
	integrationKey := fmt.Sprintf("integration:%s", name)
	keysToDelete := []string{}
	for k := range state.CredentialStore {
		if k == integrationKey || strings.HasPrefix(k, integrationKey+":") {
			keysToDelete = append(keysToDelete, k)
		}
	}

	for _, k := range keysToDelete {
		delete(state.CredentialStore, k)
	}

	state.MarkDirty()

	return nil
}

// IntegrationTypeInfo provides metadata about an integration type
type IntegrationTypeInfo struct {
	ID          string
	DisplayName string
	Description string
	Fields      []IntegrationField
}

// IntegrationField defines a configuration field for an integration
type IntegrationField struct {
	Name        string
	DisplayName string
	Required    bool
	IsSecret    bool
	DefaultEnv  string
}

// Common integration types
var commonIntegrationTypes = []IntegrationTypeInfo{
	{
		ID:          "github",
		DisplayName: "GitHub",
		Description: "GitHub API integration",
		Fields: []IntegrationField{
			{Name: "token", DisplayName: "Access Token", Required: true, IsSecret: true, DefaultEnv: "GITHUB_TOKEN"},
			{Name: "base_url", DisplayName: "Base URL (for Enterprise)", Required: false, IsSecret: false},
		},
	},
	{
		ID:          "slack",
		DisplayName: "Slack",
		Description: "Slack API integration",
		Fields: []IntegrationField{
			{Name: "bot_token", DisplayName: "Bot Token", Required: true, IsSecret: true, DefaultEnv: "SLACK_BOT_TOKEN"},
		},
	},
	{
		ID:          "jira",
		DisplayName: "Jira",
		Description: "Jira Cloud/Server integration",
		Fields: []IntegrationField{
			{Name: "base_url", DisplayName: "Base URL", Required: true, IsSecret: false},
			{Name: "email", DisplayName: "Email", Required: true, IsSecret: false, DefaultEnv: "JIRA_EMAIL"},
			{Name: "api_token", DisplayName: "API Token", Required: true, IsSecret: true, DefaultEnv: "JIRA_API_TOKEN"},
		},
	},
	{
		ID:          "discord",
		DisplayName: "Discord",
		Description: "Discord bot integration",
		Fields: []IntegrationField{
			{Name: "bot_token", DisplayName: "Bot Token", Required: true, IsSecret: true, DefaultEnv: "DISCORD_BOT_TOKEN"},
		},
	},
	{
		ID:          "jenkins",
		DisplayName: "Jenkins",
		Description: "Jenkins CI/CD integration",
		Fields: []IntegrationField{
			{Name: "base_url", DisplayName: "Base URL", Required: true, IsSecret: false},
			{Name: "username", DisplayName: "Username", Required: true, IsSecret: false, DefaultEnv: "JENKINS_USER"},
			{Name: "api_token", DisplayName: "API Token", Required: true, IsSecret: true, DefaultEnv: "JENKINS_API_TOKEN"},
		},
	},
}

// GetCommonIntegrationTypes returns the list of common integration types
func GetCommonIntegrationTypes() []IntegrationTypeInfo {
	return commonIntegrationTypes
}

// GetIntegrationTypeInfo returns info for a specific integration type
func GetIntegrationTypeInfo(id string) (IntegrationTypeInfo, bool) {
	for _, info := range commonIntegrationTypes {
		if info.ID == id {
			return info, true
		}
	}
	return IntegrationTypeInfo{}, false
}

// BuildIntegrationSummary builds a summary line for an integration
func BuildIntegrationSummary(name, integrationType string, config map[string]string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("✓ %s", name))

	// Add type-specific info
	if baseURL, ok := config["base_url"]; ok && baseURL != "" {
		parts = append(parts, baseURL)
	}

	// Add credential info
	for field, value := range config {
		if strings.Contains(field, "token") || strings.Contains(field, "key") {
			parts = append(parts, fmt.Sprintf("%s: %s", field, value))
		}
	}

	return strings.Join(parts, " ")
}

// EditIntegrationFlow guides the user through editing an existing integration
func EditIntegrationFlow(ctx context.Context, state *setup.SetupState, integrationName string) error {
	// Get integration from credential store
	integrationKey := fmt.Sprintf("integration:%s", integrationName)
	integrationType, ok := state.CredentialStore[integrationKey]
	if !ok {
		return fmt.Errorf("integration %q not found", integrationName)
	}

	integrationInfo, ok := GetIntegrationTypeInfo(integrationType)
	if !ok {
		return fmt.Errorf("integration type %q not found", integrationType)
	}

	// Show edit menu
	var choice string
	options := []huh.Option[string]{
		huh.NewOption("Test connection", "test"),
		huh.NewOption("Reconfigure", "reconfigure"),
		huh.NewOption("Done editing", "done"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("Edit Integration: %s\nType: %s", integrationName, integrationInfo.DisplayName)),
			huh.NewSelect[string]().
				Title("What would you like to do?").
				Options(options...).
				Value(&choice),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	switch choice {
	case "test":
		return testSingleIntegration(ctx, state, integrationName, integrationType)
	case "reconfigure":
		// Re-run the add flow
		if err := RemoveIntegration(state, integrationName); err != nil {
			return err
		}
		return AddIntegrationFlow(ctx, state)
	case "done":
		return nil
	}

	return nil
}

// testSingleIntegration tests a single integration connection
func testSingleIntegration(ctx context.Context, state *setup.SetupState, integrationName, integrationType string) error {
	// Get integration config from credential store
	integrationKey := fmt.Sprintf("integration:%s", integrationName)
	config := make(map[string]string)

	// Collect all config fields
	for k, v := range state.CredentialStore {
		if strings.HasPrefix(k, integrationKey+":") {
			fieldName := strings.TrimPrefix(k, integrationKey+":")
			config[fieldName] = v
		}
	}

	// Test the integration
	result := actions.TestIntegration(ctx, integrationType, config)

	// Display result
	message := fmt.Sprintf("Testing %s...\n\n%s", integrationName, result.Message)
	if !result.Success && result.ErrorDetails != "" {
		message += "\n\nError: " + result.ErrorDetails
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}

// TestAllIntegrations tests all configured integrations
func TestAllIntegrations(ctx context.Context, state *setup.SetupState) error {
	// Find all integrations in credential store
	integrations := make(map[string]string)
	for k, v := range state.CredentialStore {
		if strings.HasPrefix(k, "integration:") && strings.Count(k, ":") == 1 {
			// This is an integration entry (not a field)
			integrationName := strings.TrimPrefix(k, "integration:")
			integrations[integrationName] = v
		}
	}

	if len(integrations) == 0 {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().
					Title("No integrations configured yet."),
				huh.NewConfirm().
					Title("Press Enter to go back").
					Affirmative("Back").
					Negative(""),
			),
		)
		return form.Run()
	}

	// Test each integration
	var results []string
	for name, integrationType := range integrations {
		// Get integration config
		integrationKey := fmt.Sprintf("integration:%s", name)
		config := make(map[string]string)
		for k, v := range state.CredentialStore {
			if strings.HasPrefix(k, integrationKey+":") {
				fieldName := strings.TrimPrefix(k, integrationKey+":")
				config[fieldName] = v
			}
		}

		result := actions.TestIntegration(ctx, integrationType, config)
		status := result.StatusIcon
		results = append(results, fmt.Sprintf("%s %s (%s)", status, name, integrationType))
	}

	message := "Test Results\n\n" + strings.Join(results, "\n")

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(message),
			huh.NewConfirm().
				Title("Press Enter to continue").
				Affirmative("Continue").
				Negative(""),
		),
	)

	return form.Run()
}
