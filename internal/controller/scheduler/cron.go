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

package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr represents a parsed cron expression.
type CronExpr struct {
	minute     []int // 0-59
	hour       []int // 0-23
	dayOfMonth []int // 1-31
	month      []int // 1-12
	dayOfWeek  []int // 0-6 (0 = Sunday)
}

// ParseCron parses a cron expression.
// Format: minute hour day-of-month month day-of-week
// Examples:
//   - "0 * * * *" - every hour at minute 0
//   - "*/15 * * * *" - every 15 minutes
//   - "0 9 * * 1-5" - 9 AM on weekdays
//   - "0 0 1 * *" - midnight on the first of each month
func ParseCron(expr string) (*CronExpr, error) {
	// Handle special expressions
	switch strings.ToLower(expr) {
	case "@hourly":
		expr = "0 * * * *"
	case "@daily", "@midnight":
		expr = "0 0 * * *"
	case "@weekly":
		expr = "0 0 * * 0"
	case "@monthly":
		expr = "0 0 1 * *"
	case "@yearly", "@annually":
		expr = "0 0 1 1 *"
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	c := &CronExpr{}
	var err error

	c.minute, err = parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}

	c.hour, err = parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}

	c.dayOfMonth, err = parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}

	c.month, err = parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}

	c.dayOfWeek, err = parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	return c, nil
}

// parseField parses a single cron field.
func parseField(field string, min, max int) ([]int, error) {
	// Handle wildcard
	if field == "*" {
		result := make([]int, max-min+1)
		for i := range result {
			result[i] = min + i
		}
		return result, nil
	}

	var result []int

	// Handle comma-separated values
	parts := strings.Split(field, ",")
	for _, part := range parts {
		values, err := parseFieldPart(part, min, max)
		if err != nil {
			return nil, err
		}
		result = append(result, values...)
	}

	// Remove duplicates and sort
	result = unique(result)
	return result, nil
}

// parseFieldPart parses a single part of a cron field (handles ranges and steps).
func parseFieldPart(part string, min, max int) ([]int, error) {
	// Handle step values (*/5 or 1-10/2)
	var step int = 1
	if idx := strings.Index(part, "/"); idx != -1 {
		stepStr := part[idx+1:]
		var err error
		step, err = strconv.Atoi(stepStr)
		if err != nil || step <= 0 {
			return nil, fmt.Errorf("invalid step: %s", stepStr)
		}
		part = part[:idx]
	}

	var start, end int

	if part == "*" {
		start = min
		end = max
	} else if idx := strings.Index(part, "-"); idx != -1 {
		// Range
		var err error
		start, err = strconv.Atoi(part[:idx])
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", part[:idx])
		}
		end, err = strconv.Atoi(part[idx+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", part[idx+1:])
		}
	} else {
		// Single value
		var err error
		start, err = strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s", part)
		}
		end = start
	}

	// Validate range
	if start < min || start > max {
		return nil, fmt.Errorf("value %d out of range [%d-%d]", start, min, max)
	}
	if end < min || end > max {
		return nil, fmt.Errorf("value %d out of range [%d-%d]", end, min, max)
	}
	if start > end {
		return nil, fmt.Errorf("invalid range: %d > %d", start, end)
	}

	// Generate values
	var result []int
	for i := start; i <= end; i += step {
		result = append(result, i)
	}

	return result, nil
}

// Next returns the next time that matches the cron expression after the given time.
func (c *CronExpr) Next(from time.Time) time.Time {
	// Start from the next minute
	t := from.Truncate(time.Minute).Add(time.Minute)

	// Search for up to 4 years
	maxTime := from.Add(4 * 365 * 24 * time.Hour)

	for t.Before(maxTime) {
		// Check month
		if !contains(c.month, int(t.Month())) {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		// Check day of month and day of week
		dayOfMonthMatch := contains(c.dayOfMonth, t.Day())
		dayOfWeekMatch := contains(c.dayOfWeek, int(t.Weekday()))

		// Both day constraints must be satisfied if they're both restricted
		// (If one is *, only the other matters)
		isDayMatch := dayOfMonthMatch && dayOfWeekMatch

		if !isDayMatch {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		// Check hour
		if !contains(c.hour, t.Hour()) {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}

		// Check minute
		if !contains(c.minute, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}

		// Found a match
		return t
	}

	// No match found within 4 years
	return time.Time{}
}

// contains checks if a slice contains a value.
func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// unique removes duplicates from a slice.
func unique(slice []int) []int {
	seen := make(map[int]bool)
	var result []int
	for _, v := range slice {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
