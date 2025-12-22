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

package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/pkg/observability"
)

func TestSQLiteStore_WithEncryption(t *testing.T) {
	// Generate and set encryption key
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	os.Setenv("CONDUCTOR_TRACE_KEY", key.String())
	defer os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Create store with encryption enabled
	store, err := New(Config{
		Path:             ":memory:",
		EnableEncryption: true,
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create a span with sensitive data
	span := &observability.Span{
		TraceID:   "trace-123",
		SpanID:    "span-456",
		Name:      "test-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code:    observability.StatusCodeOK,
			Message: "",
		},
		Attributes: map[string]any{
			"api_key":  "secret-key-12345",
			"password": "super-secret",
			"user_id":  "user-789",
		},
		Events: []observability.Event{
			{
				Name:      "login",
				Timestamp: time.Now(),
				Attributes: map[string]any{
					"session_token": "token-xyz",
				},
			},
		},
	}

	// Store the span
	err = store.StoreSpan(ctx, span)
	require.NoError(t, err)

	// Verify data is encrypted in database
	var encryptedAttrs []byte
	err = store.DB().QueryRowContext(ctx,
		"SELECT attributes FROM spans WHERE trace_id = ? AND span_id = ?",
		span.TraceID, span.SpanID,
	).Scan(&encryptedAttrs)
	require.NoError(t, err)

	// Encrypted data should not contain plaintext secrets
	encryptedStr := string(encryptedAttrs)
	assert.NotContains(t, encryptedStr, "secret-key-12345")
	assert.NotContains(t, encryptedStr, "super-secret")

	// Verify we can retrieve and decrypt
	retrieved, err := store.GetSpan(ctx, span.TraceID, span.SpanID)
	require.NoError(t, err)
	assert.Equal(t, span.Attributes["api_key"], retrieved.Attributes["api_key"])
	assert.Equal(t, span.Attributes["password"], retrieved.Attributes["password"])
	assert.Len(t, retrieved.Events, 1)
	assert.Equal(t, "token-xyz", retrieved.Events[0].Attributes["session_token"])
}

func TestSQLiteStore_WithoutEncryption(t *testing.T) {
	os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Create store without encryption
	store, err := New(Config{
		Path:             ":memory:",
		EnableEncryption: false,
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	span := &observability.Span{
		TraceID:   "trace-123",
		SpanID:    "span-456",
		Name:      "test-operation",
		Kind:      observability.SpanKindInternal,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Status: observability.SpanStatus{
			Code: observability.StatusCodeOK,
		},
		Attributes: map[string]any{
			"key": "value",
		},
	}

	err = store.StoreSpan(ctx, span)
	require.NoError(t, err)

	// Verify we can retrieve
	retrieved, err := store.GetSpan(ctx, span.TraceID, span.SpanID)
	require.NoError(t, err)
	assert.Equal(t, span.Attributes["key"], retrieved.Attributes["key"])
}

func TestSQLiteStore_EncryptionKeyRequired(t *testing.T) {
	os.Unsetenv("CONDUCTOR_TRACE_KEY")

	// Should fail when encryption is enabled but no key is set
	_, err := New(Config{
		Path:             ":memory:",
		EnableEncryption: true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no key found")
}

func TestSQLiteStore_LargeTraceWithEncryption(t *testing.T) {
	// Generate and set encryption key
	key, err := GenerateEncryptionKey()
	require.NoError(t, err)

	os.Setenv("CONDUCTOR_TRACE_KEY", key.String())
	defer os.Unsetenv("CONDUCTOR_TRACE_KEY")

	store, err := New(Config{
		Path:             ":memory:",
		EnableEncryption: true,
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	traceID := "large-trace"

	// Create a trace with many spans
	for i := 0; i < 100; i++ {
		span := &observability.Span{
			TraceID:   traceID,
			SpanID:    time.Now().Format("span-" + time.RFC3339Nano),
			Name:      "operation",
			Kind:      observability.SpanKindInternal,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(10 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{
				"iteration": i,
				"data":      "some data here",
			},
		}

		err = store.StoreSpan(ctx, span)
		require.NoError(t, err)
	}

	// Retrieve all spans
	spans, err := store.GetTraceSpans(ctx, traceID)
	require.NoError(t, err)
	assert.Len(t, spans, 100)

	// Verify they're all decrypted correctly
	for _, span := range spans {
		assert.NotNil(t, span.Attributes["iteration"])
		assert.Equal(t, "some data here", span.Attributes["data"])
	}
}

func TestSQLiteStore_ConcurrentAccess(t *testing.T) {
	// Use a temporary file for concurrent access tests
	tmpfile := t.TempDir() + "/test.db"

	store, err := New(Config{
		Path:         tmpfile,
		MaxOpenConns: 5,
	})
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Concurrent reads and sequential writes
	// First, write some data
	for i := 0; i < 10; i++ {
		span := &observability.Span{
			TraceID:   "concurrent-trace",
			SpanID:    time.Now().Format("span-" + time.RFC3339Nano),
			Name:      "concurrent-op",
			Kind:      observability.SpanKindInternal,
			StartTime: time.Now(),
			EndTime:   time.Now().Add(1 * time.Millisecond),
			Status: observability.SpanStatus{
				Code: observability.StatusCodeOK,
			},
			Attributes: map[string]any{
				"worker": i,
			},
		}
		err := store.StoreSpan(ctx, span)
		require.NoError(t, err)
	}

	// Now test concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			spans, err := store.GetTraceSpans(ctx, "concurrent-trace")
			assert.NoError(t, err)
			assert.GreaterOrEqual(t, len(spans), 1)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all spans were stored
	spans, err := store.GetTraceSpans(ctx, "concurrent-trace")
	require.NoError(t, err)
	assert.Len(t, spans, 10)
}
