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
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/tombee/conductor/internal/commands/setup"
)

// IntegrationsMenuChoice represents a selection in the integrations menu
type IntegrationsMenuChoice string

const (
	IntegrationAdd      IntegrationsMenuChoice = "add"
	IntegrationEdit     IntegrationsMenuChoice = "edit"
	IntegrationRemove   IntegrationsMenuChoice = "remove"
	IntegrationTestAll  IntegrationsMenuChoice = "test_all"
	IntegrationDone     IntegrationsMenuChoice = "done"
)

// ShowIntegrationsMenu displays the integrations management screen.
func ShowIntegrationsMenu(state *setup.SetupState) (IntegrationsMenuChoice, error) {
	var choice string

	// Build integration list summary
	integrationList := buildIntegrationListSummary(state)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Integrations\n\n" + integrationList),
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
	// TODO: Once integrations are added to config.Config, build the actual list
	// For now, show placeholder
	return "No integrations configured yet.\n\n(Integration support coming soon)"
}

// AddIntegrationFlow guides the user through adding a new integration.
func AddIntegrationFlow(state *setup.SetupState) error {
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

	// TODO: Implement integration-specific field collection
	// TODO: Implement credential collection and backend selection
	// TODO: Implement connection testing
	// TODO: Add integration to config

	// For now, show placeholder
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title(fmt.Sprintf("Integration %q configuration coming soon.\n\nIntegration support is under development.", instanceName)),
			huh.NewConfirm().
				Title("Press Enter to go back").
				Affirmative("Back").
				Negative(""),
		),
	)

	return form.Run()
}

// SelectIntegrationForEdit shows a list of integrations and returns the selected one
func SelectIntegrationForEdit(state *setup.SetupState) (string, error) {
	// TODO: Once integrations are in config, build actual list
	return "", fmt.Errorf("no integrations configured")
}

// SelectIntegrationForRemoval shows a list of integrations and returns the selected one
func SelectIntegrationForRemoval(state *setup.SetupState) (string, error) {
	// TODO: Once integrations are in config, build actual list
	return "", fmt.Errorf("no integrations configured")
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

	// TODO: Remove the integration from config once integrations are added
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
