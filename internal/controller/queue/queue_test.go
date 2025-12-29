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

package queue

import (
	"context"
	"testing"
	"time"
)

func TestMemoryQueue_EnqueueDequeue(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()

	// Enqueue a job
	job := &Job{
		ID:        "test-job-1",
		Inputs:    map[string]any{"foo": "bar"},
		Priority:  0,
		CreatedAt: time.Now(),
	}

	err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if q.Len() != 1 {
		t.Errorf("Expected queue length 1, got %d", q.Len())
	}

	// Dequeue the job
	dequeued, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue failed: %v", err)
	}

	if dequeued.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, dequeued.ID)
	}

	if q.Len() != 0 {
		t.Errorf("Expected queue length 0, got %d", q.Len())
	}
}

func TestMemoryQueue_Priority(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()

	// Enqueue jobs with different priorities
	lowPriority := &Job{ID: "low", Priority: 0}
	highPriority := &Job{ID: "high", Priority: 10}
	medPriority := &Job{ID: "med", Priority: 5}

	q.Enqueue(ctx, lowPriority)
	q.Enqueue(ctx, highPriority)
	q.Enqueue(ctx, medPriority)

	// Should dequeue in priority order
	job1, _ := q.Dequeue(ctx)
	if job1.ID != "high" {
		t.Errorf("Expected high priority job first, got %s", job1.ID)
	}

	job2, _ := q.Dequeue(ctx)
	if job2.ID != "med" {
		t.Errorf("Expected medium priority job second, got %s", job2.ID)
	}

	job3, _ := q.Dequeue(ctx)
	if job3.ID != "low" {
		t.Errorf("Expected low priority job third, got %s", job3.ID)
	}
}

func TestMemoryQueue_Peek(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx := context.Background()

	// Peek on empty queue
	peeked, err := q.Peek(ctx)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if peeked != nil {
		t.Errorf("Expected nil on empty queue, got %v", peeked)
	}

	// Add a job and peek
	job := &Job{ID: "test-job"}
	q.Enqueue(ctx, job)

	peeked, err = q.Peek(ctx)
	if err != nil {
		t.Fatalf("Peek failed: %v", err)
	}
	if peeked.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, peeked.ID)
	}

	// Peek should not remove the job
	if q.Len() != 1 {
		t.Errorf("Expected queue length 1 after peek, got %d", q.Len())
	}
}

func TestMemoryQueue_DequeueBlocks(t *testing.T) {
	q := NewMemoryQueue()
	defer q.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Dequeue on empty queue should block and timeout
	_, err := q.Dequeue(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestMemoryQueue_Close(t *testing.T) {
	q := NewMemoryQueue()

	ctx := context.Background()
	job := &Job{ID: "test-job"}
	q.Enqueue(ctx, job)

	// Close the queue
	err := q.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Enqueue should fail
	err = q.Enqueue(ctx, job)
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}

	// Dequeue should fail
	_, err = q.Dequeue(ctx)
	if err != ErrQueueClosed {
		t.Errorf("Expected ErrQueueClosed, got %v", err)
	}
}
