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
	"testing"
	"time"
)

func TestParseCron(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every minute", "* * * * *", false},
		{"every hour", "0 * * * *", false},
		{"every day at midnight", "0 0 * * *", false},
		{"every weekday at 9am", "0 9 * * 1-5", false},
		{"every 15 minutes", "*/15 * * * *", false},
		{"specific minutes", "0,15,30,45 * * * *", false},
		{"@hourly", "@hourly", false},
		{"@daily", "@daily", false},
		{"@weekly", "@weekly", false},
		{"@monthly", "@monthly", false},
		{"@yearly", "@yearly", false},
		{"invalid - too few fields", "* * *", true},
		{"invalid - too many fields", "* * * * * *", true},
		{"invalid - bad minute", "60 * * * *", true},
		{"invalid - bad hour", "0 25 * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCron(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCron(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestCronExpr_Next(t *testing.T) {
	// Fixed reference time: 2025-01-15 10:30:00 UTC (Wednesday)
	ref := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		from     time.Time
		expected time.Time
	}{
		{
			name:     "every minute - next minute",
			expr:     "* * * * *",
			from:     ref,
			expected: time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC),
		},
		{
			name:     "every hour at 0 - next hour",
			expr:     "0 * * * *",
			from:     ref,
			expected: time.Date(2025, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name:     "midnight - next midnight",
			expr:     "0 0 * * *",
			from:     ref,
			expected: time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "every 15 minutes - next 15 minute mark",
			expr:     "*/15 * * * *",
			from:     ref,
			expected: time.Date(2025, 1, 15, 10, 45, 0, 0, time.UTC),
		},
		{
			name:     "weekdays at 9am - next weekday (today is Wednesday)",
			expr:     "0 9 * * 1-5",
			from:     ref,
			expected: time.Date(2025, 1, 16, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseCron(tt.expr)
			if err != nil {
				t.Fatalf("ParseCron failed: %v", err)
			}

			got := expr.Next(tt.from)
			if !got.Equal(tt.expected) {
				t.Errorf("Next() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseField(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		min, max int
		expected []int
		wantErr  bool
	}{
		{"wildcard", "*", 0, 5, []int{0, 1, 2, 3, 4, 5}, false},
		{"single value", "3", 0, 5, []int{3}, false},
		{"range", "1-3", 0, 5, []int{1, 2, 3}, false},
		{"step", "*/2", 0, 5, []int{0, 2, 4}, false},
		{"comma list", "1,3,5", 0, 5, []int{1, 3, 5}, false},
		{"range with step", "0-4/2", 0, 5, []int{0, 2, 4}, false},
		{"out of range", "10", 0, 5, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseField(tt.field, tt.min, tt.max)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !sliceEqual(got, tt.expected) {
				t.Errorf("parseField() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
