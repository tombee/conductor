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

package filewatcher

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDebouncer_SingleEvent(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(50*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})
	defer debouncer.Stop()

	ctx := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	debouncer.Add(ctx)

	// Wait for debounce window
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 1)
	assert.Equal(t, "/tmp/test.txt", flushed[0].Path)
	assert.Equal(t, "modified", flushed[0].Event)
}

func TestDebouncer_MultipleEventsNonBatch(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(50*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})
	defer debouncer.Stop()

	// Add multiple events for same file rapidly
	ctx1 := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	ctx2 := NewContext("/tmp/test.txt", "modified", false, 200, time.Now())
	ctx3 := NewContext("/tmp/test.txt", "modified", false, 300, time.Now())

	debouncer.Add(ctx1)
	time.Sleep(10 * time.Millisecond)
	debouncer.Add(ctx2)
	time.Sleep(10 * time.Millisecond)
	debouncer.Add(ctx3)

	// Wait for debounce window after last event
	time.Sleep(100 * time.Millisecond)

	// Should only get the last event in non-batch mode
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 1)
	assert.Equal(t, int64(300), flushed[0].Size)
}

func TestDebouncer_MultipleEventsBatch(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(50*time.Millisecond, true, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})
	defer debouncer.Stop()

	// Add multiple events for same file rapidly
	ctx1 := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	ctx2 := NewContext("/tmp/test.txt", "modified", false, 200, time.Now())
	ctx3 := NewContext("/tmp/test.txt", "modified", false, 300, time.Now())

	debouncer.Add(ctx1)
	time.Sleep(10 * time.Millisecond)
	debouncer.Add(ctx2)
	time.Sleep(10 * time.Millisecond)
	debouncer.Add(ctx3)

	// Wait for debounce window after last event
	time.Sleep(100 * time.Millisecond)

	// Should get all events in batch mode
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 3)
	assert.Equal(t, int64(100), flushed[0].Size)
	assert.Equal(t, int64(200), flushed[1].Size)
	assert.Equal(t, int64(300), flushed[2].Size)
}

func TestDebouncer_MultiplePaths(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(50*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})
	defer debouncer.Stop()

	// Add events for different files
	ctx1 := NewContext("/tmp/file1.txt", "modified", false, 100, time.Now())
	ctx2 := NewContext("/tmp/file2.txt", "modified", false, 200, time.Now())

	debouncer.Add(ctx1)
	debouncer.Add(ctx2)

	// Wait for debounce window
	time.Sleep(100 * time.Millisecond)

	// Should get both events (different paths have independent timers)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 2)
	paths := map[string]bool{
		flushed[0].Path: true,
		flushed[1].Path: true,
	}
	assert.True(t, paths["/tmp/file1.txt"])
	assert.True(t, paths["/tmp/file2.txt"])
}

func TestDebouncer_Stop(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(100*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})

	// Add event but stop before debounce window expires
	ctx := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	debouncer.Add(ctx)

	time.Sleep(20 * time.Millisecond)
	debouncer.Stop()

	// Should flush pending event on stop
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 1)
	assert.Equal(t, "/tmp/test.txt", flushed[0].Path)
}

func TestDebouncer_StopFlushesAllPending(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(100*time.Millisecond, true, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})

	// Add multiple events for same file
	ctx1 := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	ctx2 := NewContext("/tmp/test.txt", "modified", false, 200, time.Now())
	debouncer.Add(ctx1)
	time.Sleep(10 * time.Millisecond)
	debouncer.Add(ctx2)

	// Add event for different file
	ctx3 := NewContext("/tmp/other.txt", "created", false, 300, time.Now())
	debouncer.Add(ctx3)

	time.Sleep(20 * time.Millisecond)
	debouncer.Stop()

	// Should flush all pending events in batch mode
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 3)
}

func TestDebouncer_NoEventsAfterStop(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(50*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})

	debouncer.Stop()

	// Try to add event after stop
	ctx := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	debouncer.Add(ctx)

	time.Sleep(100 * time.Millisecond)

	// Should not receive any events after stop
	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, flushed)
}

func TestDebouncer_Pending(t *testing.T) {
	debouncer := NewDebouncer(100*time.Millisecond, false, func(events []*Context) {})
	defer debouncer.Stop()

	assert.Equal(t, 0, debouncer.Pending())

	ctx1 := NewContext("/tmp/file1.txt", "modified", false, 100, time.Now())
	ctx2 := NewContext("/tmp/file2.txt", "modified", false, 200, time.Now())

	debouncer.Add(ctx1)
	assert.Equal(t, 1, debouncer.Pending())

	debouncer.Add(ctx2)
	assert.Equal(t, 2, debouncer.Pending())

	// Wait for timers to fire
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, 0, debouncer.Pending())
}

func TestDebouncer_ResetTimer(t *testing.T) {
	var mu sync.Mutex
	var flushed []*Context
	debouncer := NewDebouncer(100*time.Millisecond, false, func(events []*Context) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, events...)
	})
	defer debouncer.Stop()

	// Add first event
	ctx1 := NewContext("/tmp/test.txt", "modified", false, 100, time.Now())
	debouncer.Add(ctx1)

	// Wait 70ms (less than debounce window)
	time.Sleep(70 * time.Millisecond)

	// Add second event - should reset the timer
	ctx2 := NewContext("/tmp/test.txt", "modified", false, 200, time.Now())
	debouncer.Add(ctx2)

	// Wait another 70ms (total 140ms from first event, but only 70ms from second)
	time.Sleep(70 * time.Millisecond)

	// Should not have flushed yet because timer was reset
	mu.Lock()
	assert.Empty(t, flushed)
	mu.Unlock()

	// Wait for remaining debounce window
	time.Sleep(50 * time.Millisecond)

	// Now should have flushed with the second event
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 1)
	assert.Equal(t, int64(200), flushed[0].Size)
}
