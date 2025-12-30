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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStorage implements Storage using SQLite for local CLI usage.
//
// Database location: ~/.conductor/conductor.db
//
// Features:
//   - WAL mode for better concurrency
//   - Foreign key constraints enabled
//   - AES-256-GCM encryption for credentials
//   - Automatic default workspace creation
type SQLiteStorage struct {
	db        *sql.DB
	encryptor Encryptor
}

// SQLiteConfig contains configuration for SQLite storage.
type SQLiteConfig struct {
	// Path is the filesystem path to the SQLite database file
	// Example: /Users/user/.conductor/conductor.db
	Path string

	// Encryptor handles encryption/decryption of credentials
	// Required - credentials must always be encrypted at rest
	Encryptor Encryptor
}

// NewSQLiteStorage creates a new SQLite storage backend.
//
// The database is created if it doesn't exist, and migrations are run automatically.
// The "default" workspace is created if it doesn't exist.
func NewSQLiteStorage(cfg SQLiteConfig) (*SQLiteStorage, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("database path is required")
	}

	if cfg.Encryptor == nil {
		return nil, fmt.Errorf("encryptor is required")
	}

	// SQLite connection string with WAL mode for better concurrency
	connStr := cfg.Path + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON"

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	// SQLite with WAL mode can handle multiple concurrent readers
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	storage := &SQLiteStorage{
		db:        db,
		encryptor: cfg.Encryptor,
	}

	// Run migrations
	if err := storage.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Ensure default workspace exists
	if err := storage.ensureDefaultWorkspace(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create default workspace: %w", err)
	}

	return storage, nil
}

// migrate creates the database schema.
func (s *SQLiteStorage) migrate(ctx context.Context) error {
	migrations := []string{
		// Workspaces table
		`CREATE TABLE IF NOT EXISTS workspaces (
			name TEXT PRIMARY KEY,
			description TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		// Integrations table
		`CREATE TABLE IF NOT EXISTS integrations (
			id TEXT PRIMARY KEY,
			workspace_name TEXT NOT NULL REFERENCES workspaces(name) ON DELETE CASCADE,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			base_url TEXT,
			auth_type TEXT NOT NULL,
			auth_encrypted BLOB,
			headers_json TEXT,
			timeout_seconds INTEGER DEFAULT 30,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(workspace_name, name)
		)`,

		// Config table for storing current workspace and other settings
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,

		// Indexes for efficient queries
		`CREATE INDEX IF NOT EXISTS idx_integrations_workspace
			ON integrations(workspace_name)`,
		`CREATE INDEX IF NOT EXISTS idx_integrations_type
			ON integrations(type)`,
		`CREATE INDEX IF NOT EXISTS idx_integrations_workspace_type
			ON integrations(workspace_name, type)`,
	}

	for _, migration := range migrations {
		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// ensureDefaultWorkspace creates the default workspace if it doesn't exist.
func (s *SQLiteStorage) ensureDefaultWorkspace(ctx context.Context) error {
	workspace := &Workspace{
		Name:        "default",
		Description: "Default workspace",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Try to create - ignore error if already exists
	err := s.CreateWorkspace(ctx, workspace)
	if err != nil && !errors.Is(err, ErrWorkspaceExists) {
		return err
	}

	return nil
}

// CreateWorkspace creates a new workspace.
func (s *SQLiteStorage) CreateWorkspace(ctx context.Context, workspace *Workspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}

	if workspace.Name == "" {
		return fmt.Errorf("workspace name is required")
	}

	now := time.Now()
	workspace.CreatedAt = now
	workspace.UpdatedAt = now

	query := `INSERT INTO workspaces (name, description, created_at, updated_at)
	          VALUES (?, ?, ?, ?)`

	_, err := s.db.ExecContext(ctx, query,
		workspace.Name,
		workspace.Description,
		workspace.CreatedAt.Format(time.RFC3339),
		workspace.UpdatedAt.Format(time.RFC3339),
	)

	if err != nil {
		if isSQLiteUniqueConstraintError(err) {
			return ErrWorkspaceExists
		}
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	return nil
}

// GetWorkspace retrieves a workspace by name.
func (s *SQLiteStorage) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	query := `SELECT name, description, created_at, updated_at
	          FROM workspaces WHERE name = ?`

	var workspace Workspace
	var createdAt, updatedAt string

	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&workspace.Name,
		&workspace.Description,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Parse timestamps
	workspace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	workspace.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &workspace, nil
}

// ListWorkspaces returns all workspaces sorted by name.
func (s *SQLiteStorage) ListWorkspaces(ctx context.Context) ([]*Workspace, error) {
	query := `SELECT name, description, created_at, updated_at
	          FROM workspaces ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []*Workspace
	for rows.Next() {
		var workspace Workspace
		var createdAt, updatedAt string

		if err := rows.Scan(
			&workspace.Name,
			&workspace.Description,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %w", err)
		}

		// Parse timestamps
		workspace.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		workspace.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		workspaces = append(workspaces, &workspace)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating workspaces: %w", err)
	}

	return workspaces, nil
}

// UpdateWorkspace updates an existing workspace.
func (s *SQLiteStorage) UpdateWorkspace(ctx context.Context, workspace *Workspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}

	workspace.UpdatedAt = time.Now()

	query := `UPDATE workspaces
	          SET description = ?, updated_at = ?
	          WHERE name = ?`

	result, err := s.db.ExecContext(ctx, query,
		workspace.Description,
		workspace.UpdatedAt.Format(time.RFC3339),
		workspace.Name,
	)

	if err != nil {
		return fmt.Errorf("failed to update workspace: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}

// DeleteWorkspace deletes a workspace and all its integrations.
func (s *SQLiteStorage) DeleteWorkspace(ctx context.Context, name string) error {
	// Check if workspace exists
	_, err := s.GetWorkspace(ctx, name)
	if err != nil {
		return err
	}

	// Cannot delete default workspace
	if name == "default" {
		return fmt.Errorf("cannot delete default workspace")
	}

	// Delete workspace (CASCADE will delete integrations)
	query := `DELETE FROM workspaces WHERE name = ?`

	result, err := s.db.ExecContext(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}

// GetCurrentWorkspace returns the name of the current active workspace.
func (s *SQLiteStorage) GetCurrentWorkspace(ctx context.Context) (string, error) {
	query := `SELECT value FROM config WHERE key = ?`

	var workspaceName string
	err := s.db.QueryRowContext(ctx, query, "current_workspace").Scan(&workspaceName)

	if err == sql.ErrNoRows {
		// No current workspace set, return default
		return "default", nil
	}

	if err != nil {
		return "", fmt.Errorf("failed to get current workspace: %w", err)
	}

	return workspaceName, nil
}

// SetCurrentWorkspace sets the current active workspace.
func (s *SQLiteStorage) SetCurrentWorkspace(ctx context.Context, name string) error {
	// Verify workspace exists
	_, err := s.GetWorkspace(ctx, name)
	if err != nil {
		return err
	}

	// Upsert current workspace setting
	query := `INSERT INTO config (key, value) VALUES (?, ?)
	          ON CONFLICT(key) DO UPDATE SET value = excluded.value`

	_, err = s.db.ExecContext(ctx, query, "current_workspace", name)
	if err != nil {
		return fmt.Errorf("failed to set current workspace: %w", err)
	}

	return nil
}

// CreateIntegration creates a new integration in a workspace.
func (s *SQLiteStorage) CreateIntegration(ctx context.Context, integration *Integration) error {
	if integration == nil {
		return fmt.Errorf("integration cannot be nil")
	}

	if integration.WorkspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}

	if integration.Name == "" {
		return fmt.Errorf("integration name is required")
	}

	if integration.Type == "" {
		return fmt.Errorf("integration type is required")
	}

	// Check workspace exists
	_, err := s.GetWorkspace(ctx, integration.WorkspaceName)
	if err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			return ErrWorkspaceNotFound
		}
		return err
	}

	// Generate ID if not provided
	if integration.ID == "" {
		integration.ID = uuid.New().String()
	}

	now := time.Now()
	integration.CreatedAt = now
	integration.UpdatedAt = now

	// Encrypt auth credentials
	authEncrypted, err := s.encryptAuth(&integration.Auth)
	if err != nil {
		return fmt.Errorf("failed to encrypt auth: %w", err)
	}

	// Serialize headers to JSON
	headersJSON := ""
	if len(integration.Headers) > 0 {
		headersBytes, err := json.Marshal(integration.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
		headersJSON = string(headersBytes)
	}

	query := `INSERT INTO integrations
	          (id, workspace_name, name, type, base_url, auth_type, auth_encrypted,
	           headers_json, timeout_seconds, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, query,
		integration.ID,
		integration.WorkspaceName,
		integration.Name,
		integration.Type,
		integration.BaseURL,
		string(integration.Auth.Type),
		authEncrypted,
		headersJSON,
		integration.TimeoutSeconds,
		integration.CreatedAt.Format(time.RFC3339),
		integration.UpdatedAt.Format(time.RFC3339),
	)

	if err != nil {
		if isSQLiteUniqueConstraintError(err) {
			return ErrIntegrationExists
		}
		if isSQLiteForeignKeyError(err) {
			return ErrWorkspaceNotFound
		}
		return fmt.Errorf("failed to create integration: %w", err)
	}

	return nil
}

// GetIntegration retrieves an integration by workspace and name.
func (s *SQLiteStorage) GetIntegration(ctx context.Context, workspaceName, name string) (*Integration, error) {
	query := `SELECT id, workspace_name, name, type, base_url, auth_type, auth_encrypted,
	                 headers_json, timeout_seconds, created_at, updated_at
	          FROM integrations
	          WHERE workspace_name = ? AND name = ?`

	var integration Integration
	var authType string
	var authEncrypted []byte
	var headersJSON string
	var createdAt, updatedAt string

	err := s.db.QueryRowContext(ctx, query, workspaceName, name).Scan(
		&integration.ID,
		&integration.WorkspaceName,
		&integration.Name,
		&integration.Type,
		&integration.BaseURL,
		&authType,
		&authEncrypted,
		&headersJSON,
		&integration.TimeoutSeconds,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrIntegrationNotFound
		}
		return nil, fmt.Errorf("failed to get integration: %w", err)
	}

	// Parse timestamps
	integration.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	integration.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Set auth type
	integration.Auth.Type = AuthType(authType)

	// Decrypt auth credentials
	if len(authEncrypted) > 0 {
		auth, err := s.decryptAuth(authEncrypted, integration.Auth.Type)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt auth: %w", err)
		}
		integration.Auth = *auth
	}

	// Deserialize headers
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &integration.Headers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
		}
	}

	return &integration, nil
}

// ListIntegrations returns all integrations in a workspace sorted by name.
func (s *SQLiteStorage) ListIntegrations(ctx context.Context, workspaceName string) ([]*Integration, error) {
	// Check workspace exists
	_, err := s.GetWorkspace(ctx, workspaceName)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, workspace_name, name, type, base_url, auth_type, auth_encrypted,
	                 headers_json, timeout_seconds, created_at, updated_at
	          FROM integrations
	          WHERE workspace_name = ?
	          ORDER BY name`

	return s.queryIntegrations(ctx, query, workspaceName)
}

// ListIntegrationsByType returns all integrations of a specific type in a workspace.
func (s *SQLiteStorage) ListIntegrationsByType(ctx context.Context, workspaceName, integrationType string) ([]*Integration, error) {
	// Check workspace exists
	_, err := s.GetWorkspace(ctx, workspaceName)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, workspace_name, name, type, base_url, auth_type, auth_encrypted,
	                 headers_json, timeout_seconds, created_at, updated_at
	          FROM integrations
	          WHERE workspace_name = ? AND type = ?
	          ORDER BY name`

	return s.queryIntegrations(ctx, query, workspaceName, integrationType)
}

// queryIntegrations is a helper to execute integration queries.
func (s *SQLiteStorage) queryIntegrations(ctx context.Context, query string, args ...interface{}) ([]*Integration, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query integrations: %w", err)
	}
	defer rows.Close()

	var integrations []*Integration
	for rows.Next() {
		var integration Integration
		var authType string
		var authEncrypted []byte
		var headersJSON string
		var createdAt, updatedAt string

		if err := rows.Scan(
			&integration.ID,
			&integration.WorkspaceName,
			&integration.Name,
			&integration.Type,
			&integration.BaseURL,
			&authType,
			&authEncrypted,
			&headersJSON,
			&integration.TimeoutSeconds,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan integration: %w", err)
		}

		// Parse timestamps
		integration.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		integration.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		// Set auth type
		integration.Auth.Type = AuthType(authType)

		// Decrypt auth credentials
		if len(authEncrypted) > 0 {
			auth, err := s.decryptAuth(authEncrypted, integration.Auth.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt auth: %w", err)
			}
			integration.Auth = *auth
		}

		// Deserialize headers
		if headersJSON != "" {
			if err := json.Unmarshal([]byte(headersJSON), &integration.Headers); err != nil {
				return nil, fmt.Errorf("failed to unmarshal headers: %w", err)
			}
		}

		integrations = append(integrations, &integration)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating integrations: %w", err)
	}

	return integrations, nil
}

// UpdateIntegration updates an existing integration.
func (s *SQLiteStorage) UpdateIntegration(ctx context.Context, integration *Integration) error {
	if integration == nil {
		return fmt.Errorf("integration cannot be nil")
	}

	integration.UpdatedAt = time.Now()

	// Encrypt auth credentials
	authEncrypted, err := s.encryptAuth(&integration.Auth)
	if err != nil {
		return fmt.Errorf("failed to encrypt auth: %w", err)
	}

	// Serialize headers to JSON
	headersJSON := ""
	if len(integration.Headers) > 0 {
		headersBytes, err := json.Marshal(integration.Headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
		headersJSON = string(headersBytes)
	}

	query := `UPDATE integrations
	          SET type = ?, base_url = ?, auth_type = ?, auth_encrypted = ?,
	              headers_json = ?, timeout_seconds = ?, updated_at = ?
	          WHERE workspace_name = ? AND name = ?`

	result, err := s.db.ExecContext(ctx, query,
		integration.Type,
		integration.BaseURL,
		string(integration.Auth.Type),
		authEncrypted,
		headersJSON,
		integration.TimeoutSeconds,
		integration.UpdatedAt.Format(time.RFC3339),
		integration.WorkspaceName,
		integration.Name,
	)

	if err != nil {
		return fmt.Errorf("failed to update integration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrIntegrationNotFound
	}

	return nil
}

// DeleteIntegration deletes an integration from a workspace.
func (s *SQLiteStorage) DeleteIntegration(ctx context.Context, workspaceName, name string) error {
	query := `DELETE FROM integrations WHERE workspace_name = ? AND name = ?`

	result, err := s.db.ExecContext(ctx, query, workspaceName, name)
	if err != nil {
		return fmt.Errorf("failed to delete integration: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrIntegrationNotFound
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// encryptAuth encrypts authentication credentials.
func (s *SQLiteStorage) encryptAuth(auth *AuthConfig) ([]byte, error) {
	// Serialize auth config (only credential fields)
	authData := map[string]string{}

	switch auth.Type {
	case AuthTypeToken:
		if auth.Token != "" {
			authData["token"] = auth.Token
		}
	case AuthTypeBasic:
		if auth.Username != "" {
			authData["username"] = auth.Username
		}
		if auth.Password != "" {
			authData["password"] = auth.Password
		}
	case AuthTypeAPIKey:
		if auth.APIKeyHeader != "" {
			authData["api_key_header"] = auth.APIKeyHeader
		}
		if auth.APIKeyValue != "" {
			authData["api_key_value"] = auth.APIKeyValue
		}
	case AuthTypeNone:
		// No credentials to encrypt
		return nil, nil
	}

	if len(authData) == 0 {
		return nil, nil
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(authData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth data: %w", err)
	}

	// Encrypt
	encrypted, err := s.encryptor.Encrypt(jsonData)
	if err != nil {
		return nil, err
	}

	return encrypted, nil
}

// decryptAuth decrypts authentication credentials.
func (s *SQLiteStorage) decryptAuth(encrypted []byte, authType AuthType) (*AuthConfig, error) {
	if len(encrypted) == 0 {
		return &AuthConfig{Type: authType}, nil
	}

	// Decrypt
	jsonData, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}

	// Unmarshal from JSON
	var authData map[string]string
	if err := json.Unmarshal(jsonData, &authData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth data: %w", err)
	}

	// Reconstruct auth config
	auth := &AuthConfig{Type: authType}

	switch authType {
	case AuthTypeToken:
		auth.Token = authData["token"]
	case AuthTypeBasic:
		auth.Username = authData["username"]
		auth.Password = authData["password"]
	case AuthTypeAPIKey:
		auth.APIKeyHeader = authData["api_key_header"]
		auth.APIKeyValue = authData["api_key_value"]
	}

	return auth, nil
}

// isSQLiteUniqueConstraintError checks if error is a unique constraint violation.
func isSQLiteUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if sql.ErrNoRows == err {
		return false
	}
	errMsg := err.Error()
	return errMsg == "UNIQUE constraint failed" ||
		errMsg == "constraint failed: UNIQUE constraint failed" ||
		(len(errMsg) > 20 && errMsg[:20] == "constraint failed: U")
}

// isSQLiteForeignKeyError checks if error is a foreign key constraint violation.
func isSQLiteForeignKeyError(err error) bool {
	if err == nil {
		return false
	}
	return sql.ErrNoRows != err && (err.Error() == "FOREIGN KEY constraint failed" ||
		err.Error() == "constraint failed: FOREIGN KEY constraint failed")
}
