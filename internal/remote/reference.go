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

// Package remote provides types and utilities for remote workflow references.
package remote

import (
	"fmt"
	"regexp"
	"strings"
)

// RefType represents the type of remote reference.
type RefType string

const (
	RefTypeTag    RefType = "tag"
	RefTypeBranch RefType = "branch"
	RefTypeCommit RefType = "commit"
	RefTypeNone   RefType = "none"
)

// Reference represents a parsed remote workflow reference.
// Format: github:owner/repo[/path][@version]
// Examples:
//   - github:user/repo
//   - github:user/repo@v1.0
//   - github:user/repo@main
//   - github:user/repo@abc123def
//   - github:user/repo/workflows/review
//   - github:user/repo/workflows/review@v1.0
type Reference struct {
	// Owner is the repository owner/organization
	Owner string

	// Repo is the repository name
	Repo string

	// Path is the optional subdirectory path within the repo
	// If empty, defaults to workflow.yaml at root
	// If it's a directory path, appends /workflow.yaml
	// If it ends with .yaml or .yml, treats as direct file reference
	Path string

	// Version is the optional git reference (tag, branch, or commit SHA)
	// If empty, uses the default branch (typically "main" or "master")
	Version string

	// RefType indicates whether Version is a tag, branch, or commit
	RefType RefType
}

var (
	// referencePattern matches github:owner/repo[/path][@version]
	// Captures: owner, repo, optional path, optional version
	referencePattern = regexp.MustCompile(`^github:([a-zA-Z0-9][\w-]*)/([a-zA-Z0-9][\w.-]*)(/[^@]*)?(@.+)?$`)

	// commitSHAPattern matches full (40 char) or short (7+ char) commit SHAs
	commitSHAPattern = regexp.MustCompile(`^[a-f0-9]{7,40}$`)
)

// ParseReference parses a remote reference string into a Reference struct.
func ParseReference(ref string) (*Reference, error) {
	if !strings.HasPrefix(ref, "github:") {
		return nil, fmt.Errorf("reference must start with 'github:' prefix")
	}

	matches := referencePattern.FindStringSubmatch(ref)
	if matches == nil {
		return nil, fmt.Errorf("invalid reference format: %s (expected github:owner/repo[/path][@version])", ref)
	}

	owner := matches[1]
	repo := matches[2]
	path := strings.TrimPrefix(matches[3], "/")
	version := strings.TrimPrefix(matches[4], "@")

	// Determine ref type if version is specified
	refType := RefTypeNone
	if version != "" {
		refType = determineRefType(version)
	}

	return &Reference{
		Owner:   owner,
		Repo:    repo,
		Path:    path,
		Version: version,
		RefType: refType,
	}, nil
}

// determineRefType attempts to classify a version string as a tag, branch, or commit.
func determineRefType(version string) RefType {
	// Check if it looks like a commit SHA (hexadecimal, 7-40 chars)
	if commitSHAPattern.MatchString(version) {
		return RefTypeCommit
	}

	// Check if it starts with 'v' followed by a number (common tag pattern)
	if strings.HasPrefix(version, "v") && len(version) > 1 {
		rest := version[1:]
		if len(rest) > 0 && (rest[0] >= '0' && rest[0] <= '9') {
			return RefTypeTag
		}
	}

	// Default to branch for everything else (e.g., "main", "develop", "feature/x")
	return RefTypeBranch
}

// FullPath returns the complete file path within the repository.
// If Path is empty, returns "workflow.yaml"
// If Path doesn't end with .yaml/.yml, appends "/workflow.yaml"
func (r *Reference) FullPath() string {
	if r.Path == "" {
		return "workflow.yaml"
	}

	// If path already points to a YAML file, use it as-is
	if strings.HasSuffix(r.Path, ".yaml") || strings.HasSuffix(r.Path, ".yml") {
		return r.Path
	}

	// Otherwise, treat as directory and append workflow.yaml
	return r.Path + "/workflow.yaml"
}

// String returns the canonical string representation of the reference.
func (r *Reference) String() string {
	s := fmt.Sprintf("github:%s/%s", r.Owner, r.Repo)
	if r.Path != "" {
		s += "/" + r.Path
	}
	if r.Version != "" {
		s += "@" + r.Version
	}
	return s
}

// IsRemote checks if a string looks like a remote reference.
func IsRemote(ref string) bool {
	return strings.HasPrefix(ref, "github:")
}
