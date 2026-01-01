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

package completion

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompleteRunIDs(t *testing.T) {
	// Clear cache before test
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	// Create mock controller server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/runs" {
			http.NotFound(w, r)
			return
		}

		resp := map[string]interface{}{
			"runs": []interface{}{
				map[string]interface{}{
					"id":       "run-001",
					"workflow": "test-workflow",
					"status":   "running",
				},
				map[string]interface{}{
					"id":       "run-002",
					"workflow": "another-workflow",
					"status":   "completed",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Configure client to use test server
	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	completions, directive := CompleteRunIDs(nil, nil, "")

	if len(completions) != 2 {
		t.Fatalf("expected 2 completions, got %d", len(completions))
	}

	// Check first completion
	expected := "run-001\ttest-workflow (running)"
	if completions[0] != expected {
		t.Errorf("expected %q, got %q", expected, completions[0])
	}

	// Check second completion
	expected = "run-002\tanother-workflow (completed)"
	if completions[1] != expected {
		t.Errorf("expected %q, got %q", expected, completions[1])
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
}

func TestCompleteActiveRunIDs(t *testing.T) {
	// Clear cache before test
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	// Create mock controller server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"runs": []interface{}{
				map[string]interface{}{
					"id":       "run-001",
					"workflow": "test-workflow",
					"status":   "running",
				},
				map[string]interface{}{
					"id":       "run-002",
					"workflow": "another-workflow",
					"status":   "completed",
				},
				map[string]interface{}{
					"id":       "run-003",
					"workflow": "pending-workflow",
					"status":   "pending",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	completions, _ := CompleteActiveRunIDs(nil, nil, "")

	// Should only include running and pending runs
	if len(completions) != 2 {
		t.Fatalf("expected 2 active completions, got %d", len(completions))
	}

	// Verify completed run is not included
	for _, c := range completions {
		if c == "run-002\tanother-workflow (completed)" {
			t.Error("completed run should not be in active completions")
		}
	}
}

func TestRunCaching(t *testing.T) {
	// Clear cache before test
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := map[string]interface{}{
			"runs": []interface{}{
				map[string]interface{}{
					"id":       "run-001",
					"workflow": "test",
					"status":   "running",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	// First call should hit the server
	_, _ = CompleteRunIDs(nil, nil, "")
	if callCount != 1 {
		t.Errorf("expected 1 API call, got %d", callCount)
	}

	// Second call should use cache
	_, _ = CompleteRunIDs(nil, nil, "")
	if callCount != 1 {
		t.Errorf("expected cache hit, but got %d API calls", callCount)
	}

	// Wait for cache to expire
	time.Sleep(runCacheTTL + 100*time.Millisecond)

	// Third call should hit the server again
	_, _ = CompleteRunIDs(nil, nil, "")
	if callCount != 2 {
		t.Errorf("expected 2 API calls after cache expiry, got %d", callCount)
	}
}

func TestCompleteRunIDs_DaemonNotRunning(t *testing.T) {
	// Clear cache
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	// Point to non-existent controller
	t.Setenv("CONDUCTOR_HOST", "tcp://localhost:9999")

	completions, directive := CompleteRunIDs(nil, nil, "")

	// Should return empty list when controller is not available
	if len(completions) != 0 {
		t.Errorf("expected 0 completions when controller unavailable, got %d", len(completions))
	}

	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp
		t.Errorf("expected NoFileComp directive, got %d", directive)
	}
}

func TestCompleteRunIDs_MalformedResponse(t *testing.T) {
	// Clear cache
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return invalid response structure
		resp := map[string]interface{}{
			"runs": "not-an-array",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	completions, _ := CompleteRunIDs(nil, nil, "")

	// Should handle gracefully
	if len(completions) != 0 {
		t.Errorf("expected 0 completions for malformed response, got %d", len(completions))
	}
}

func TestCompleteRunIDs_MissingFields(t *testing.T) {
	// Clear cache
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"runs": []interface{}{
				map[string]interface{}{
					// Missing id
					"workflow": "test",
					"status":   "running",
				},
				map[string]interface{}{
					"id": "run-001",
					// Missing workflow and status
				},
				map[string]interface{}{
					"id":       "run-002",
					"workflow": "valid-workflow",
					"status":   "running",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	completions, _ := CompleteRunIDs(nil, nil, "")

	// Should only include valid entries
	if len(completions) != 2 {
		t.Fatalf("expected 2 valid completions, got %d", len(completions))
	}
}

func TestFilterActiveRuns(t *testing.T) {
	runs := []runInfo{
		{id: "run-001", status: "running"},
		{id: "run-002", status: "completed"},
		{id: "run-003", status: "pending"},
		{id: "run-004", status: "failed"},
		{id: "run-005", status: "cancelled"},
	}

	active := filterActiveRuns(runs)

	if len(active) != 2 {
		t.Fatalf("expected 2 active runs, got %d", len(active))
	}

	// Verify only running and pending are included
	for _, run := range active {
		if run.status != "running" && run.status != "pending" {
			t.Errorf("unexpected status in active runs: %s", run.status)
		}
	}
}

func TestCompleteRunIDs_Timeout(t *testing.T) {
	// Clear cache
	runCacheMu.Lock()
	runCache = nil
	runCacheMu.Unlock()

	// Create a server that delays longer than the timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(controllerTimeout + 100*time.Millisecond)
		resp := map[string]interface{}{
			"runs": []interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("CONDUCTOR_HOST", "tcp://"+server.Listener.Addr().String())

	start := time.Now()
	completions, _ := CompleteRunIDs(nil, nil, "")
	elapsed := time.Since(start)

	// Should timeout and return empty
	if len(completions) != 0 {
		t.Errorf("expected 0 completions on timeout, got %d", len(completions))
	}

	// Verify it timed out around the expected time
	if elapsed > controllerTimeout+200*time.Millisecond {
		t.Errorf("timeout took too long: %v (expected ~%v)", elapsed, controllerTimeout)
	}
}

func init() {
	// Override timeout for tests to make them faster
	// This is already set in runs.go but we can keep it short for tests
}
