package file

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	operationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "conductor_file_operation_duration_seconds",
			Help:    "Duration of file operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	bytesRead = promauto.NewCounter(prometheus.CounterOpts{
		Name: "conductor_file_bytes_read_total",
		Help: "Total bytes read from files",
	})

	bytesWritten = promauto.NewCounter(prometheus.CounterOpts{
		Name: "conductor_file_bytes_written_total",
		Help: "Total bytes written to files",
	})

	errorsByType = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conductor_file_errors_total",
			Help: "Total file operation errors by type",
		},
		[]string{"error_type"},
	)
)

// recordMetrics records metrics for a file operation
func recordMetrics(operation string, duration float64, status string, bytesReadCount int64, bytesWrittenCount int64, errType ErrorType) {
	// Record operation duration
	operationDuration.WithLabelValues(operation, status).Observe(duration)

	// Record bytes read/written
	if bytesReadCount > 0 {
		bytesRead.Add(float64(bytesReadCount))
	}
	if bytesWrittenCount > 0 {
		bytesWritten.Add(float64(bytesWrittenCount))
	}

	// Record errors by type
	if status == "error" && errType != "" {
		errorsByType.WithLabelValues(string(errType)).Inc()
	}
}
