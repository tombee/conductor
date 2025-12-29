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

package cache

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewWorkflowCache(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()

	t.Run("with custom path", func(t *testing.T) {
		cache, err := NewWorkflowCache(Config{
			BasePath: tmpDir,
		})
		if err != nil {
			t.Fatalf("NewWorkflowCache() error = %v", err)
		}
		if cache.basePath != tmpDir {
			t.Errorf("basePath = %s, want %s", cache.basePath, tmpDir)
		}
	})

	t.Run("with TTL", func(t *testing.T) {
		cache, err := NewWorkflowCache(Config{
			BasePath: tmpDir,
			TTL:      24 * time.Hour,
		})
		if err != nil {
			t.Fatalf("NewWorkflowCache() error = %v", err)
		}
		if cache.ttl != 24*time.Hour {
			t.Errorf("ttl = %v, want %v", cache.ttl, 24*time.Hour)
		}
	})
}

func TestWorkflowCache_PutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	owner := "testuser"
	repo := "testrepo"
	sha := "abc123def456"
	sourceURL := "github:testuser/testrepo@v1.0"
	content := []byte("name: test-workflow\nsteps:\n  - id: step1")

	// Put
	err = cache.Put(owner, repo, sha, sourceURL, content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Get
	cached, err := cache.Get(owner, repo, sha)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cached == nil {
		t.Fatal("Get() returned nil, expected cached entry")
	}

	// Verify
	if cached.SourceURL != sourceURL {
		t.Errorf("SourceURL = %s, want %s", cached.SourceURL, sourceURL)
	}
	if cached.CommitSHA != sha {
		t.Errorf("CommitSHA = %s, want %s", cached.CommitSHA, sha)
	}
	if string(cached.Content) != string(content) {
		t.Errorf("Content = %s, want %s", string(cached.Content), string(content))
	}
	if cached.Size != len(content) {
		t.Errorf("Size = %d, want %d", cached.Size, len(content))
	}
}

func TestWorkflowCache_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	// Get non-existent entry
	cached, err := cache.Get("nonexistent", "repo", "sha")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cached != nil {
		t.Errorf("Get() returned %v, expected nil (cache miss)", cached)
	}
}

func TestWorkflowCache_TTL(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{
		BasePath: tmpDir,
		TTL:      100 * time.Millisecond, // Very short TTL for testing
	})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	owner := "testuser"
	repo := "testrepo"
	sha := "expired-sha"
	content := []byte("expired workflow")

	// Put
	err = cache.Put(owner, repo, sha, "github:testuser/testrepo", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Immediate get should work
	cached, err := cache.Get(owner, repo, sha)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cached == nil {
		t.Fatal("Get() returned nil, expected cached entry")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Get after expiration should return nil
	cached, err = cache.Get(owner, repo, sha)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cached != nil {
		t.Errorf("Get() returned %v, expected nil (expired)", cached)
	}

	// Verify cache directory was deleted
	cachePath := cache.getCachePath(owner, repo, sha)
	if fileExists(cachePath) {
		t.Errorf("Expired cache entry still exists at %s", cachePath)
	}
}

func TestWorkflowCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	owner := "testuser"
	repo := "testrepo"
	sha := "delete-sha"
	content := []byte("to be deleted")

	// Put
	err = cache.Put(owner, repo, sha, "github:testuser/testrepo", content)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Verify it exists
	cached, err := cache.Get(owner, repo, sha)
	if err != nil || cached == nil {
		t.Fatal("Entry should exist before delete")
	}

	// Delete
	err = cache.Delete(owner, repo, sha)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	cached, err = cache.Get(owner, repo, sha)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if cached != nil {
		t.Errorf("Get() returned %v, expected nil (deleted)", cached)
	}
}

func TestWorkflowCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	// Put multiple entries
	cache.Put("user1", "repo1", "sha1", "github:user1/repo1", []byte("content1"))
	cache.Put("user1", "repo1", "sha2", "github:user1/repo1", []byte("content2"))
	cache.Put("user1", "repo2", "sha3", "github:user1/repo2", []byte("content3"))
	cache.Put("user2", "repo1", "sha4", "github:user2/repo1", []byte("content4"))

	t.Run("clear specific repo", func(t *testing.T) {
		err := cache.Clear("user1", "repo1")
		if err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		// user1/repo1 entries should be gone
		if cached, _ := cache.Get("user1", "repo1", "sha1"); cached != nil {
			t.Error("user1/repo1/sha1 should be deleted")
		}
		if cached, _ := cache.Get("user1", "repo1", "sha2"); cached != nil {
			t.Error("user1/repo1/sha2 should be deleted")
		}

		// Others should still exist
		if cached, _ := cache.Get("user1", "repo2", "sha3"); cached == nil {
			t.Error("user1/repo2/sha3 should still exist")
		}
		if cached, _ := cache.Get("user2", "repo1", "sha4"); cached == nil {
			t.Error("user2/repo1/sha4 should still exist")
		}
	})

	t.Run("clear all repos for owner", func(t *testing.T) {
		err := cache.Clear("user1", "")
		if err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		// All user1 entries should be gone
		if cached, _ := cache.Get("user1", "repo2", "sha3"); cached != nil {
			t.Error("user1/repo2/sha3 should be deleted")
		}

		// user2 should still exist
		if cached, _ := cache.Get("user2", "repo1", "sha4"); cached == nil {
			t.Error("user2/repo1/sha4 should still exist")
		}
	})

	t.Run("clear entire cache", func(t *testing.T) {
		err := cache.Clear("", "")
		if err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		// Everything should be gone
		if cached, _ := cache.Get("user2", "repo1", "sha4"); cached != nil {
			t.Error("user2/repo1/sha4 should be deleted")
		}
	})
}

func TestWorkflowCache_List(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	// Put multiple entries for same repo
	cache.Put("testuser", "testrepo", "sha1", "github:testuser/testrepo@v1.0", []byte("v1.0"))
	cache.Put("testuser", "testrepo", "sha2", "github:testuser/testrepo@v2.0", []byte("v2.0"))
	cache.Put("testuser", "testrepo", "sha3", "github:testuser/testrepo@main", []byte("main"))

	// List
	entries, err := cache.List("testuser", "testrepo")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("List() returned %d entries, want 3", len(entries))
	}

	// Verify entries
	shas := make(map[string]bool)
	for _, entry := range entries {
		shas[entry.CommitSHA] = true
	}

	for _, expectedSHA := range []string{"sha1", "sha2", "sha3"} {
		if !shas[expectedSHA] {
			t.Errorf("Expected SHA %s not found in list", expectedSHA)
		}
	}
}

func TestWorkflowCache_ListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	// List non-existent repo
	entries, err := cache.List("nonexistent", "repo")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("List() returned %d entries, want 0", len(entries))
	}
}

func TestWorkflowCache_GetCachePath(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewWorkflowCache(Config{BasePath: tmpDir})
	if err != nil {
		t.Fatalf("NewWorkflowCache() error = %v", err)
	}

	path := cache.getCachePath("user", "repo", "sha123")
	expected := filepath.Join(tmpDir, "github.com", "user", "repo", "sha123")

	if path != expected {
		t.Errorf("getCachePath() = %s, want %s", path, expected)
	}
}
