package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	persistenceErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_persistence_errors_total",
			Help: "Total persistence operation errors by operation and error type",
		},
		[]string{"operation", "error_type"},
	)
)

// RecordPersistenceError increments the persistence error counter.
// operation should be one of: UpdateRun, CleanupCheckpoint, CleanupCache
// errorType is derived from the error (e.g., "context_canceled", "io_error", "unknown")
func RecordPersistenceError(operation, errorType string) {
	persistenceErrors.WithLabelValues(operation, errorType).Inc()
}
