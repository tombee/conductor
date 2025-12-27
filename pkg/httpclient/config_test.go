package httpclient

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", cfg.Timeout)
	}

	if cfg.RetryAttempts != 3 {
		t.Errorf("expected retry attempts 3, got %d", cfg.RetryAttempts)
	}

	if cfg.RetryBackoff != 100*time.Millisecond {
		t.Errorf("expected retry backoff 100ms, got %v", cfg.RetryBackoff)
	}

	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("expected max backoff 30s, got %v", cfg.MaxBackoff)
	}

	if cfg.UserAgent == "" {
		t.Error("expected non-empty user agent")
	}

	if cfg.AllowNonIdempotentRetry {
		t.Error("expected AllowNonIdempotentRetry to be false by default")
	}

	// Should be valid
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr bool
		errText   string
	}{
		{
			name: "valid config",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  100 * time.Millisecond,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: false,
		},
		{
			name: "zero timeout",
			cfg: Config{
				Timeout:       0,
				RetryAttempts: 3,
				RetryBackoff:  100 * time.Millisecond,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: true,
			errText:   "timeout must be > 0",
		},
		{
			name: "negative timeout",
			cfg: Config{
				Timeout:       -1 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  100 * time.Millisecond,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: true,
			errText:   "timeout must be > 0",
		},
		{
			name: "negative retry attempts",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: -1,
				RetryBackoff:  100 * time.Millisecond,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: true,
			errText:   "retry_attempts must be >= 0",
		},
		{
			name: "zero retry backoff with retries enabled",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  0,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: true,
			errText:   "retry_backoff must be > 0 when retry_attempts > 0",
		},
		{
			name: "max backoff less than retry backoff",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  5 * time.Second,
				MaxBackoff:    100 * time.Millisecond,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: true,
			errText:   "max_backoff",
		},
		{
			name: "empty user agent",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  100 * time.Millisecond,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "",
			},
			expectErr: true,
			errText:   "user_agent is required",
		},
		{
			name: "zero retries is valid",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 0,
				RetryBackoff:  0, // Doesn't matter when retries disabled
				MaxBackoff:    0,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: false,
		},
		{
			name: "max backoff equal to retry backoff",
			cfg: Config{
				Timeout:       10 * time.Second,
				RetryAttempts: 3,
				RetryBackoff:  5 * time.Second,
				MaxBackoff:    5 * time.Second,
				UserAgent:     "test-agent/1.0",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errText)
				} else if tt.errText != "" && !contains(err.Error(), tt.errText) {
					t.Errorf("expected error containing %q, got %q", tt.errText, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
