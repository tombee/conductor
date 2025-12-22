package triggers

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/tombee/conductor/internal/config"
	"gopkg.in/yaml.v3"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// generateTriggerID generates a random ID for trigger names.
func generateTriggerID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

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

// AddFileWatcher adds a file watcher trigger.
func (m *Manager) AddFileWatcher(ctx context.Context, req CreateFileWatcherRequest) (string, error) {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return "", err
	}

	// Generate unique name
	name := fmt.Sprintf("file-%s", generateTriggerID())

	// Convert debounce duration to string
	var debounceStr string
	if req.DebounceWindow > 0 {
		debounceStr = req.DebounceWindow.String()
	}

	// Create entry
	entry := config.FileWatcherEntry{
		Name:                 name,
		Workflow:             req.Workflow,
		Paths:                []string{req.Path},
		IncludePatterns:      req.IncludePatterns,
		ExcludePatterns:      req.ExcludePatterns,
		Events:               req.Events,
		DebounceWindow:       debounceStr,
		BatchMode:            req.BatchMode,
		MaxTriggersPerMinute: req.MaxTriggersPerMinute,
		Inputs:               req.Inputs,
		Enabled:              true,
	}

	// Ensure file watchers config exists
	if cfg.Controller.FileWatchers.Watchers == nil {
		cfg.Controller.FileWatchers.Watchers = []config.FileWatcherEntry{}
	}

	// Append watcher
	cfg.Controller.FileWatchers.Watchers = append(cfg.Controller.FileWatchers.Watchers, entry)

	// Enable file watchers if not already enabled
	if !cfg.Controller.FileWatchers.Enabled {
		cfg.Controller.FileWatchers.Enabled = true
	}

	if err := m.saveConfig(cfg, lock); err != nil {
		return "", err
	}

	return name, nil
}

// ListFileWatchers lists all file watcher triggers.
func (m *Manager) ListFileWatchers(ctx context.Context) ([]FileWatcherTrigger, error) {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	var result []FileWatcherTrigger
	for _, entry := range cfg.Controller.FileWatchers.Watchers {
		// Parse debounce window back to duration
		var debounce time.Duration
		if entry.DebounceWindow != "" {
			debounce, _ = time.ParseDuration(entry.DebounceWindow)
		}

		// Get first path (for MVP we only support single path)
		path := ""
		if len(entry.Paths) > 0 {
			path = entry.Paths[0]
		}

		result = append(result, FileWatcherTrigger{
			Name:                 entry.Name,
			Path:                 path,
			Workflow:             entry.Workflow,
			Events:               entry.Events,
			IncludePatterns:      entry.IncludePatterns,
			ExcludePatterns:      entry.ExcludePatterns,
			DebounceWindow:       debounce,
			BatchMode:            entry.BatchMode,
			MaxTriggersPerMinute: entry.MaxTriggersPerMinute,
			Inputs:               entry.Inputs,
			Enabled:              entry.Enabled,
		})
	}

	return result, nil
}

// RemoveFileWatcher removes a file watcher trigger by name.
func (m *Manager) RemoveFileWatcher(ctx context.Context, name string) error {
	cfg, lock, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}

	// Find and remove the watcher
	found := false
	filtered := make([]config.FileWatcherEntry, 0, len(cfg.Controller.FileWatchers.Watchers))
	for _, entry := range cfg.Controller.FileWatchers.Watchers {
		if entry.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, entry)
	}

	if !found {
		lock.Release()
		return fmt.Errorf("file watcher %q not found", name)
	}

	cfg.Controller.FileWatchers.Watchers = filtered

	return m.saveConfig(cfg, lock)
}
