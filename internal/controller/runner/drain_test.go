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

package runner

import (
	"context"
	"testing"
	"time"
)

func TestStartDraining(t *testing.T) {
	r := New(Config{}, nil, nil)

	if r.IsDraining() {
		t.Error("Runner should not be draining initially")
	}

	r.StartDraining()

	if !r.IsDraining() {
		t.Error("Runner should be draining after StartDraining()")
	}
}

func TestIsDraining(t *testing.T) {
	r := New(Config{}, nil, nil)

	// Initial state
	if r.IsDraining() {
		t.Error("IsDraining() should return false initially")
	}

	// After starting drain
	r.StartDraining()
	if !r.IsDraining() {
		t.Error("IsDraining() should return true after StartDraining()")
	}
}

func TestActiveRunCount(t *testing.T) {
	r := New(Config{}, nil, nil)

	// No runs initially
	if count := r.ActiveRunCount(); count != 0 {
		t.Errorf("ActiveRunCount() = %d, want 0", count)
	}

	// Add a pending run
	ctx := context.Background()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.state.mu.Lock()
	r.state.runs["run1"] = &Run{
		ID:        "run1",
		Status:    RunStatusPending,
		ctx:       runCtx,
		cancel:    cancel,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	if count := r.ActiveRunCount(); count != 1 {
		t.Errorf("ActiveRunCount() = %d, want 1 (pending run)", count)
	}

	// Add a running run
	runCtx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	r.state.mu.Lock()
	r.state.runs["run2"] = &Run{
		ID:        "run2",
		Status:    RunStatusRunning,
		ctx:       runCtx2,
		cancel:    cancel2,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	if count := r.ActiveRunCount(); count != 2 {
		t.Errorf("ActiveRunCount() = %d, want 2 (pending + running)", count)
	}

	// Add a completed run
	runCtx3, cancel3 := context.WithCancel(ctx)
	defer cancel3()

	r.state.mu.Lock()
	r.state.runs["run3"] = &Run{
		ID:        "run3",
		Status:    RunStatusCompleted,
		ctx:       runCtx3,
		cancel:    cancel3,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	if count := r.ActiveRunCount(); count != 2 {
		t.Errorf("ActiveRunCount() = %d, want 2 (completed run not counted)", count)
	}

	// Add a failed run
	runCtx4, cancel4 := context.WithCancel(ctx)
	defer cancel4()

	r.state.mu.Lock()
	r.state.runs["run4"] = &Run{
		ID:        "run4",
		Status:    RunStatusFailed,
		ctx:       runCtx4,
		cancel:    cancel4,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	if count := r.ActiveRunCount(); count != 2 {
		t.Errorf("ActiveRunCount() = %d, want 2 (failed run not counted)", count)
	}

	// Add a cancelled run
	runCtx5, cancel5 := context.WithCancel(ctx)
	defer cancel5()

	r.state.mu.Lock()
	r.state.runs["run5"] = &Run{
		ID:        "run5",
		Status:    RunStatusCancelled,
		ctx:       runCtx5,
		cancel:    cancel5,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	if count := r.ActiveRunCount(); count != 2 {
		t.Errorf("ActiveRunCount() = %d, want 2 (cancelled run not counted)", count)
	}
}

func TestWaitForDrain_NoActiveRuns(t *testing.T) {
	r := New(Config{}, nil, nil)

	ctx := context.Background()
	err := r.WaitForDrain(ctx, 1*time.Second)

	if err != nil {
		t.Errorf("WaitForDrain() with no active runs should return nil, got: %v", err)
	}
}

func TestWaitForDrain_RunsComplete(t *testing.T) {
	r := New(Config{}, nil, nil)

	// Add a running run
	ctx := context.Background()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.state.mu.Lock()
	r.state.runs["run1"] = &Run{
		ID:        "run1",
		Status:    RunStatusRunning,
		ctx:       runCtx,
		cancel:    cancel,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	// Start a goroutine to complete the run after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		r.state.mu.Lock()
		r.state.runs["run1"].Status = RunStatusCompleted
		r.state.mu.Unlock()
	}()

	// Wait for drain with longer timeout
	err := r.WaitForDrain(ctx, 2*time.Second)

	if err != nil {
		t.Errorf("WaitForDrain() should return nil when run completes, got: %v", err)
	}
}

func TestWaitForDrain_Timeout(t *testing.T) {
	r := New(Config{}, nil, nil)

	// Add a long-running run that won't complete
	ctx := context.Background()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.state.mu.Lock()
	r.state.runs["run1"] = &Run{
		ID:        "run1",
		Status:    RunStatusRunning,
		ctx:       runCtx,
		cancel:    cancel,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	// Wait for drain with short timeout
	err := r.WaitForDrain(ctx, 200*time.Millisecond)

	if err == nil {
		t.Error("WaitForDrain() should return error on timeout")
	}

	// Verify error message mentions the remaining workflows
	expectedMsg := "drain timeout: 1 workflow(s) still running"
	if err.Error() != expectedMsg {
		t.Errorf("WaitForDrain() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestWaitForDrain_ContextCancelled(t *testing.T) {
	r := New(Config{}, nil, nil)

	// Add a running run
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r.state.mu.Lock()
	r.state.runs["run1"] = &Run{
		ID:        "run1",
		Status:    RunStatusRunning,
		ctx:       runCtx,
		cancel:    cancel,
		stopped:   make(chan struct{}),
		CreatedAt: time.Now(),
	}
	r.state.mu.Unlock()

	// Create a context that we'll cancel
	ctx, ctxCancel := context.WithCancel(context.Background())

	// Cancel context after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		ctxCancel()
	}()

	// Wait for drain with longer timeout
	err := r.WaitForDrain(ctx, 2*time.Second)

	if err == nil {
		t.Error("WaitForDrain() should return error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("WaitForDrain() error = %v, want %v", err, context.Canceled)
	}
}

func TestWaitForDrain_MultipleRuns(t *testing.T) {
	r := New(Config{}, nil, nil)

	ctx := context.Background()

	// Add multiple running runs
	for i := 1; i <= 3; i++ {
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		runID := string(rune('0' + i))
		r.state.mu.Lock()
		r.state.runs[runID] = &Run{
			ID:        runID,
			Status:    RunStatusRunning,
			ctx:       runCtx,
			cancel:    cancel,
			stopped:   make(chan struct{}),
			CreatedAt: time.Now(),
		}
		r.state.mu.Unlock()
	}

	// Start goroutines to complete runs at different times
	go func() {
		time.Sleep(100 * time.Millisecond)
		r.state.mu.Lock()
		r.state.runs["1"].Status = RunStatusCompleted
		r.state.mu.Unlock()
	}()
	go func() {
		time.Sleep(200 * time.Millisecond)
		r.state.mu.Lock()
		r.state.runs["2"].Status = RunStatusFailed
		r.state.mu.Unlock()
	}()
	go func() {
		time.Sleep(300 * time.Millisecond)
		r.state.mu.Lock()
		r.state.runs["3"].Status = RunStatusCancelled
		r.state.mu.Unlock()
	}()

	// Wait for all runs to complete
	err := r.WaitForDrain(ctx, 2*time.Second)

	if err != nil {
		t.Errorf("WaitForDrain() should return nil when all runs complete, got: %v", err)
	}
}
