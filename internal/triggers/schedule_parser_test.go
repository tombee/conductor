package triggers

import (
	"strings"
	"testing"
)

func TestParseEverySchedule(t *testing.T) {
	tests := []struct {
		name        string
		every       string
		at          string
		timezone    string
		wantCron    string
		wantTZ      string
		wantErr     bool
		errContains string
	}{
		{
			name:     "hour",
			every:    "hour",
			at:       "",
			timezone: "",
			wantCron: "0 * * * *",
			wantTZ:   "UTC",
			wantErr:  false,
		},
		{
			name:     "day at 09:00",
			every:    "day",
			at:       "09:00",
			timezone: "America/New_York",
			wantCron: "0 9 * * *",
			wantTZ:   "America/New_York",
			wantErr:  false,
		},
		{
			name:     "week at 14:30",
			every:    "week",
			at:       "14:30",
			timezone: "",
			wantCron: "30 14 * * 1",
			wantTZ:   "UTC",
			wantErr:  false,
		},
		{
			name:     "month at midnight",
			every:    "month",
			at:       "00:00",
			timezone: "Europe/London",
			wantCron: "0 0 1 * *",
			wantTZ:   "Europe/London",
			wantErr:  false,
		},
		{
			name:        "hour with at not supported",
			every:       "hour",
			at:          "09:00",
			timezone:    "",
			wantErr:     true,
			errContains: "--at not supported with --every=hour",
		},
		{
			name:        "invalid every value",
			every:       "invalid",
			at:          "",
			timezone:    "",
			wantErr:     true,
			errContains: "invalid --every value",
		},
		{
			name:        "invalid time format",
			every:       "day",
			at:          "25:00",
			timezone:    "",
			wantErr:     true,
			errContains: "invalid time format",
		},
		{
			name:        "empty every",
			every:       "",
			at:          "",
			timezone:    "",
			wantErr:     true,
			errContains: "--every is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCron, gotTZ, err := ParseEverySchedule(tt.every, tt.at, tt.timezone)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEverySchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("ParseEverySchedule() error = %v, should contain %q", err, tt.errContains)
				}
				return
			}
			if !tt.wantErr {
				if gotCron != tt.wantCron {
					t.Errorf("ParseEverySchedule() cron = %v, want %v", gotCron, tt.wantCron)
				}
				if gotTZ != tt.wantTZ {
					t.Errorf("ParseEverySchedule() timezone = %v, want %v", gotTZ, tt.wantTZ)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
