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

package security

import (
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultOverrideDuration is how long an override lasts (1 hour)
	DefaultOverrideDuration = 1 * time.Hour
)

// OverrideType represents the type of security override.
type OverrideType string

const (
	// OverrideDisableEnforcement disables security enforcement entirely
	OverrideDisableEnforcement OverrideType = "disable-enforcement"

	// OverrideDisableSandbox disables container sandboxing
	OverrideDisableSandbox OverrideType = "disable-sandbox"

	// OverrideDisableAudit disables audit logging
	OverrideDisableAudit OverrideType = "disable-audit"
)

// Override represents an active security override.
type Override struct {
	Type      OverrideType
	Reason    string
	AppliedBy string
	AppliedAt time.Time
	ExpiresAt time.Time
}

// IsActive returns whether the override is still active.
func (o *Override) IsActive() bool {
	return time.Now().Before(o.ExpiresAt)
}

// OverrideManager manages security overrides.
type OverrideManager struct {
	mu        sync.RWMutex
	overrides map[OverrideType]*Override
	logger    EventLogger
}

// NewOverrideManager creates a new override manager.
func NewOverrideManager(logger EventLogger) *OverrideManager {
	return &OverrideManager{
		overrides: make(map[OverrideType]*Override),
		logger:    logger,
	}
}

// Apply applies a security override with the given reason.
// Uses DefaultOverrideDuration for TTL.
func (m *OverrideManager) Apply(overrideType OverrideType, reason, appliedBy string) (*Override, error) {
	return m.ApplyWithTTL(overrideType, reason, appliedBy, DefaultOverrideDuration)
}

// ApplyWithTTL applies a security override with a custom time-to-live.
func (m *OverrideManager) ApplyWithTTL(overrideType OverrideType, reason, appliedBy string, ttl time.Duration) (*Override, error) {
	if reason == "" {
		return nil, fmt.Errorf("override reason is required")
	}

	if appliedBy == "" {
		return nil, fmt.Errorf("override applied_by is required")
	}

	if ttl <= 0 {
		return nil, fmt.Errorf("override TTL must be positive")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	override := &Override{
		Type:      overrideType,
		Reason:    reason,
		AppliedBy: appliedBy,
		AppliedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	m.overrides[overrideType] = override

	// Log the override application
	if m.logger != nil {
		m.logger.Log(SecurityEvent{
			Timestamp: now,
			EventType: EventType("security_override_applied"),
			Decision:  "override",
			Reason:    reason,
			Resource:  string(overrideType),
			UserID:    appliedBy,
		})
	}

	return override, nil
}

// IsActive checks if a specific override type is active.
func (m *OverrideManager) IsActive(overrideType OverrideType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	override, exists := m.overrides[overrideType]
	if !exists {
		return false
	}

	return override.IsActive()
}

// Get returns the override for the given type, if active.
func (m *OverrideManager) Get(overrideType OverrideType) (*Override, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	override, exists := m.overrides[overrideType]
	if !exists || !override.IsActive() {
		return nil, false
	}

	return override, true
}

// Revoke manually revokes an override before it expires.
func (m *OverrideManager) Revoke(overrideType OverrideType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	override, exists := m.overrides[overrideType]
	if !exists {
		return fmt.Errorf("no override found for type: %s", overrideType)
	}

	delete(m.overrides, overrideType)

	// Log the revocation
	if m.logger != nil {
		m.logger.Log(SecurityEvent{
			Timestamp: time.Now(),
			EventType: EventType("security_override_revoked"),
			Decision:  "revoked",
			Reason:    fmt.Sprintf("override for %s revoked", overrideType),
			Resource:  string(overrideType),
		})
	}

	// If override was still active, log warning
	if override.IsActive() {
		if m.logger != nil {
			m.logger.Log(SecurityEvent{
				Timestamp: time.Now(),
				EventType: EventType("security_override_revoked_early"),
				Decision:  "warning",
				Reason:    fmt.Sprintf("override for %s revoked before expiry", overrideType),
				Resource:  string(overrideType),
			})
		}
	}

	return nil
}

// Cleanup removes expired overrides.
func (m *OverrideManager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for typ, override := range m.overrides {
		if now.After(override.ExpiresAt) {
			delete(m.overrides, typ)

			// Log automatic expiry
			if m.logger != nil {
				m.logger.Log(SecurityEvent{
					Timestamp: now,
					EventType: EventType("security_override_expired"),
					Decision:  "expired",
					Reason:    fmt.Sprintf("override for %s expired after %v", typ, DefaultOverrideDuration),
					Resource:  string(typ),
				})
			}
		}
	}
}

// List returns all active overrides.
func (m *OverrideManager) List() []*Override {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Override
	for _, override := range m.overrides {
		if override.IsActive() {
			result = append(result, override)
		}
	}

	return result
}

// GetActive is an alias for List, returns all active overrides.
func (m *OverrideManager) GetActive() []*Override {
	return m.List()
}

// StartAutoCleanup starts a background goroutine that cleans up expired overrides.
func (m *OverrideManager) StartAutoCleanup(stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-stop:
			return
		}
	}
}
