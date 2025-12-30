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

package integrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tombee/conductor/internal/workspace"
)

// getStorage creates and returns a storage instance.
// The storage uses the database at ~/.conductor/conductor.db
func getStorage(ctx context.Context) (workspace.Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	conductorDir := filepath.Join(homeDir, ".conductor")
	if err := os.MkdirAll(conductorDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create conductor directory: %w", err)
	}

	dbPath := filepath.Join(conductorDir, "conductor.db")

	// Get master key from keychain or environment
	keychainMgr := workspace.NewKeychainManager()
	masterKey, err := keychainMgr.GetOrCreateMasterKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get master key: %w", err)
	}

	// Create encryptor with master key
	encryptor, err := workspace.NewAESEncryptor(masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	storage, err := workspace.NewSQLiteStorage(workspace.SQLiteConfig{
		Path:      dbPath,
		Encryptor: encryptor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	return storage, nil
}

// getWorkspaceName returns the workspace name from flag, environment, or current workspace.
// Priority: flag > environment > current workspace > "default"
func getWorkspaceName(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv("CONDUCTOR_WORKSPACE"); envValue != "" {
		return envValue
	}

	// Try to get current workspace from storage
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	storage, err := getStorage(ctx)
	if err == nil {
		defer storage.Close()
		currentWorkspace, err := storage.GetCurrentWorkspace(ctx)
		if err == nil && currentWorkspace != "" {
			return currentWorkspace
		}
	}

	return "default"
}

// getIntegrationBindings parses integration bindings from environment variable.
// CONDUCTOR_BIND_INTEGRATION format: "github=work,slack=team,source=personal"
// Returns a map from requirement identifier to integration name.
func getIntegrationBindings() map[string]string {
	envValue := os.Getenv("CONDUCTOR_BIND_INTEGRATION")
	if envValue == "" {
		return nil
	}

	bindings := make(map[string]string)

	// Split by comma
	pairs := splitAndTrim(envValue, ',')
	for _, pair := range pairs {
		// Split by equals
		parts := splitAndTrim(pair, '=')
		if len(parts) == 2 {
			bindings[parts[0]] = parts[1]
		}
	}

	return bindings
}

// splitAndTrim splits a string by a separator and trims whitespace from each part.
func splitAndTrim(s string, sep rune) []string {
	var parts []string
	current := ""

	for _, ch := range s {
		if ch == sep {
			if trimmed := trimSpace(current); trimmed != "" {
				parts = append(parts, trimmed)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}

	if trimmed := trimSpace(current); trimmed != "" {
		parts = append(parts, trimmed)
	}

	return parts
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	// Trim leading
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	// Trim trailing
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// redactAuth returns a human-readable description of auth without exposing secrets.
func redactAuth(auth workspace.AuthConfig) string {
	switch auth.Type {
	case workspace.AuthTypeNone:
		return "none"
	case workspace.AuthTypeToken:
		if auth.Token != "" {
			return "token (configured)"
		}
		return "token (not configured)"
	case workspace.AuthTypeBasic:
		if auth.Username != "" && auth.Password != "" {
			return fmt.Sprintf("basic (%s)", auth.Username)
		}
		return "basic (not configured)"
	case workspace.AuthTypeAPIKey:
		if auth.APIKeyHeader != "" && auth.APIKeyValue != "" {
			return fmt.Sprintf("api-key (%s: configured)", auth.APIKeyHeader)
		}
		return "api-key (not configured)"
	default:
		return string(auth.Type)
	}
}

// truncate truncates a string to the specified length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
