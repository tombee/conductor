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

package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tombee/conductor/internal/remote"
)

func TestNewClient(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		client := NewClient(Config{})
		if client.baseURL != "https://api.github.com" {
			t.Errorf("Expected default GitHub API URL, got %s", client.baseURL)
		}
	})

	t.Run("with token", func(t *testing.T) {
		client := NewClient(Config{Token: "test-token"})
		if client.token != "test-token" {
			t.Errorf("Expected token to be set")
		}
	})

	t.Run("GitHub Enterprise host", func(t *testing.T) {
		client := NewClient(Config{Host: "github.company.com"})
		expected := "https://github.company.com/api/v3"
		if client.baseURL != expected {
			t.Errorf("Expected %s, got %s", expected, client.baseURL)
		}
	})
}

func TestGetContent(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(*httptest.Server) string
		owner      string
		repo       string
		path       string
		ref        string
		wantErr    bool
		wantName   string
		wantSHA    string
		wantStatus int
	}{
		{
			name: "successful fetch",
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			owner:    "user",
			repo:     "repo",
			path:     "workflow.yaml",
			ref:      "main",
			wantName: "workflow.yaml",
			wantSHA:  "abc123",
		},
		{
			name: "404 not found",
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			owner:      "user",
			repo:       "repo",
			path:       "nonexistent.yaml",
			ref:        "main",
			wantErr:    true,
			wantStatus: 404,
		},
		{
			name: "401 unauthorized",
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			owner:      "user",
			repo:       "private-repo",
			path:       "workflow.yaml",
			ref:        "main",
			wantErr:    true,
			wantStatus: 401,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check authorization header
				auth := r.Header.Get("Authorization")
				if tt.wantStatus == 401 && auth == "" {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode(map[string]string{
						"message": "Requires authentication",
					})
					return
				}

				// Handle 404
				if tt.wantStatus == 404 {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{
						"message": "Not Found",
					})
					return
				}

				// Successful response
				w.WriteHeader(http.StatusOK)
				content := base64.StdEncoding.EncodeToString([]byte("name: test-workflow\nsteps: []"))
				json.NewEncoder(w).Encode(ContentResponse{
					Type:     "file",
					Encoding: "base64",
					Name:     tt.wantName,
					Path:     tt.path,
					Content:  content,
					SHA:      tt.wantSHA,
				})
			}))
			defer server.Close()

			// Create client with test server URL
			client := &Client{
				baseURL:    server.URL,
				httpClient: http.DefaultClient,
			}

			// Execute
			ctx := context.Background()
			resp, err := client.GetContent(ctx, tt.owner, tt.repo, tt.path, tt.ref)

			// Verify
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if resp.Name != tt.wantName {
				t.Errorf("Name = %s, want %s", resp.Name, tt.wantName)
			}
			if resp.SHA != tt.wantSHA {
				t.Errorf("SHA = %s, want %s", resp.SHA, tt.wantSHA)
			}
		})
	}
}

func TestDecodeContent(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
		wantErr bool
	}{
		{
			name:    "simple content",
			encoded: base64.StdEncoding.EncodeToString([]byte("hello world")),
			want:    "hello world",
		},
		{
			name:    "content with newlines",
			encoded: addNewlines(base64.StdEncoding.EncodeToString([]byte("hello\nworld"))),
			want:    "hello\nworld",
		},
		{
			name:    "YAML content",
			encoded: base64.StdEncoding.EncodeToString([]byte("name: test\nsteps:\n  - id: step1")),
			want:    "name: test\nsteps:\n  - id: step1",
		},
		{
			name:    "invalid base64",
			encoded: "not-valid-base64!!!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeContent(tt.encoded)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if string(decoded) != tt.want {
				t.Errorf("Decoded = %q, want %q", string(decoded), tt.want)
			}
		})
	}
}

func TestResolveRef(t *testing.T) {
	tests := []struct {
		name      string
		refInput  *remote.Reference
		setupMock func(*httptest.Server) string
		wantSHA   string
		wantErr   bool
	}{
		{
			name: "commit SHA - no resolution needed",
			refInput: &remote.Reference{
				Owner:   "user",
				Repo:    "repo",
				Version: "abc123def456",
				RefType: remote.RefTypeCommit,
			},
			wantSHA: "abc123def456",
		},
		{
			name: "branch name",
			refInput: &remote.Reference{
				Owner:   "user",
				Repo:    "repo",
				Version: "main",
				RefType: remote.RefTypeBranch,
			},
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			wantSHA: "resolved-sha-123",
		},
		{
			name: "tag name",
			refInput: &remote.Reference{
				Owner:   "user",
				Repo:    "repo",
				Version: "v1.0.0",
				RefType: remote.RefTypeTag,
			},
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			wantSHA: "tag-sha-456",
		},
		{
			name: "no version - fetch default branch",
			refInput: &remote.Reference{
				Owner:   "user",
				Repo:    "repo",
				Version: "",
				RefType: remote.RefTypeNone,
			},
			setupMock: func(srv *httptest.Server) string {
				return srv.URL
			},
			wantSHA: "default-branch-sha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client *Client

			if tt.setupMock != nil {
				// Create test server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Handle repository info request (for default branch)
					if r.URL.Path == "/repos/user/repo" {
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(map[string]string{
							"default_branch": "main",
						})
						return
					}

					// Handle git refs requests
					w.WriteHeader(http.StatusOK)
					var sha string
					if r.URL.Path == "/repos/user/repo/git/refs/heads/main" {
						sha = "resolved-sha-123"
						if tt.name == "no version - fetch default branch" {
							sha = "default-branch-sha"
						}
					} else if r.URL.Path == "/repos/user/repo/git/refs/tags/v1.0.0" {
						sha = "tag-sha-456"
					}

					json.NewEncoder(w).Encode(RefResponse{
						Ref: r.URL.Path,
						Object: struct {
							Type string `json:"type"`
							SHA  string `json:"sha"`
							URL  string `json:"url"`
						}{
							Type: "commit",
							SHA:  sha,
						},
					})
				}))
				defer server.Close()

				client = &Client{
					baseURL:    server.URL,
					httpClient: http.DefaultClient,
				}
			} else {
				client = &Client{
					baseURL:    "https://api.github.com",
					httpClient: http.DefaultClient,
				}
			}

			// Execute
			ctx := context.Background()
			sha, err := client.ResolveRef(ctx, tt.refInput.Owner, tt.refInput.Repo, tt.refInput)

			// Verify
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if sha != tt.wantSHA {
				t.Errorf("SHA = %s, want %s", sha, tt.wantSHA)
			}
		})
	}
}

func TestFetchWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle repository info
		if r.URL.Path == "/repos/user/repo" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"default_branch": "main",
			})
			return
		}

		// Handle git refs
		if r.URL.Path == "/repos/user/repo/git/refs/heads/main" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(RefResponse{
				Object: struct {
					Type string `json:"type"`
					SHA  string `json:"sha"`
					URL  string `json:"url"`
				}{
					SHA: "test-sha-123",
				},
			})
			return
		}

		// Handle content fetch
		if r.URL.Path == "/repos/user/repo/contents/workflow.yaml" {
			w.WriteHeader(http.StatusOK)
			content := "name: test-workflow\nsteps:\n  - id: step1\n    type: llm"
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			json.NewEncoder(w).Encode(ContentResponse{
				Name:    "workflow.yaml",
				Content: encoded,
				SHA:     "file-sha-456",
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: http.DefaultClient,
	}

	ref := &remote.Reference{
		Owner: "user",
		Repo:  "repo",
		Path:  "",
	}

	ctx := context.Background()
	data, sha, err := client.FetchWorkflow(ctx, ref)
	if err != nil {
		t.Fatalf("FetchWorkflow() error = %v", err)
	}

	if sha != "test-sha-123" {
		t.Errorf("SHA = %s, want test-sha-123", sha)
	}

	expected := "name: test-workflow\nsteps:\n  - id: step1\n    type: llm"
	if string(data) != expected {
		t.Errorf("Content = %q, want %q", string(data), expected)
	}
}

// addNewlines adds newlines to a base64 string to simulate GitHub API response format
func addNewlines(s string) string {
	// GitHub API adds newlines every 60 characters
	var result string
	for i := 0; i < len(s); i += 60 {
		end := i + 60
		if end > len(s) {
			end = len(s)
		}
		result += s[i:end] + "\n"
	}
	return result
}
