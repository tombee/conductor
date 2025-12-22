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

package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/mcp/version"
)

// MCPLockfile represents the MCP version lockfile.
// Stored at ~/.config/conductor/mcp-lock.yaml
type MCPLockfile struct {
	// Version is the lockfile format version.
	Version int `yaml:"version"`

	// GeneratedAt is when the lockfile was last updated.
	GeneratedAt time.Time `yaml:"generated_at"`

	// Servers contains locked versions for each server.
	Servers map[string]*LockedServer `yaml:"servers,omitempty"`
}

// LockedServer represents a locked server version.
type LockedServer struct {
	// Source is the package source (e.g., "npm:@mcp/server-github")
	Source string `yaml:"source"`

	// Constraint is the version constraint from configuration.
	Constraint string `yaml:"constraint"`

	// Resolved is the resolved version.
	Resolved string `yaml:"resolved"`

	// Integrity is a hash for verification.
	Integrity string `yaml:"integrity,omitempty"`

	// Command is the resolved command to run.
	Command string `yaml:"command,omitempty"`

	// Args are the resolved arguments.
	Args []string `yaml:"args,omitempty"`

	// LockedAt is when this version was locked.
	LockedAt time.Time `yaml:"locked_at"`
}

const (
	// LockfileVersion is the current lockfile format version.
	LockfileVersion = 1

	// LockfileName is the default lockfile name.
	LockfileName = "mcp-lock.yaml"
)

// LoadLockfile loads the MCP lockfile.
func LoadLockfile() (*MCPLockfile, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	path := filepath.Join(dir, LockfileName)
	return LoadLockfileFromPath(path)
}

// LoadLockfileFromPath loads a lockfile from a specific path.
func LoadLockfileFromPath(path string) (*MCPLockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty lockfile
			return &MCPLockfile{
				Version: LockfileVersion,
				Servers: make(map[string]*LockedServer),
			}, nil
		}
		return nil, fmt.Errorf("failed to read lockfile: %w", err)
	}

	var lockfile MCPLockfile
	if err := yaml.Unmarshal(data, &lockfile); err != nil {
		return nil, fmt.Errorf("failed to parse lockfile: %w", err)
	}

	if lockfile.Servers == nil {
		lockfile.Servers = make(map[string]*LockedServer)
	}

	return &lockfile, nil
}

// SaveLockfile saves the MCP lockfile.
func SaveLockfile(lockfile *MCPLockfile) error {
	dir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}

	path := filepath.Join(dir, LockfileName)
	return SaveLockfileToPath(lockfile, path)
}

// SaveLockfileToPath saves a lockfile to a specific path.
func SaveLockfileToPath(lockfile *MCPLockfile, path string) error {
	lockfile.Version = LockfileVersion
	lockfile.GeneratedAt = time.Now()

	data, err := yaml.Marshal(lockfile)
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lockfile directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}

	return nil
}

// LockServer locks a server version in the lockfile.
func (l *MCPLockfile) LockServer(name string, resolved *version.ResolvedVersion, constraint string) {
	l.Servers[name] = &LockedServer{
		Source:     resolved.Source,
		Constraint: constraint,
		Resolved:   resolved.Version,
		Integrity:  resolved.Integrity,
		Command:    resolved.Command,
		Args:       resolved.Args,
		LockedAt:   time.Now(),
	}
}

// GetLockedServer returns the locked version for a server.
func (l *MCPLockfile) GetLockedServer(name string) (*LockedServer, bool) {
	server, ok := l.Servers[name]
	return server, ok
}

// RemoveServer removes a server from the lockfile.
func (l *MCPLockfile) RemoveServer(name string) {
	delete(l.Servers, name)
}

// IsLocked returns true if a server has a locked version.
func (l *MCPLockfile) IsLocked(name string) bool {
	_, ok := l.Servers[name]
	return ok
}
