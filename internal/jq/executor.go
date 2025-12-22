// Package jq provides shared jq expression execution utilities.
package jq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/itchyny/gojq"
)

const (
	// DefaultTimeout is the default execution time for jq expressions (1 second)
	DefaultTimeout = 1 * time.Second

	// DefaultMaxInputSize is the default maximum input size for transforms (10MB)
	DefaultMaxInputSize = 10 * 1024 * 1024
)

// Executor handles jq expression evaluation with timeout and size limits.
type Executor struct {
	timeout      time.Duration
	maxInputSize int64
}

// NewExecutor creates a new jq executor with the given configuration.
func NewExecutor(timeout time.Duration, maxInputSize int64) *Executor {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if maxInputSize == 0 {
		maxInputSize = DefaultMaxInputSize
	}

	return &Executor{
		timeout:      timeout,
		maxInputSize: maxInputSize,
	}
}

// Execute runs a jq expression against the given data with timeout protection.
func (e *Executor) Execute(ctx context.Context, expression string, data interface{}) (interface{}, error) {
	if expression == "" {
		// No transform specified, return data as-is
		return data, nil
	}

	// Validate input size
	if err := e.validateInputSize(data); err != nil {
		return nil, err
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Parse the jq expression
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Compile the query
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("compile error: %w", err)
	}

	// Execute with timeout
	resultChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)

	go func() {
		// Run the query
		iter := code.Run(data)

		// Collect results
		var results []interface{}
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}

			// Check for errors
			if err, isErr := v.(error); isErr {
				errorChan <- err
				return
			}

			results = append(results, v)
		}

		// If single result, return it directly
		// If multiple results, return as array
		if len(results) == 0 {
			resultChan <- nil
		} else if len(results) == 1 {
			resultChan <- results[0]
		} else {
			resultChan <- results
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return nil, err
	case <-execCtx.Done():
		return nil, fmt.Errorf("execution timeout after %v", e.timeout)
	}
}

// Validate validates a jq expression by attempting to compile it.
// This is used during workflow validation to catch syntax errors early.
func (e *Executor) Validate(expression string) error {
	if expression == "" {
		return nil
	}

	query, err := gojq.Parse(expression)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}

	_, err = gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("jq compilation failed: %w", err)
	}

	return nil
}

// validateInputSize checks if the data size is within limits.
func (e *Executor) validateInputSize(data interface{}) error {
	// Estimate size by marshaling to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if int64(len(jsonData)) > e.maxInputSize {
		return fmt.Errorf("data size (%d bytes) exceeds maximum (%d bytes)",
			len(jsonData), e.maxInputSize)
	}

	return nil
}
