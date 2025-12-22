package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordPersistenceError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		errorType string
	}{
		{
			name:      "UpdateRun error",
			operation: "UpdateRun",
			errorType: "io_error",
		},
		{
			name:      "CleanupCheckpoint error",
			operation: "CleanupCheckpoint",
			errorType: "not_found",
		},
		{
			name:      "CleanupCache error",
			operation: "CleanupCache",
			errorType: "permission_denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get initial count
			initialCount := testutil.ToFloat64(persistenceErrors.With(prometheus.Labels{
				"operation":  tt.operation,
				"error_type": tt.errorType,
			}))

			// Record error
			RecordPersistenceError(tt.operation, tt.errorType)

			// Verify increment
			newCount := testutil.ToFloat64(persistenceErrors.With(prometheus.Labels{
				"operation":  tt.operation,
				"error_type": tt.errorType,
			}))

			if newCount != initialCount+1 {
				t.Errorf("expected count to increment by 1, got initial=%f, new=%f", initialCount, newCount)
			}
		})
	}
}

func TestRecordPersistenceError_MultipleIncrements(t *testing.T) {
	operation := "UpdateRun"
	errorType := "test_error"

	// Get initial count
	initialCount := testutil.ToFloat64(persistenceErrors.With(prometheus.Labels{
		"operation":  operation,
		"error_type": errorType,
	}))

	// Record multiple errors
	for i := 0; i < 5; i++ {
		RecordPersistenceError(operation, errorType)
	}

	// Verify all increments
	newCount := testutil.ToFloat64(persistenceErrors.With(prometheus.Labels{
		"operation":  operation,
		"error_type": errorType,
	}))

	if newCount != initialCount+5 {
		t.Errorf("expected count to increment by 5, got initial=%f, new=%f", initialCount, newCount)
	}
}
