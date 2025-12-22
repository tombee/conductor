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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultPermissionExpiry is the default expiration time for stored permissions (90 days)
	DefaultPermissionExpiry = 90 * 24 * time.Hour

	// PermissionsFileName is the name of the permissions file
	PermissionsFileName = "permissions.yaml"
)

// PermissionGrant represents a stored permission grant for a workflow.
type PermissionGrant struct {
	// WorkflowHash is the content hash of the workflow
	WorkflowHash string `yaml:"workflow_hash" json:"workflow_hash"`

	// WorkflowName is a human-readable identifier
	WorkflowName string `yaml:"workflow_name" json:"workflow_name"`

	// GrantedAt is when the permission was granted
	GrantedAt time.Time `yaml:"granted_at" json:"granted_at"`

	// ExpiresAt is when the permission expires
	ExpiresAt time.Time `yaml:"expires_at" json:"expires_at"`

	// Permissions describes what was granted
	Permissions GrantedPermissions `yaml:"permissions" json:"permissions"`

	// Revoked indicates if this grant was explicitly revoked
	Revoked bool `yaml:"revoked,omitempty" json:"revoked,omitempty"`
}

// GrantedPermissions describes the permissions that were granted.
type GrantedPermissions struct {
	Filesystem *FilesystemPermissions `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	Network    *NetworkPermissions    `yaml:"network,omitempty" json:"network,omitempty"`
	Commands   *CommandPermissions    `yaml:"commands,omitempty" json:"commands,omitempty"`
}

// PermissionStore manages persistent permission grants.
type PermissionStore interface {
	// Grant stores a permission grant
	Grant(workflowContent string, workflowName string, perms GrantedPermissions) error

	// Revoke revokes a permission grant by workflow hash
	Revoke(workflowHash string) error

	// Check checks if a workflow has a valid permission grant
	Check(workflowContent string) (*PermissionGrant, bool)

	// List returns all stored permission grants
	List() ([]*PermissionGrant, error)

	// Cleanup removes expired grants
	Cleanup() error
}

// FilePermissionStore implements PermissionStore using a YAML file.
type FilePermissionStore struct {
	mu       sync.RWMutex
	filePath string
	grants   map[string]*PermissionGrant
}

// NewFilePermissionStore creates a new file-based permission store.
// If configDir is empty, uses ~/.config/conductor/
func NewFilePermissionStore(configDir string) (*FilePermissionStore, error) {
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config", "conductor")
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	filePath := filepath.Join(configDir, PermissionsFileName)

	store := &FilePermissionStore{
		filePath: filePath,
		grants:   make(map[string]*PermissionGrant),
	}

	// Load existing grants if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load existing permissions: %w", err)
		}
	}

	return store, nil
}

// Grant stores a new permission grant.
func (s *FilePermissionStore) Grant(workflowContent string, workflowName string, perms GrantedPermissions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := computeWorkflowHash(workflowContent)
	now := time.Now()

	grant := &PermissionGrant{
		WorkflowHash: hash,
		WorkflowName: workflowName,
		GrantedAt:    now,
		ExpiresAt:    now.Add(DefaultPermissionExpiry),
		Permissions:  perms,
		Revoked:      false,
	}

	s.grants[hash] = grant

	return s.save()
}

// Revoke revokes a permission grant.
func (s *FilePermissionStore) Revoke(workflowHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	grant, exists := s.grants[workflowHash]
	if !exists {
		return fmt.Errorf("no permission grant found for hash: %s", workflowHash)
	}

	grant.Revoked = true

	return s.save()
}

// Check checks if a workflow has a valid permission grant.
func (s *FilePermissionStore) Check(workflowContent string) (*PermissionGrant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash := computeWorkflowHash(workflowContent)
	grant, exists := s.grants[hash]

	if !exists {
		return nil, false
	}

	// Check if revoked
	if grant.Revoked {
		return nil, false
	}

	// Check if expired
	if time.Now().After(grant.ExpiresAt) {
		return nil, false
	}

	return grant, true
}

// List returns all stored permission grants.
func (s *FilePermissionStore) List() ([]*PermissionGrant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	grants := make([]*PermissionGrant, 0, len(s.grants))
	for _, grant := range s.grants {
		grants = append(grants, grant)
	}

	return grants, nil
}

// Cleanup removes expired and revoked grants.
func (s *FilePermissionStore) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleaned := false

	for hash, grant := range s.grants {
		// Remove if expired or revoked
		if grant.Revoked || now.After(grant.ExpiresAt) {
			delete(s.grants, hash)
			cleaned = true
		}
	}

	if cleaned {
		return s.save()
	}

	return nil
}

// load reads grants from disk.
func (s *FilePermissionStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read permissions file: %w", err)
	}

	var grants map[string]*PermissionGrant
	if err := yaml.Unmarshal(data, &grants); err != nil {
		return fmt.Errorf("failed to parse permissions file: %w", err)
	}

	s.grants = grants
	return nil
}

// save writes grants to disk.
func (s *FilePermissionStore) save() error {
	data, err := yaml.Marshal(s.grants)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	// Write with restricted permissions (owner only)
	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write permissions file: %w", err)
	}

	return nil
}

// computeWorkflowHash computes a SHA256 hash of the workflow content.
// This is used to detect workflow modifications.
func computeWorkflowHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
