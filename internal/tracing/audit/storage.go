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

package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Store provides read access to audit logs
type Store struct {
	path string
}

// NewStore creates a new audit log store
func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

// Query retrieves audit log entries matching the given criteria
func (s *Store) Query(filter QueryFilter) ([]Entry, error) {
	// Open audit log file for reading
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil // No audit log yet
		}
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip malformed entries
			continue
		}

		// Apply filters
		if filter.matches(entry) {
			entries = append(entries, entry)
		}

		// Limit results
		if filter.Limit > 0 && len(entries) >= filter.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	return entries, nil
}

// QueryFilter defines criteria for querying audit logs
type QueryFilter struct {
	// UserID filters by user ID
	UserID string

	// Action filters by action type
	Action Action

	// Result filters by result
	Result Result

	// Since filters entries after this time
	Since time.Time

	// Until filters entries before this time
	Until time.Time

	// Limit limits the number of results
	Limit int
}

// matches returns true if the entry matches the filter
func (f QueryFilter) matches(entry Entry) bool {
	if f.UserID != "" && entry.UserID != f.UserID {
		return false
	}

	if f.Action != "" && entry.Action != f.Action {
		return false
	}

	if f.Result != "" && entry.Result != f.Result {
		return false
	}

	if !f.Since.IsZero() && entry.Timestamp.Before(f.Since) {
		return false
	}

	if !f.Until.IsZero() && entry.Timestamp.After(f.Until) {
		return false
	}

	return true
}

// Cleanup removes audit log entries older than the retention period
func (s *Store) Cleanup(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Read all entries
	entries, err := s.Query(QueryFilter{
		Since: cutoff,
	})
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	// Rewrite file with only recent entries
	// This is safe because audit logs are append-only
	tmpPath := s.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temp audit log: %w", err)
	}
	defer f.Close()

	// Write entries to temp file
	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("failed to write audit entry: %w", err)
		}
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	// Replace original file
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("failed to replace audit log: %w", err)
	}

	return nil
}
