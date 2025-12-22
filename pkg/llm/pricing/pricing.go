package pricing

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ModelPricing contains pricing information for a specific model.
type ModelPricing struct {
	// Provider is the LLM provider name (e.g., "anthropic", "openai").
	Provider string `yaml:"provider" json:"provider"`

	// Model is the model identifier (e.g., "claude-3-opus-20240229").
	Model string `yaml:"model" json:"model"`

	// InputPricePerMillion is the cost per million input tokens in USD.
	InputPricePerMillion float64 `yaml:"input_price_per_million" json:"input_price_per_million"`

	// OutputPricePerMillion is the cost per million output tokens in USD.
	OutputPricePerMillion float64 `yaml:"output_price_per_million" json:"output_price_per_million"`

	// CacheCreationPricePerMillion is the cost per million cache creation tokens in USD.
	// Zero if cache not supported.
	CacheCreationPricePerMillion float64 `yaml:"cache_creation_price_per_million,omitempty" json:"cache_creation_price_per_million,omitempty"`

	// CacheReadPricePerMillion is the cost per million cache read tokens in USD.
	// Zero if cache not supported.
	CacheReadPricePerMillion float64 `yaml:"cache_read_price_per_million,omitempty" json:"cache_read_price_per_million,omitempty"`

	// EffectiveDate is when this pricing became effective.
	EffectiveDate time.Time `yaml:"effective_date" json:"effective_date"`

	// IsSubscription indicates if this is a subscription-based model (no per-token cost).
	IsSubscription bool `yaml:"is_subscription,omitempty" json:"is_subscription,omitempty"`
}

// PricingConfig contains all pricing information.
type PricingConfig struct {
	// Version is the pricing configuration version.
	Version string `yaml:"version" json:"version"`

	// UpdatedAt is when this configuration was last updated.
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// Models contains pricing for all models.
	Models []ModelPricing `yaml:"models" json:"models"`
}

// PricingManager manages pricing lookups with caching and staleness warnings.
type PricingManager struct {
	mu     sync.RWMutex
	config *PricingConfig

	// configPath is the path to user pricing configuration.
	configPath string

	// stalenessThreshold is how old pricing can be before warning (default: 30 days).
	stalenessThreshold time.Duration
}

// NewPricingManager creates a new pricing manager with built-in defaults.
func NewPricingManager() *PricingManager {
	return &PricingManager{
		config:             getBuiltInPricing(),
		stalenessThreshold: 30 * 24 * time.Hour, // 30 days
	}
}

// NewPricingManagerWithConfig creates a pricing manager and loads user config if available.
func NewPricingManagerWithConfig(configPath string) (*PricingManager, error) {
	pm := NewPricingManager()
	pm.configPath = configPath

	// Try to load user config, but don't fail if it doesn't exist
	if err := pm.LoadUserConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load pricing config: %w", err)
	}

	return pm, nil
}

// LoadUserConfig loads pricing configuration from the user's config file.
// If the file doesn't exist, this is not an error - built-in defaults will be used.
func (pm *PricingManager) LoadUserConfig() error {
	if pm.configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		pm.configPath = filepath.Join(home, ".conductor", "pricing.yaml")
	}

	data, err := os.ReadFile(pm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No user config, use built-in defaults
			return nil
		}
		return fmt.Errorf("failed to read pricing config: %w", err)
	}

	var config PricingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse pricing config: %w", err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Merge user config with built-in defaults
	pm.config = pm.mergePricing(pm.config, &config)

	return nil
}

// mergePricing merges user pricing with built-in defaults.
// User pricing takes precedence for matching provider/model combinations.
func (pm *PricingManager) mergePricing(builtIn, user *PricingConfig) *PricingConfig {
	merged := &PricingConfig{
		Version:   user.Version,
		UpdatedAt: user.UpdatedAt,
		Models:    make([]ModelPricing, 0),
	}

	// Create lookup map for user pricing
	userPricing := make(map[string]ModelPricing)
	for _, mp := range user.Models {
		key := fmt.Sprintf("%s:%s", mp.Provider, mp.Model)
		userPricing[key] = mp
	}

	// Start with all built-in pricing
	for _, mp := range builtIn.Models {
		key := fmt.Sprintf("%s:%s", mp.Provider, mp.Model)
		if userMP, exists := userPricing[key]; exists {
			// Use user override
			merged.Models = append(merged.Models, userMP)
			delete(userPricing, key) // Mark as processed
		} else {
			// Use built-in
			merged.Models = append(merged.Models, mp)
		}
	}

	// Add any user pricing not in built-in
	for _, mp := range userPricing {
		merged.Models = append(merged.Models, mp)
	}

	return merged
}

// GetPricing returns pricing for a specific provider and model.
// Returns nil if pricing not found.
func (pm *PricingManager) GetPricing(provider, model string) *ModelPricing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for i := range pm.config.Models {
		mp := &pm.config.Models[i]
		if mp.Provider == provider && mp.Model == model {
			return mp
		}
	}

	return nil
}

// GetPricingWithWarning returns pricing and a staleness warning if applicable.
func (pm *PricingManager) GetPricingWithWarning(provider, model string) (*ModelPricing, string) {
	pricing := pm.GetPricing(provider, model)
	if pricing == nil {
		return nil, ""
	}

	// Check staleness
	age := time.Since(pricing.EffectiveDate)
	if age > pm.stalenessThreshold {
		days := int(age.Hours() / 24)
		warning := fmt.Sprintf("pricing data is %d days old - consider updating with 'conductor config update-pricing'", days)
		return pricing, warning
	}

	return pricing, ""
}

// ListProviders returns all providers with pricing data.
func (pm *PricingManager) ListProviders() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	seen := make(map[string]bool)
	providers := make([]string, 0)

	for _, mp := range pm.config.Models {
		if !seen[mp.Provider] {
			seen[mp.Provider] = true
			providers = append(providers, mp.Provider)
		}
	}

	return providers
}

// ListModels returns all models for a specific provider.
func (pm *PricingManager) ListModels(provider string) []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	models := make([]string, 0)
	for _, mp := range pm.config.Models {
		if mp.Provider == provider {
			models = append(models, mp.Model)
		}
	}

	return models
}

// SetStalenessThreshold sets the duration after which pricing is considered stale.
func (pm *PricingManager) SetStalenessThreshold(d time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.stalenessThreshold = d
}

// GetConfig returns a copy of the current pricing configuration.
func (pm *PricingManager) GetConfig() PricingConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	config := *pm.config
	config.Models = make([]ModelPricing, len(pm.config.Models))
	copy(config.Models, pm.config.Models)

	return config
}
