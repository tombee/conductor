package triggers

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/config"
)

// AddEndpoint adds a new endpoint trigger to the configuration.
func (m *Manager) AddEndpoint(ctx context.Context, req CreateEndpointRequest) error {
	// Validate inputs
	if err := ValidateWorkflowExists(m.workflowsDir, req.Workflow); err != nil {
		return err
	}

	if req.Name == "" {
		return fmt.Errorf("endpoint name cannot be empty")
	}

	if err := ValidateSecretRef(req.Secret); err != nil {
		return err
	}

	// Load config with lock
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Enable endpoints if not already configured
	cfg.Controller.Endpoints.Enabled = true

	// Initialize endpoints section if needed
	if cfg.Controller.Endpoints.Endpoints == nil {
		cfg.Controller.Endpoints.Endpoints = []config.EndpointEntry{}
	}

	// Check for duplicate name
	for _, endpoint := range cfg.Controller.Endpoints.Endpoints {
		if endpoint.Name == req.Name {
			lock.Release()
			return fmt.Errorf("endpoint name already exists: %s", req.Name)
		}
	}

	// Add new endpoint
	newEndpoint := config.EndpointEntry{
		Name:        req.Name,
		Description: req.Description,
		Workflow:    req.Workflow,
		Inputs:      req.Inputs,
		Scopes:      req.Scopes,
		RateLimit:   req.RateLimit,
		Timeout:     req.Timeout,
	}
	cfg.Controller.Endpoints.Endpoints = append(cfg.Controller.Endpoints.Endpoints, newEndpoint)

	// Save config
	return m.saveConfig(cfg, lock)
}

// ListEndpoints returns all configured endpoint triggers.
func (m *Manager) ListEndpoints(ctx context.Context) ([]EndpointTrigger, error) {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	var triggers []EndpointTrigger
	for _, endpoint := range cfg.Controller.Endpoints.Endpoints {
		triggers = append(triggers, EndpointTrigger{
			Name:        endpoint.Name,
			Description: endpoint.Description,
			Workflow:    endpoint.Workflow,
			Inputs:      endpoint.Inputs,
			Scopes:      endpoint.Scopes,
			RateLimit:   endpoint.RateLimit,
			Timeout:     endpoint.Timeout,
		})
	}

	return triggers, nil
}

// RemoveEndpoint removes an endpoint trigger by name.
func (m *Manager) RemoveEndpoint(ctx context.Context, name string) error {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Find and remove endpoint
	found := false
	newEndpoints := make([]config.EndpointEntry, 0, len(cfg.Controller.Endpoints.Endpoints))
	for _, endpoint := range cfg.Controller.Endpoints.Endpoints {
		if endpoint.Name == name {
			found = true
			continue
		}
		newEndpoints = append(newEndpoints, endpoint)
	}

	if !found {
		lock.Release()
		return fmt.Errorf("endpoint not found: %s", name)
	}

	cfg.Controller.Endpoints.Endpoints = newEndpoints

	return m.saveConfig(cfg, lock)
}
