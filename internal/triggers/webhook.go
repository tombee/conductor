package triggers

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/config"
)

// AddWebhook adds a new webhook trigger to the configuration.
func (m *Manager) AddWebhook(ctx context.Context, req CreateWebhookRequest) error {
	// Validate inputs
	if err := ValidateWorkflowExists(m.workflowsDir, req.Workflow); err != nil {
		return err
	}

	if req.Path == "" {
		return fmt.Errorf("webhook path cannot be empty")
	}

	if req.Source == "" {
		return fmt.Errorf("webhook source cannot be empty")
	}

	if err := ValidateSecretRef(req.Secret); err != nil {
		return err
	}

	for key, value := range req.InputMapping {
		if err := ValidateJSONPath(value); err != nil {
			return fmt.Errorf("input mapping for %s: %w", key, err)
		}
	}

	// Load config with lock
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Initialize webhooks section if needed
	if cfg.Controller.Webhooks.Routes == nil {
		cfg.Controller.Webhooks.Routes = []config.WebhookRoute{}
	}

	// Check for duplicate path
	for _, route := range cfg.Controller.Webhooks.Routes {
		if route.Path == req.Path {
			lock.Release()
			return fmt.Errorf("webhook path already exists: %s", req.Path)
		}
	}

	// Add new webhook
	newRoute := config.WebhookRoute{
		Path:         req.Path,
		Source:       req.Source,
		Workflow:     req.Workflow,
		Events:       req.Events,
		Secret:       req.Secret,
		InputMapping: req.InputMapping,
	}
	cfg.Controller.Webhooks.Routes = append(cfg.Controller.Webhooks.Routes, newRoute)

	// Save config
	return m.saveConfig(cfg, lock)
}

// ListWebhooks returns all configured webhook triggers.
func (m *Manager) ListWebhooks(ctx context.Context) ([]WebhookTrigger, error) {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	var triggers []WebhookTrigger
	for _, route := range cfg.Controller.Webhooks.Routes {
		triggers = append(triggers, WebhookTrigger{
			Path:         route.Path,
			Source:       route.Source,
			Workflow:     route.Workflow,
			Events:       route.Events,
			Secret:       route.Secret,
			InputMapping: route.InputMapping,
		})
	}

	return triggers, nil
}

// RemoveWebhook removes a webhook trigger by path.
func (m *Manager) RemoveWebhook(ctx context.Context, path string) error {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Find and remove webhook
	found := false
	newRoutes := make([]config.WebhookRoute, 0, len(cfg.Controller.Webhooks.Routes))
	for _, route := range cfg.Controller.Webhooks.Routes {
		if route.Path == path {
			found = true
			continue
		}
		newRoutes = append(newRoutes, route)
	}

	if !found {
		lock.Release()
		return fmt.Errorf("webhook not found: %s", path)
	}

	cfg.Controller.Webhooks.Routes = newRoutes

	return m.saveConfig(cfg, lock)
}
