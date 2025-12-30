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

package workspace

import (
	"context"
	"errors"
)

var (
	// ErrWorkspaceNotFound is returned when a workspace doesn't exist
	ErrWorkspaceNotFound = errors.New("workspace not found")

	// ErrWorkspaceExists is returned when trying to create a workspace that already exists
	ErrWorkspaceExists = errors.New("workspace already exists")

	// ErrIntegrationNotFound is returned when an integration doesn't exist
	ErrIntegrationNotFound = errors.New("integration not found")

	// ErrIntegrationExists is returned when trying to create an integration that already exists
	ErrIntegrationExists = errors.New("integration already exists")

	// ErrWorkspaceHasRuns is returned when trying to delete a workspace with active runs
	ErrWorkspaceHasRuns = errors.New("workspace has active runs")
)

// Storage defines the interface for workspace and integration persistence.
//
// Implementations:
//   - SQLite: For local CLI usage (~/.conductor/conductor.db)
//   - PostgreSQL: For controller mode with centralized configuration
//
// The storage layer handles:
//   - CRUD operations for workspaces and integrations
//   - Encryption/decryption of credentials
//   - Audit logging of credential access
//   - Transaction support for atomic operations
type Storage interface {
	// Workspace operations

	// CreateWorkspace creates a new workspace.
	// Returns ErrWorkspaceExists if a workspace with the same name already exists.
	CreateWorkspace(ctx context.Context, workspace *Workspace) error

	// GetWorkspace retrieves a workspace by name.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	GetWorkspace(ctx context.Context, name string) (*Workspace, error)

	// ListWorkspaces returns all workspaces.
	// Results are sorted by name.
	ListWorkspaces(ctx context.Context) ([]*Workspace, error)

	// UpdateWorkspace updates an existing workspace.
	// Only Description can be updated (Name is immutable).
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	UpdateWorkspace(ctx context.Context, workspace *Workspace) error

	// DeleteWorkspace deletes a workspace and all its integrations.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	// Returns ErrWorkspaceHasRuns if the workspace has active workflow runs.
	DeleteWorkspace(ctx context.Context, name string) error

	// GetCurrentWorkspace returns the name of the current active workspace.
	// Returns "default" if no workspace is set.
	GetCurrentWorkspace(ctx context.Context) (string, error)

	// SetCurrentWorkspace sets the current active workspace.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	SetCurrentWorkspace(ctx context.Context, name string) error

	// Integration operations

	// CreateIntegration creates a new integration in a workspace.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	// Returns ErrIntegrationExists if an integration with the same name already exists in the workspace.
	// The integration's credentials (auth fields) are encrypted before storage.
	CreateIntegration(ctx context.Context, integration *Integration) error

	// GetIntegration retrieves an integration by workspace and name.
	// Returns ErrIntegrationNotFound if the integration doesn't exist.
	// The integration's credentials are decrypted before return.
	GetIntegration(ctx context.Context, workspaceName, name string) (*Integration, error)

	// ListIntegrations returns all integrations in a workspace.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	// Results are sorted by name.
	// The integrations' credentials are decrypted before return.
	ListIntegrations(ctx context.Context, workspaceName string) ([]*Integration, error)

	// ListIntegrationsByType returns all integrations of a specific type in a workspace.
	// Returns ErrWorkspaceNotFound if the workspace doesn't exist.
	// Results are sorted by name.
	// The integrations' credentials are decrypted before return.
	ListIntegrationsByType(ctx context.Context, workspaceName, integrationType string) ([]*Integration, error)

	// UpdateIntegration updates an existing integration.
	// Returns ErrIntegrationNotFound if the integration doesn't exist.
	// The integration's credentials are encrypted before storage.
	UpdateIntegration(ctx context.Context, integration *Integration) error

	// DeleteIntegration deletes an integration from a workspace.
	// Returns ErrIntegrationNotFound if the integration doesn't exist.
	DeleteIntegration(ctx context.Context, workspaceName, name string) error

	// Utility operations

	// Close closes the storage connection and releases resources.
	Close() error
}

// Encryptor defines the interface for encrypting and decrypting credentials.
//
// Implementations must use AES-256-GCM authenticated encryption with keys
// derived from the system keychain or CONDUCTOR_MASTER_KEY environment variable.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns ciphertext.
	// The ciphertext includes the nonce and authentication tag.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext and returns plaintext.
	// Returns an error if authentication fails or the ciphertext is invalid.
	Decrypt(ciphertext []byte) ([]byte, error)
}
