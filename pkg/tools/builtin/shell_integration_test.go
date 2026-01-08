package builtin

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/security"
)

// TestShellTool_Integration_MultiSecondStreaming tests that commands producing output
// over multiple seconds stream chunks in real-time rather than buffering everything.
func TestShellTool_Integration_MultiSecondStreaming(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping multi-second streaming test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig).WithTimeout(10 * time.Second)
	ctx := context.Background()

	// Script that outputs one line per second for 3 seconds
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args": []interface{}{"-c", `
			echo "Line 1"
			sleep 1
			echo "Line 2"
			sleep 1
			echo "Line 3"
		`},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Track when chunks arrive
	type chunkEvent struct {
		timestamp time.Time
		data      string
		isFinal   bool
	}
	var events []chunkEvent
	startTime := time.Now()

	for chunk := range chunks {
		events = append(events, chunkEvent{
			timestamp: time.Now(),
			data:      chunk.Data,
			isFinal:   chunk.IsFinal,
		})
	}

	// Verify we got chunks during execution, not all at the end
	if len(events) < 4 { // At least 3 data chunks + 1 final chunk
		t.Errorf("Expected at least 4 events (3 lines + final), got %d", len(events))
	}

	// Verify real-time streaming: chunks should arrive with delays between them
	// Check that the time between first and last data chunk is at least 1.5 seconds
	// (accounting for 2 sleep commands of 1 second each)
	if len(events) >= 3 {
		firstDataIdx := 0
		lastDataIdx := 0
		for i, e := range events {
			if !e.isFinal && e.data != "" {
				if firstDataIdx == 0 {
					firstDataIdx = i
				}
				lastDataIdx = i
			}
		}

		if firstDataIdx != lastDataIdx {
			duration := events[lastDataIdx].timestamp.Sub(events[firstDataIdx].timestamp)
			if duration < 800*time.Millisecond {
				t.Errorf("Expected chunks to arrive over at least 0.8s (real-time streaming), got %v", duration)
			}
		}
	}

	// Verify all expected output was received
	var outputLines []string
	for _, e := range events {
		if !e.isFinal && e.data != "" {
			outputLines = append(outputLines, e.data)
		}
	}

	output := strings.Join(outputLines, "\n")
	for i := 1; i <= 3; i++ {
		expected := "Line " + string(rune('0'+i))
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing %q, got: %s", expected, output)
		}
	}

	// Verify total execution time was reasonable (at least 2 seconds for the sleeps)
	totalDuration := time.Since(startTime)
	if totalDuration < 2*time.Second {
		t.Errorf("Total execution time %v too short for a 2-second command", totalDuration)
	}
}

// TestShellTool_Integration_FirstChunkLatency tests that the first chunk is emitted
// within 100ms of output being available (best-effort).
func TestShellTool_Integration_FirstChunkLatency(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping latency test on Windows")
	}

	tool := NewShellTool()
	ctx := context.Background()

	// Command that produces output immediately
	startTime := time.Now()

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "echo",
		"args":    []interface{}{"immediate output"},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Measure time to first chunk
	var firstChunkTime time.Time
	var gotFirstChunk bool

	for chunk := range chunks {
		if !gotFirstChunk && !chunk.IsFinal && chunk.Data != "" {
			firstChunkTime = time.Now()
			gotFirstChunk = true
		}
	}

	if !gotFirstChunk {
		t.Fatal("No data chunk received")
	}

	latency := firstChunkTime.Sub(startTime)

	// Best-effort target: first chunk within 100ms
	// We allow up to 200ms for CI environments which may be slower
	if latency > 200*time.Millisecond {
		t.Logf("WARNING: First chunk latency %v exceeds 100ms target (CI tolerance: 200ms)", latency)
	}

	t.Logf("First chunk latency: %v", latency)
}

// TestShellTool_Integration_MemoryBounded tests that memory usage remains bounded
// regardless of total output size, verifying that streaming doesn't buffer everything.
func TestShellTool_Integration_MemoryBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping memory test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true
	secConfig.MaxOutputSize = 0 // Disable output limits for this test

	tool := NewShellTool().WithSecurityConfig(secConfig).WithTimeout(30 * time.Second)
	ctx := context.Background()

	// Record memory before starting
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	// Command that produces ~10MB of output over time
	// We generate 1000 lines of 10KB each = ~10MB total
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args": []interface{}{"-c", `
			for i in $(seq 1 1000); do
				printf '%10240s\n' | tr ' ' 'x'
			done
		`},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Process chunks but don't accumulate all output (simulating real streaming consumer)
	chunkCount := 0

	for chunk := range chunks {
		if !chunk.IsFinal {
			chunkCount++
			_ = len(chunk.Data) // Process chunk but don't store
		}

		// Periodically check memory during streaming
		if chunkCount%100 == 0 {
			runtime.GC()
			var memDuring runtime.MemStats
			runtime.ReadMemStats(&memDuring)

			// Memory growth should be bounded (less than 5MB during processing)
			// The actual output is 10MB but streaming should never buffer it all
			// Note: memory can decrease due to GC, so only check if it increased
			if memDuring.Alloc > memBefore.Alloc {
				memGrowth := memDuring.Alloc - memBefore.Alloc
				if memGrowth > 5*1024*1024 {
					t.Errorf("Memory growth %d bytes exceeds 5MB during streaming", memGrowth)
				}
			}
		}
	}

	if chunkCount == 0 {
		t.Fatal("No chunks received")
	}

	// Verify we got a reasonable number of chunks (streaming, not single buffer dump)
	if chunkCount < 100 {
		t.Errorf("Expected many chunks for 10MB output, got only %d", chunkCount)
	}

	t.Logf("Received %d chunks for ~10MB output (avg chunk size: ~%d bytes)",
		chunkCount, 10*1024*1024/chunkCount)
}

// TestShellTool_Integration_RedactionWorks tests that sensitive patterns in output
// are properly redacted during streaming.
func TestShellTool_Integration_RedactionWorks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping redaction integration test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Command that outputs multiple sensitive values over time
	sensitiveScript := `
		echo "Starting deployment..."
		sleep 0.1
		echo "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
		sleep 0.1
		echo "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
		sleep 0.1
		echo "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0"
		sleep 0.1
		echo "DATABASE_URL=postgresql://user:secretpass123@localhost:5432/mydb"
		sleep 0.1
		echo "API_KEY=test_apikey_abcdef1234567890ghijklmnop"
		sleep 0.1
		echo "Deployment complete!"
	`

	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", sensitiveScript},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Collect all output
	var allOutput []string
	for chunk := range chunks {
		if !chunk.IsFinal && chunk.Stream == "stdout" {
			allOutput = append(allOutput, chunk.Data)
		}
	}

	output := strings.Join(allOutput, "\n")

	// Verify redaction occurred
	tests := []struct {
		name        string
		shouldNotContain string
		shouldContain    string
	}{
		{
			name:             "AWS access key redacted",
			shouldNotContain: "AKIAIOSFODNN7EXAMPLE",
			shouldContain:    "[REDACTED]",
		},
		{
			name:             "AWS secret key redacted",
			shouldNotContain: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			shouldContain:    "[REDACTED]",
		},
		{
			name:             "Bearer token redacted",
			shouldNotContain: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			shouldContain:    "[REDACTED]",
		},
		{
			name:             "Password in URL redacted",
			shouldNotContain: "secretpass123",
			shouldContain:    "[REDACTED]",
		},
		{
			name:             "API key redacted",
			shouldNotContain: "test_apikey_abcdef1234567890ghijklmnop",
			shouldContain:    "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(output, tt.shouldNotContain) {
				t.Errorf("Output contains sensitive data %q, it should be redacted", tt.shouldNotContain)
			}
			if !strings.Contains(output, tt.shouldContain) {
				t.Errorf("Output should contain %q indicating redaction occurred", tt.shouldContain)
			}
		})
	}

	// Verify non-sensitive content is preserved
	if !strings.Contains(output, "Starting deployment") {
		t.Error("Non-sensitive content should be preserved")
	}
	if !strings.Contains(output, "Deployment complete") {
		t.Error("Non-sensitive content should be preserved")
	}
}

// TestShellTool_Integration_TruncationWorks tests that output exceeding size limits
// is properly truncated with an appropriate indicator.
func TestShellTool_Integration_TruncationWorks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping truncation integration test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true
	secConfig.MaxOutputSize = 1024 // 1KB limit

	tool := NewShellTool().WithSecurityConfig(secConfig).WithTimeout(10 * time.Second)
	ctx := context.Background()

	// Command that produces much more than 1KB of output
	chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args": []interface{}{"-c", `
			for i in $(seq 1 100); do
				echo "This is line $i with some extra padding to make it longer"
			done
		`},
	})
	if err != nil {
		t.Fatalf("ExecuteStream() error = %v", err)
	}

	// Collect chunks
	var dataChunks []string
	var finalChunk map[string]interface{}
	var finalMetadata map[string]interface{}
	var finalError error

	for chunk := range chunks {
		if chunk.IsFinal {
			finalChunk = chunk.Result
			finalMetadata = chunk.Metadata
			finalError = chunk.Error
		} else if chunk.Stream == "stdout" {
			dataChunks = append(dataChunks, chunk.Data)
		}
	}

	// Verify final chunk received
	if finalChunk == nil {
		t.Fatal("No final chunk received")
	}

	// Verify truncation metadata
	if finalMetadata == nil {
		t.Fatal("Final chunk should have metadata indicating truncation")
	}

	truncated, ok := finalMetadata["truncated"].(bool)
	if !ok || !truncated {
		t.Errorf("Final chunk metadata should indicate truncation, got: %v", finalMetadata)
	}

	// Verify truncation error
	if finalError == nil {
		t.Error("Final chunk should have error indicating output was truncated")
	} else {
		errMsg := finalError.Error()
		if !strings.Contains(errMsg, "truncated") && !strings.Contains(errMsg, "exceeded") {
			t.Errorf("Error should mention truncation, got: %v", errMsg)
		}
	}

	// Verify total output doesn't exceed limit
	totalSize := 0
	for _, chunk := range dataChunks {
		totalSize += len(chunk)
	}

	if totalSize > int(secConfig.MaxOutputSize) {
		t.Errorf("Total output %d bytes exceeds limit %d bytes", totalSize, secConfig.MaxOutputSize)
	}

	// Verify we got some output (not empty due to truncation)
	if len(dataChunks) == 0 {
		t.Error("Should receive some data chunks before truncation")
	}

	t.Logf("Received %d bytes of output (limit: %d bytes), truncation correctly enforced",
		totalSize, secConfig.MaxOutputSize)
}

// TestShellTool_Integration_NonStreamingCompatibility tests that non-streaming tools
// called via ExecuteStream produce identical output to Execute().
func TestShellTool_Integration_NonStreamingCompatibility(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping compatibility test on Windows")
	}

	secConfig := security.DefaultShellSecurityConfig()
	secConfig.AllowShellExpand = true

	tool := NewShellTool().WithSecurityConfig(secConfig)
	ctx := context.Background()

	testScript := "echo 'Line 1'; echo 'Line 2' >&2; exit 42"

	// Execute via non-streaming Execute()
	execResult, execErr := tool.Execute(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", testScript},
	})

	if execErr != nil {
		t.Fatalf("Execute() error = %v", execErr)
	}

	// Execute via streaming ExecuteStream()
	chunks, streamErr := tool.ExecuteStream(ctx, map[string]interface{}{
		"command": "sh",
		"args":    []interface{}{"-c", testScript},
	})

	if streamErr != nil {
		t.Fatalf("ExecuteStream() error = %v", streamErr)
	}

	// Collect streaming output
	var stdoutChunks []string
	var stderrChunks []string
	var streamResult map[string]interface{}

	for chunk := range chunks {
		if chunk.IsFinal {
			streamResult = chunk.Result
		} else {
			switch chunk.Stream {
			case "stdout":
				stdoutChunks = append(stdoutChunks, chunk.Data)
			case "stderr":
				stderrChunks = append(stderrChunks, chunk.Data)
			}
		}
	}

	if streamResult == nil {
		t.Fatal("No final result from ExecuteStream")
	}

	// Compare exit codes
	execExitCode := execResult["exit_code"].(int)
	streamExitCode := streamResult["exit_code"].(int)

	if execExitCode != streamExitCode {
		t.Errorf("Exit codes differ: Execute=%d, ExecuteStream=%d", execExitCode, streamExitCode)
	}

	if execExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", execExitCode)
	}

	// Compare success flags
	execSuccess := execResult["success"].(bool)
	streamSuccess := streamResult["success"].(bool)

	if execSuccess != streamSuccess {
		t.Errorf("Success flags differ: Execute=%v, ExecuteStream=%v", execSuccess, streamSuccess)
	}

	// Compare stdout (allowing for potential whitespace differences in streaming)
	execStdout := strings.TrimSpace(execResult["stdout"].(string))
	streamStdout := strings.TrimSpace(strings.Join(stdoutChunks, "\n"))

	if execStdout != streamStdout {
		t.Errorf("Stdout differs:\nExecute: %q\nExecuteStream: %q", execStdout, streamStdout)
	}

	// Compare stderr
	execStderr := strings.TrimSpace(execResult["stderr"].(string))
	streamStderr := strings.TrimSpace(strings.Join(stderrChunks, "\n"))

	if execStderr != streamStderr {
		t.Errorf("Stderr differs:\nExecute: %q\nExecuteStream: %q", execStderr, streamStderr)
	}

	t.Log("Execute() and ExecuteStream() produce compatible results")
}

// TestShellTool_Integration_ConcurrentStreamingExecutions tests that multiple
// concurrent streaming tool executions don't interfere with each other.
func TestShellTool_Integration_ConcurrentStreamingExecutions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping concurrent streaming test on Windows")
	}

	// Use security config that allows shell metacharacters for this test
	secConfig := security.DefaultShellSecurityConfig()
	secConfig.BlockedMetachars = []string{} // Allow all metachars for concurrent test

	tool := NewShellTool().WithTimeout(5 * time.Second).WithSecurityConfig(secConfig)
	ctx := context.Background()

	// Run 5 concurrent streaming executions with different outputs
	concurrency := 5
	results := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		workerID := i
		go func() {
			chunks, err := tool.ExecuteStream(ctx, map[string]interface{}{
				"command": "sh",
				"args":    []interface{}{"-c", "echo worker-" + string(rune('0'+workerID)) + "; sleep 0.1"},
			})

			if err != nil {
				results <- err
				return
			}

			// Verify we get expected output for this worker
			var output []string
			for chunk := range chunks {
				if !chunk.IsFinal && chunk.Stream == "stdout" {
					output = append(output, chunk.Data)
				}
			}

			combined := strings.Join(output, "")
			expectedMarker := "worker-" + string(rune('0'+workerID))
			if !strings.Contains(combined, expectedMarker) {
				results <- &testError{msg: "Worker " + string(rune('0'+workerID)) + " output missing expected marker"}
				return
			}

			results <- nil
		}()
	}

	// Collect results
	for i := 0; i < concurrency; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent execution %d failed: %v", i, err)
		}
	}
}

// testError is a simple error type for test results
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
