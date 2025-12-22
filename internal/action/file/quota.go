package file

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
)

// QuotaTracker tracks disk usage and enforces quotas
type QuotaTracker struct {
	mu             sync.Mutex
	bytesWritten   map[string]int64 // path prefix -> bytes
	quotas         map[string]int64 // path prefix -> max bytes
	warnThreshold  float64          // 0.8 (80%)
	errorThreshold float64          // 0.95 (95%)
	logger         *slog.Logger     // optional logger for warnings
}

// QuotaConfig holds quota configuration
type QuotaConfig struct {
	DefaultQuota   int64        // default quota in bytes (100MB)
	WarnThreshold  float64      // threshold for warnings (0.8 = 80%)
	ErrorThreshold float64      // threshold for errors (0.95 = 95%)
	Logger         *slog.Logger // optional logger
}

// DefaultQuotaConfig returns sensible defaults
func DefaultQuotaConfig() *QuotaConfig {
	return &QuotaConfig{
		DefaultQuota:   100 * 1024 * 1024, // 100MB
		WarnThreshold:  0.8,
		ErrorThreshold: 0.95,
	}
}

// NewQuotaTracker creates a new quota tracker
func NewQuotaTracker(config *QuotaConfig) *QuotaTracker {
	if config == nil {
		config = DefaultQuotaConfig()
	}

	return &QuotaTracker{
		bytesWritten:   make(map[string]int64),
		quotas:         make(map[string]int64),
		warnThreshold:  config.WarnThreshold,
		errorThreshold: config.ErrorThreshold,
		logger:         config.Logger,
	}
}

// SetQuota sets the quota for a path prefix
func (q *QuotaTracker) SetQuota(pathPrefix string, quota int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Normalize path prefix
	normalized := filepath.Clean(pathPrefix)
	q.quotas[normalized] = quota
}

// TrackWrite records bytes written and checks quota
func (q *QuotaTracker) TrackWrite(path string, bytes int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Find the matching quota prefix
	prefix, quota := q.findQuotaPrefix(path)
	if quota == 0 {
		// No quota configured for this path
		return nil
	}

	// Update bytes written
	currentBytes := q.bytesWritten[prefix]
	newBytes := currentBytes + bytes

	// Calculate usage percentage
	usage := float64(newBytes) / float64(quota)

	// Check error threshold first (blocks the write)
	if usage >= q.errorThreshold {
		return &OperationError{
			Message: fmt.Sprintf("disk quota exceeded: %.1f%% of %d bytes used in %s (would exceed %.0f%% threshold)",
				usage*100, quota, prefix, q.errorThreshold*100),
			ErrorType: ErrorTypeDiskFull,
		}
	}

	// Check warn threshold (logs but allows the write)
	if usage >= q.warnThreshold && currentBytes < int64(float64(quota)*q.warnThreshold) {
		// Only warn when crossing the threshold
		q.logWarning(prefix, usage, quota)
	}

	// Update tracked bytes
	q.bytesWritten[prefix] = newBytes

	return nil
}

// findQuotaPrefix finds the quota that applies to the given path
func (q *QuotaTracker) findQuotaPrefix(path string) (string, int64) {
	normalized := filepath.Clean(path)

	// Look for the most specific matching prefix
	var bestPrefix string
	var bestQuota int64

	for prefix, quota := range q.quotas {
		// Check if path is under this prefix
		if strings.HasPrefix(normalized, prefix) {
			// Keep the longest matching prefix (most specific)
			if len(prefix) > len(bestPrefix) {
				bestPrefix = prefix
				bestQuota = quota
			}
		}
	}

	return bestPrefix, bestQuota
}

// logWarning logs a warning when approaching quota limit
func (q *QuotaTracker) logWarning(prefix string, usage float64, quota int64) {
	if q.logger != nil {
		q.logger.Warn("disk quota warning",
			slog.String("path_prefix", prefix),
			slog.Float64("usage_percent", usage*100),
			slog.Int64("quota_bytes", quota),
			slog.Float64("threshold_percent", q.warnThreshold*100),
		)
	}
}

// GetUsage returns current usage for a path prefix
func (q *QuotaTracker) GetUsage(pathPrefix string) (used int64, quota int64, percent float64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	normalized := filepath.Clean(pathPrefix)
	used = q.bytesWritten[normalized]
	quota = q.quotas[normalized]

	if quota > 0 {
		percent = float64(used) / float64(quota)
	}

	return used, quota, percent
}

// Reset clears all tracked bytes (useful for testing or workflow restart)
func (q *QuotaTracker) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.bytesWritten = make(map[string]int64)
}
