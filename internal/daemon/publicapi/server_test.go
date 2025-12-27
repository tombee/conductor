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

package publicapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/config"
)

func TestServerLifecycle(t *testing.T) {
	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Create server on random port
	cfg := config.PublicAPIConfig{
		Enabled: true,
		TCP:     "127.0.0.1:0", // Random port
	}

	server := New(cfg, handler, nil)

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running and has an address
	if server.Addr() == "" {
		t.Fatal("server address is empty")
	}

	// Stop server
	cancel()

	// Wait for shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Verify Start() returned without error
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("unexpected error from Start: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Start() did not return after shutdown")
	}
}

func TestServerShutdownBeforeStart(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	cfg := config.PublicAPIConfig{
		Enabled: true,
		TCP:     "127.0.0.1:0",
	}

	server := New(cfg, handler, nil)

	// Shutdown before Start should not error
	ctx := context.Background()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown before start returned error: %v", err)
	}
}
