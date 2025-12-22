package file

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestQuotaTracker_Basic(t *testing.T) {
	config := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}

	tracker := NewQuotaTracker(config)
	tracker.SetQuota("/tmp/test", 1000)

	// Should allow write within quota
	err := tracker.TrackWrite("/tmp/test/file.txt", 500)
	if err != nil {
		t.Errorf("Expected no error for write within quota, got: %v", err)
	}

	// Check usage
	used, quota, percent := tracker.GetUsage("/tmp/test")
	if used != 500 {
		t.Errorf("Expected 500 bytes used, got %d", used)
	}
	if quota != 1000 {
		t.Errorf("Expected quota of 1000, got %d", quota)
	}
	if percent != 0.5 {
		t.Errorf("Expected 50%% usage, got %.2f%%", percent*100)
	}
}

func TestQuotaTracker_Warning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	config := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
		Logger:         logger,
	}

	tracker := NewQuotaTracker(config)
	tracker.SetQuota("/tmp/test", 1000)

	// Write up to 79% - should not warn
	err := tracker.TrackWrite("/tmp/test/file.txt", 790)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if buf.Len() > 0 {
		t.Errorf("Expected no warning at 79%%, got: %s", buf.String())
	}

	// Write to cross 80% threshold - should warn
	err = tracker.TrackWrite("/tmp/test/file2.txt", 20)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "disk quota warning") {
		t.Errorf("Expected warning log, got: %s", output)
	}
}

func TestQuotaTracker_Error(t *testing.T) {
	config := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}

	tracker := NewQuotaTracker(config)
	tracker.SetQuota("/tmp/test", 1000)

	// Write up to 94% - should succeed
	err := tracker.TrackWrite("/tmp/test/file.txt", 940)
	if err != nil {
		t.Errorf("Expected no error at 94%%, got: %v", err)
	}

	// Try to write beyond 95% - should fail
	err = tracker.TrackWrite("/tmp/test/file2.txt", 60)
	if err == nil {
		t.Error("Expected error when exceeding quota threshold")
	}

	if opErr, ok := err.(*OperationError); ok {
		if opErr.ErrorType != ErrorTypeDiskFull {
			t.Errorf("Expected ErrorTypeDiskFull, got %s", opErr.ErrorType)
		}
	} else {
		t.Errorf("Expected OperationError, got %T", err)
	}
}

func TestQuotaTracker_NoQuota(t *testing.T) {
	tracker := NewQuotaTracker(nil)

	// Should allow any write when no quota is set
	err := tracker.TrackWrite("/tmp/test/file.txt", 1000000)
	if err != nil {
		t.Errorf("Expected no error when no quota set, got: %v", err)
	}
}

func TestQuotaTracker_PrefixMatching(t *testing.T) {
	config := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}

	tracker := NewQuotaTracker(config)
	tracker.SetQuota("/tmp/out", 1000)
	tracker.SetQuota("/tmp/temp", 500)

	// Write to /tmp/out
	err := tracker.TrackWrite("/tmp/out/file1.txt", 300)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Write to /tmp/temp
	err = tracker.TrackWrite("/tmp/temp/file2.txt", 200)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check separate tracking
	used1, _, _ := tracker.GetUsage("/tmp/out")
	if used1 != 300 {
		t.Errorf("Expected 300 bytes in /tmp/out, got %d", used1)
	}

	used2, _, _ := tracker.GetUsage("/tmp/temp")
	if used2 != 200 {
		t.Errorf("Expected 200 bytes in /tmp/temp, got %d", used2)
	}
}

func TestQuotaTracker_MostSpecificPrefix(t *testing.T) {
	config := &QuotaConfig{
		DefaultQuota:   1000,
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}

	tracker := NewQuotaTracker(config)
	tracker.SetQuota("/tmp", 10000)           // broad quota
	tracker.SetQuota("/tmp/out", 1000)        // specific quota

	// Write to /tmp/out/file.txt should use the more specific quota
	err := tracker.TrackWrite("/tmp/out/file.txt", 500)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that it's tracked under the specific quota
	used, quota, _ := tracker.GetUsage("/tmp/out")
	if used != 500 {
		t.Errorf("Expected 500 bytes in /tmp/out, got %d", used)
	}
	if quota != 1000 {
		t.Errorf("Expected quota of 1000, got %d", quota)
	}

	// Broad quota should not be affected
	usedBroad, _, _ := tracker.GetUsage("/tmp")
	if usedBroad != 0 {
		t.Errorf("Expected 0 bytes in /tmp, got %d", usedBroad)
	}
}

func TestQuotaTracker_Reset(t *testing.T) {
	tracker := NewQuotaTracker(nil)
	tracker.SetQuota("/tmp/test", 1000)

	// Write some data
	err := tracker.TrackWrite("/tmp/test/file.txt", 500)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Reset
	tracker.Reset()

	// Usage should be zero
	used, quota, _ := tracker.GetUsage("/tmp/test")
	if used != 0 {
		t.Errorf("Expected 0 bytes after reset, got %d", used)
	}
	if quota != 1000 {
		t.Errorf("Expected quota still set to 1000, got %d", quota)
	}
}
