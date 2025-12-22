// Package leader provides leader election for distributed controller deployments.
package leader

import (
	"testing"
	"time"
)

func TestAdvisoryLockID(t *testing.T) {
	// Verify the lock ID is a valid non-zero value
	if AdvisoryLockID == 0 {
		t.Error("AdvisoryLockID should not be zero")
	}

	// The value should be consistent (not random)
	expected := int64(0x636F6E6475637464)
	if AdvisoryLockID != expected {
		t.Errorf("AdvisoryLockID = %x, want %x", AdvisoryLockID, expected)
	}
}

func TestNewElector(t *testing.T) {
	t.Run("with default retry interval", func(t *testing.T) {
		cfg := Config{
			DB:         nil, // Would need real DB for full test
			InstanceID: "test-instance",
		}

		e := NewElector(cfg)
		if e == nil {
			t.Fatal("NewElector() returned nil")
		}

		if e.instanceID != "test-instance" {
			t.Errorf("instanceID = %v, want test-instance", e.instanceID)
		}
	})

	t.Run("with custom retry interval", func(t *testing.T) {
		cfg := Config{
			InstanceID:    "test-instance",
			RetryInterval: 10 * time.Second,
		}

		e := NewElector(cfg)
		if e == nil {
			t.Fatal("NewElector() returned nil")
		}
	})

	t.Run("with zero retry interval uses default", func(t *testing.T) {
		cfg := Config{
			InstanceID:    "test-instance",
			RetryInterval: 0,
		}

		e := NewElector(cfg)
		if e == nil {
			t.Fatal("NewElector() returned nil")
		}
		// Default should be 5 seconds - can't directly verify but constructor should handle it
	})

	t.Run("negative retry interval uses default", func(t *testing.T) {
		cfg := Config{
			InstanceID:    "test-instance",
			RetryInterval: -1 * time.Second,
		}

		e := NewElector(cfg)
		if e == nil {
			t.Fatal("NewElector() returned nil")
		}
	})
}

func TestElector_IsLeader_Initial(t *testing.T) {
	cfg := Config{
		InstanceID: "test-instance",
	}

	e := NewElector(cfg)

	// Initially should not be leader
	if e.IsLeader() {
		t.Error("New elector should not be leader")
	}
}

func TestElector_OnLeadershipChange(t *testing.T) {
	cfg := Config{
		InstanceID: "test-instance",
	}

	e := NewElector(cfg)

	e.OnLeadershipChange(func(isLeader bool) {
		// Callback registered
	})

	// Verify callback was registered
	if len(e.callbacks) != 1 {
		t.Errorf("callbacks length = %d, want 1", len(e.callbacks))
	}

	// Add another callback
	e.OnLeadershipChange(func(isLeader bool) {})
	if len(e.callbacks) != 2 {
		t.Errorf("callbacks length = %d, want 2", len(e.callbacks))
	}
}

func TestElector_setLeader(t *testing.T) {
	cfg := Config{
		InstanceID: "test-instance",
	}

	e := NewElector(cfg)

	// Track callback invocations
	var callbackValue *bool
	e.OnLeadershipChange(func(isLeader bool) {
		val := isLeader
		callbackValue = &val
	})

	t.Run("transition to leader", func(t *testing.T) {
		callbackValue = nil
		e.setLeader(true)

		if !e.IsLeader() {
			t.Error("IsLeader() should be true after setLeader(true)")
		}

		if callbackValue == nil || !*callbackValue {
			t.Error("Callback should be called with true")
		}
	})

	t.Run("no callback when already leader", func(t *testing.T) {
		callbackValue = nil
		e.setLeader(true) // Already leader

		if callbackValue != nil {
			t.Error("Callback should not be called when status doesn't change")
		}
	})

	t.Run("transition from leader", func(t *testing.T) {
		callbackValue = nil
		e.setLeader(false)

		if e.IsLeader() {
			t.Error("IsLeader() should be false after setLeader(false)")
		}

		if callbackValue == nil || *callbackValue {
			t.Error("Callback should be called with false")
		}
	})

	t.Run("no callback when already not leader", func(t *testing.T) {
		callbackValue = nil
		e.setLeader(false) // Already not leader

		if callbackValue != nil {
			t.Error("Callback should not be called when status doesn't change")
		}
	})
}

func TestElector_Status(t *testing.T) {
	cfg := Config{
		InstanceID: "my-instance",
	}

	e := NewElector(cfg)

	status := e.Status()

	if status.InstanceID != "my-instance" {
		t.Errorf("InstanceID = %v, want my-instance", status.InstanceID)
	}

	if status.IsLeader {
		t.Error("IsLeader should be false for new elector")
	}

	// Set to leader and check again
	e.setLeader(true)
	status = e.Status()

	if !status.IsLeader {
		t.Error("IsLeader should be true after setLeader(true)")
	}
}

func TestLeaderStatus_Fields(t *testing.T) {
	now := time.Now()
	status := LeaderStatus{
		InstanceID: "node-1",
		IsLeader:   true,
		AcquiredAt: now,
	}

	if status.InstanceID != "node-1" {
		t.Errorf("InstanceID = %v, want node-1", status.InstanceID)
	}
	if !status.IsLeader {
		t.Error("IsLeader should be true")
	}
	if !status.AcquiredAt.Equal(now) {
		t.Error("AcquiredAt not preserved")
	}
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		InstanceID:    "test-id",
		RetryInterval: 10 * time.Second,
	}

	if cfg.InstanceID != "test-id" {
		t.Errorf("InstanceID = %v, want test-id", cfg.InstanceID)
	}
	if cfg.RetryInterval != 10*time.Second {
		t.Errorf("RetryInterval = %v, want 10s", cfg.RetryInterval)
	}
}

// Note: The following tests would require a real PostgreSQL database connection:
// - TestElector_Start (requires DB for advisory lock operations)
// - TestElector_Stop (requires running election loop)
// - TestElector_tryAcquireLeadership (requires pg_try_advisory_lock)
// - TestElector_verifyLeadership (requires pg_locks query)
// - TestElector_releaseLeadership (requires pg_advisory_unlock)
// - TestTryAcquireLock (requires DB)
// - TestReleaseLock (requires DB)
// - TestWithLock (requires DB)
//
// These would be integration tests that run against a test database.
// For unit testing, we verify the struct initialization and state management logic.
