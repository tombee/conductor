package pagerduty

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
)

// PagerDutyIntegration implements the Provider interface for PagerDuty API.
type PagerDutyIntegration struct {
	*api.BaseProvider
}

// NewPagerDutyIntegration creates a new PagerDuty integration.
func NewPagerDutyIntegration(config *api.ProviderConfig) (operation.Provider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.pagerduty.com"
	}

	base := api.NewBaseProvider("pagerduty", config)

	return &PagerDutyIntegration{
		BaseProvider: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *PagerDutyIntegration) Execute(ctx context.Context, opName string, inputs map[string]interface{}) (*operation.Result, error) {
	switch opName {
	// Incidents
	case "list_incidents":
		return c.listIncidents(ctx, inputs)
	case "get_incident":
		return c.getIncident(ctx, inputs)
	case "update_incident":
		return c.updateIncident(ctx, inputs)
	case "acknowledge_incident":
		return c.acknowledgeIncident(ctx, inputs)
	case "resolve_incident":
		return c.resolveIncident(ctx, inputs)

	// Incident Notes
	case "list_incident_notes":
		return c.listIncidentNotes(ctx, inputs)
	case "create_incident_note":
		return c.createIncidentNote(ctx, inputs)

	// Incident Log Entries
	case "list_incident_log_entries":
		return c.listIncidentLogEntries(ctx, inputs)

	// Users
	case "get_current_user":
		return c.getCurrentUser(ctx, inputs)

	// Services
	case "list_services":
		return c.listServices(ctx, inputs)

	// On-Calls
	case "list_oncalls":
		return c.listOnCalls(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", opName)
	}
}

// Operations returns the list of available operations.
func (c *PagerDutyIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Incidents
		{Name: "list_incidents", Description: "List incidents, optionally filtered by status or user", Category: "incidents", Tags: []string{"read", "paginated"}},
		{Name: "get_incident", Description: "Get a single incident by ID", Category: "incidents", Tags: []string{"read"}},
		{Name: "update_incident", Description: "Update an incident's status or other fields", Category: "incidents", Tags: []string{"write"}},
		{Name: "acknowledge_incident", Description: "Acknowledge an incident", Category: "incidents", Tags: []string{"write"}},
		{Name: "resolve_incident", Description: "Resolve an incident", Category: "incidents", Tags: []string{"write"}},

		// Incident Notes
		{Name: "list_incident_notes", Description: "List notes for an incident", Category: "incidents", Tags: []string{"read"}},
		{Name: "create_incident_note", Description: "Add a note to an incident", Category: "incidents", Tags: []string{"write"}},

		// Incident Log Entries
		{Name: "list_incident_log_entries", Description: "List log entries for an incident", Category: "incidents", Tags: []string{"read", "paginated"}},

		// Users
		{Name: "get_current_user", Description: "Get the current authenticated user", Category: "users", Tags: []string{"read"}},

		// Services
		{Name: "list_services", Description: "List services", Category: "services", Tags: []string{"read", "paginated"}},

		// On-Calls
		{Name: "list_oncalls", Description: "List on-call entries", Category: "oncalls", Tags: []string{"read", "paginated"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *PagerDutyIntegration) OperationSchema(opName string) *api.OperationSchema {
	return nil
}

// defaultHeaders returns default headers for PagerDuty API requests.
// PagerDuty uses "Token token=xxx" format for authentication.
func (c *PagerDutyIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}
