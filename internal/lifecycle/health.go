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

package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	// ErrHealthCheckTimeout is returned when health checks exceed the timeout.
	ErrHealthCheckTimeout = errors.New("health check timeout")

	// ErrHealthCheckFailed is returned when the health endpoint returns an error.
	ErrHealthCheckFailed = errors.New("health check failed")
)

// HealthChecker polls a health endpoint with exponential backoff.
type HealthChecker struct {
	endpoint        string
	client          *http.Client
	initialInterval time.Duration
	maxInterval     time.Duration
	multiplier      float64
}

// HealthCheckResult contains the result of a health check attempt.
type HealthCheckResult struct {
	Success      bool
	StatusCode   int
	ResponseTime time.Duration
	Error        error
}

// NewHealthChecker creates a new health checker for the given endpoint.
// Default backoff: 50ms initial, 2x multiplier, 1s max interval.
func NewHealthChecker(endpoint string) *HealthChecker {
	return &HealthChecker{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		initialInterval: 50 * time.Millisecond,
		maxInterval:     1 * time.Second,
		multiplier:      2.0,
	}
}

// WithBackoff configures custom backoff parameters.
func (h *HealthChecker) WithBackoff(initial, max time.Duration, multiplier float64) *HealthChecker {
	h.initialInterval = initial
	h.maxInterval = max
	h.multiplier = multiplier
	return h
}

// WithHTTPClient sets a custom HTTP client.
func (h *HealthChecker) WithHTTPClient(client *http.Client) *HealthChecker {
	h.client = client
	return h
}

// Check performs a single health check.
func (h *HealthChecker) Check(ctx context.Context) *HealthCheckResult {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.endpoint, nil)
	if err != nil {
		return &HealthCheckResult{
			Success: false,
			Error:   fmt.Errorf("failed to create request: %w", err),
		}
	}

	resp, err := h.client.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		return &HealthCheckResult{
			Success:      false,
			ResponseTime: responseTime,
			Error:        fmt.Errorf("request failed: %w", err),
		}
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return &HealthCheckResult{
		Success:      success,
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
		Error:        nil,
	}
}

// WaitUntilHealthy polls the health endpoint until it returns success or timeout is reached.
// Uses exponential backoff: 50ms initial, 2x multiplier, 1s max interval.
func (h *HealthChecker) WaitUntilHealthy(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interval := h.initialInterval
	attempts := 0

	for {
		attempts++
		result := h.Check(ctx)

		if result.Success {
			return nil
		}

		// Check if we've exceeded timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w after %d attempts: %v", ErrHealthCheckTimeout, attempts, result.Error)
		default:
		}

		// Wait before next attempt with exponential backoff
		time.Sleep(interval)

		// Increase interval for next attempt
		interval = time.Duration(float64(interval) * h.multiplier)
		if interval > h.maxInterval {
			interval = h.maxInterval
		}
	}
}

// WaitUntilHealthyWithCallback is like WaitUntilHealthy but calls a callback for each attempt.
// This is useful for logging progress during startup.
func (h *HealthChecker) WaitUntilHealthyWithCallback(timeout time.Duration, callback func(*HealthCheckResult, int)) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interval := h.initialInterval
	attempts := 0

	for {
		attempts++
		result := h.Check(ctx)

		if callback != nil {
			callback(result, attempts)
		}

		if result.Success {
			return nil
		}

		// Check if we've exceeded timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w after %d attempts", ErrHealthCheckTimeout, attempts)
		default:
		}

		// Wait before next attempt with exponential backoff
		time.Sleep(interval)

		// Increase interval for next attempt
		interval = time.Duration(float64(interval) * h.multiplier)
		if interval > h.maxInterval {
			interval = h.maxInterval
		}
	}
}
