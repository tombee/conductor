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

package remote

import (
	"testing"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Reference
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "basic reference",
			input: "github:user/repo",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "reference with tag",
			input: "github:user/repo@v1.0",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "v1.0",
				RefType: RefTypeTag,
			},
		},
		{
			name:  "reference with tag (v2.1.3)",
			input: "github:user/repo@v2.1.3",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "v2.1.3",
				RefType: RefTypeTag,
			},
		},
		{
			name:  "reference with branch",
			input: "github:user/repo@main",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "main",
				RefType: RefTypeBranch,
			},
		},
		{
			name:  "reference with branch (develop)",
			input: "github:user/repo@develop",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "develop",
				RefType: RefTypeBranch,
			},
		},
		{
			name:  "reference with commit SHA (short)",
			input: "github:user/repo@abc123d",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "abc123d",
				RefType: RefTypeCommit,
			},
		},
		{
			name:  "reference with commit SHA (full)",
			input: "github:user/repo@abc123def456789012345678901234567890abcd",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "",
				Version: "abc123def456789012345678901234567890abcd",
				RefType: RefTypeCommit,
			},
		},
		{
			name:  "reference with subdirectory",
			input: "github:user/repo/workflows",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "workflows",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "reference with nested subdirectory",
			input: "github:user/repo/workflows/code-review",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "workflows/code-review",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "reference with subdirectory and version",
			input: "github:user/repo/workflows/review@v1.0",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "workflows/review",
				Version: "v1.0",
				RefType: RefTypeTag,
			},
		},
		{
			name:  "reference with explicit YAML file",
			input: "github:user/repo/custom.yaml",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "custom.yaml",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "reference with explicit YAML file and version",
			input: "github:user/repo/workflows/review.yaml@v2.0",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "workflows/review.yaml",
				Version: "v2.0",
				RefType: RefTypeTag,
			},
		},
		{
			name:  "reference with .yml extension",
			input: "github:user/repo/workflow.yml",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo",
				Path:    "workflow.yml",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "organization repository",
			input: "github:my-org/my-repo",
			want: &Reference{
				Owner:   "my-org",
				Repo:    "my-repo",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:  "repository with dots",
			input: "github:user/repo.name",
			want: &Reference{
				Owner:   "user",
				Repo:    "repo.name",
				Version: "",
				RefType: RefTypeNone,
			},
		},
		{
			name:      "invalid - missing prefix",
			input:     "user/repo",
			wantErr:   true,
			errSubstr: "must start with 'github:' prefix",
		},
		{
			name:      "invalid - no repository",
			input:     "github:user",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
		{
			name:      "invalid - empty",
			input:     "",
			wantErr:   true,
			errSubstr: "must start with 'github:' prefix",
		},
		{
			name:      "invalid - only prefix",
			input:     "github:",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
		{
			name:      "invalid - special characters in owner",
			input:     "github:user@name/repo",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
		{
			name:      "invalid - spaces",
			input:     "github:user name/repo",
			wantErr:   true,
			errSubstr: "invalid reference format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseReference(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseReference() expected error, got nil")
					return
				}
				if tt.errSubstr != "" && !containsString(err.Error(), tt.errSubstr) {
					t.Errorf("ParseReference() error = %v, want substring %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseReference() unexpected error: %v", err)
				return
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.want.Owner)
			}
			if got.Repo != tt.want.Repo {
				t.Errorf("Repo = %q, want %q", got.Repo, tt.want.Repo)
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.want.Path)
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version = %q, want %q", got.Version, tt.want.Version)
			}
			if got.RefType != tt.want.RefType {
				t.Errorf("RefType = %q, want %q", got.RefType, tt.want.RefType)
			}
		})
	}
}

func TestReference_FullPath(t *testing.T) {
	tests := []struct {
		name string
		ref  *Reference
		want string
	}{
		{
			name: "empty path defaults to workflow.yaml",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: ""},
			want: "workflow.yaml",
		},
		{
			name: "directory path appends workflow.yaml",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflows"},
			want: "workflows/workflow.yaml",
		},
		{
			name: "nested directory path appends workflow.yaml",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflows/review"},
			want: "workflows/review/workflow.yaml",
		},
		{
			name: "explicit .yaml file used as-is",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "custom.yaml"},
			want: "custom.yaml",
		},
		{
			name: "explicit .yml file used as-is",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflow.yml"},
			want: "workflow.yml",
		},
		{
			name: "path with .yaml in subdirectory",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflows/custom.yaml"},
			want: "workflows/custom.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.FullPath()
			if got != tt.want {
				t.Errorf("FullPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReference_String(t *testing.T) {
	tests := []struct {
		name string
		ref  *Reference
		want string
	}{
		{
			name: "basic reference",
			ref:  &Reference{Owner: "user", Repo: "repo"},
			want: "github:user/repo",
		},
		{
			name: "with version",
			ref:  &Reference{Owner: "user", Repo: "repo", Version: "v1.0"},
			want: "github:user/repo@v1.0",
		},
		{
			name: "with path",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflows"},
			want: "github:user/repo/workflows",
		},
		{
			name: "with path and version",
			ref:  &Reference{Owner: "user", Repo: "repo", Path: "workflows/review", Version: "v1.0"},
			want: "github:user/repo/workflows/review@v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid remote reference",
			input: "github:user/repo",
			want:  true,
		},
		{
			name:  "local file path",
			input: "workflow.yaml",
			want:  false,
		},
		{
			name:  "local directory",
			input: "./workflows",
			want:  false,
		},
		{
			name:  "absolute path",
			input: "/path/to/workflow.yaml",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRemote(tt.input)
			if got != tt.want {
				t.Errorf("IsRemote(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetermineRefType(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    RefType
	}{
		{
			name:    "semantic version tag",
			version: "v1.0.0",
			want:    RefTypeTag,
		},
		{
			name:    "version tag without patch",
			version: "v2.1",
			want:    RefTypeTag,
		},
		{
			name:    "short commit SHA (7 chars)",
			version: "abc123d",
			want:    RefTypeCommit,
		},
		{
			name:    "commit SHA (8 chars)",
			version: "abc123de",
			want:    RefTypeCommit,
		},
		{
			name:    "full commit SHA (40 chars)",
			version: "abc123def456789012345678901234567890abcd",
			want:    RefTypeCommit,
		},
		{
			name:    "branch name - main",
			version: "main",
			want:    RefTypeBranch,
		},
		{
			name:    "branch name - develop",
			version: "develop",
			want:    RefTypeBranch,
		},
		{
			name:    "branch name with slashes",
			version: "feature/new-feature",
			want:    RefTypeBranch,
		},
		{
			name:    "branch name with numbers",
			version: "release-2024",
			want:    RefTypeBranch,
		},
		{
			name:    "not a tag - v with letter",
			version: "vNext",
			want:    RefTypeBranch,
		},
		{
			name:    "edge case - just v",
			version: "v",
			want:    RefTypeBranch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineRefType(tt.version)
			if got != tt.want {
				t.Errorf("determineRefType(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && containsLoop(s, substr)
}

func containsLoop(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
