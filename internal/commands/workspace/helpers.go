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
	"fmt"
	"os"
	"path/filepath"

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
