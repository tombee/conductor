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
	"fmt"
	"testing"
	"time"
)

// BenchmarkEnqueue_100 benchmarks enqueueing with 100 existing items
func BenchmarkEnqueue_100(b *testing.B) {
	benchmarkEnqueue(b, 100)
}

// BenchmarkEnqueue_1000 benchmarks enqueueing with 1000 existing items
func BenchmarkEnqueue_1000(b *testing.B) {
	benchmarkEnqueue(b, 1000)
}

// BenchmarkEnqueue_10000 benchmarks enqueueing with 10000 existing items
func BenchmarkEnqueue_10000(b *testing.B) {
	benchmarkEnqueue(b, 10000)
}

func benchmarkEnqueue(b *testing.B, queueSize int) {
	q := NewMemoryQueue()
	ctx := context.Background()

	// Pre-fill queue to simulate realistic load
	for i := 0; i < queueSize; i++ {
		job := &Job{
			ID:        fmt.Sprintf("job-%d", i),
			Priority:  i % 10, // Varying priorities
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("failed to pre-fill queue: %v", err)
		}
	}

	b.ResetTimer()

	// Benchmark insertion at the end (lowest priority case - worst case)
	for i := 0; i < b.N; i++ {
		job := &Job{
			ID:        fmt.Sprintf("bench-job-%d", i),
			Priority:  0, // Lowest priority = inserted at end = worst case
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("enqueue failed: %v", err)
		}
	}
}

// BenchmarkEnqueue_HighPriority benchmarks inserting high priority items (best case)
func BenchmarkEnqueue_HighPriority_1000(b *testing.B) {
	q := NewMemoryQueue()
	ctx := context.Background()

	// Pre-fill with low priority items
	for i := 0; i < 1000; i++ {
		job := &Job{
			ID:        fmt.Sprintf("job-%d", i),
			Priority:  0,
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("failed to pre-fill queue: %v", err)
		}
	}

	b.ResetTimer()

	// Benchmark insertion at the front (highest priority - best case)
	for i := 0; i < b.N; i++ {
		job := &Job{
			ID:        fmt.Sprintf("bench-job-%d", i),
			Priority:  100, // Highest priority = inserted at front = best case
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("enqueue failed: %v", err)
		}
	}
}

// BenchmarkDequeue benchmarks removing items from queue
func BenchmarkDequeue_1000(b *testing.B) {
	q := NewMemoryQueue()
	ctx := context.Background()

	// Pre-fill queue
	for i := 0; i < 1000; i++ {
		job := &Job{
			ID:        fmt.Sprintf("job-%d", i),
			Priority:  i % 10,
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("failed to pre-fill queue: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := q.Dequeue(ctx)
		if err != nil {
			b.Fatalf("dequeue failed: %v", err)
		}

		// Re-fill to maintain consistent queue size
		job := &Job{
			ID:        fmt.Sprintf("refill-%d", i),
			Priority:  i % 10,
			CreatedAt: time.Now(),
		}
		if err := q.Enqueue(ctx, job); err != nil {
			b.Fatalf("re-fill failed: %v", err)
		}
	}
}
