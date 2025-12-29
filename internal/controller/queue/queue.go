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

// Package queue provides job queue management for workflow execution.
package queue

import (
	"context"
	"sync"
	"time"
)

// Job represents a workflow execution job in the queue.
type Job struct {
	ID           string
	WorkflowYAML []byte
	Inputs       map[string]any
	Priority     int
	CreatedAt    time.Time
}

// Queue defines the interface for job queue implementations.
type Queue interface {
	// Enqueue adds a job to the queue.
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue removes and returns the next job from the queue.
	// Blocks until a job is available or context is cancelled.
	Dequeue(ctx context.Context) (*Job, error)

	// Peek returns the next job without removing it.
	Peek(ctx context.Context) (*Job, error)

	// Len returns the number of jobs in the queue.
	Len() int

	// Close closes the queue.
	Close() error
}

// MemoryQueue is an in-memory queue implementation.
type MemoryQueue struct {
	mu       sync.Mutex
	jobs     []*Job
	signal   chan struct{}
	closed   bool
	closedMu sync.RWMutex
}

// NewMemoryQueue creates a new in-memory queue.
func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{
		jobs:   make([]*Job, 0),
		signal: make(chan struct{}, 1),
	}
}

// Enqueue adds a job to the queue.
func (q *MemoryQueue) Enqueue(ctx context.Context, job *Job) error {
	q.closedMu.RLock()
	if q.closed {
		q.closedMu.RUnlock()
		return ErrQueueClosed
	}
	q.closedMu.RUnlock()

	q.mu.Lock()
	defer q.mu.Unlock()

	// Insert by priority (higher priority first)
	inserted := false
	for i, j := range q.jobs {
		if job.Priority > j.Priority {
			q.jobs = append(q.jobs[:i], append([]*Job{job}, q.jobs[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.jobs = append(q.jobs, job)
	}

	// Signal that a job is available
	select {
	case q.signal <- struct{}{}:
	default:
	}

	return nil
}

// Dequeue removes and returns the next job from the queue.
func (q *MemoryQueue) Dequeue(ctx context.Context) (*Job, error) {
	for {
		q.closedMu.RLock()
		if q.closed {
			q.closedMu.RUnlock()
			return nil, ErrQueueClosed
		}
		q.closedMu.RUnlock()

		q.mu.Lock()
		if len(q.jobs) > 0 {
			job := q.jobs[0]
			q.jobs = q.jobs[1:]
			q.mu.Unlock()
			return job, nil
		}
		q.mu.Unlock()

		// Wait for a job or context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-q.signal:
			// Job may be available, loop again
		}
	}
}

// Peek returns the next job without removing it.
func (q *MemoryQueue) Peek(ctx context.Context) (*Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.jobs) == 0 {
		return nil, nil
	}
	return q.jobs[0], nil
}

// Len returns the number of jobs in the queue.
func (q *MemoryQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// Close closes the queue.
func (q *MemoryQueue) Close() error {
	q.closedMu.Lock()
	defer q.closedMu.Unlock()

	if q.closed {
		return nil
	}
	q.closed = true
	close(q.signal)
	return nil
}

// ErrQueueClosed is returned when operations are performed on a closed queue.
var ErrQueueClosed = &QueueError{message: "queue is closed"}

// QueueError represents a queue-related error.
type QueueError struct {
	message string
}

func (e *QueueError) Error() string {
	return e.message
}
