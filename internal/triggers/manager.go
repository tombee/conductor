package triggers

import (
	"context"
	"fmt"
	"os"

	"github.com/tombee/conductor/internal/config"
	"gopkg.in/yaml.v3"
)

// Manager handles trigger CRUD operations.
type Manager struct {
	configPath   string
	workflowsDir string
}

// NewManager creates a new trigger manager.
func NewManager(configPath, workflowsDir string) *Manager {
	return &Manager{
		configPath:   configPath,
		workflowsDir: workflowsDir,
	}
}

// loadConfig loads the config file with locking.
func (m *Manager) loadConfig(ctx context.Context) (*config.Config, *FileLock, error) {
	lock, err := AcquireLock(ctx, m.configPath)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		lock.Release()
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		lock.Release()
		return nil, nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, lock, nil
}

// saveConfig saves the config file atomically.
func (m *Manager) saveConfig(cfg *config.Config, lock *FileLock) error {
	defer lock.Release()

	if err := AtomicWriteConfig(m.configPath, cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
