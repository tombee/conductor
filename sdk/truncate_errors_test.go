package sdk

import (
	"strings"
	"testing"
)

func TestTruncateError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *TruncateError
		wantCode string
		wantMsg  string
	}{
		{
			name: "input too large error",
			err: &TruncateError{
				Code:    ErrCodeInputTooLarge,
				Message: "input exceeds maximum size limit",
			},
			wantCode: "INPUT_TOO_LARGE",
			wantMsg:  "input exceeds maximum size limit",
		},
		{
			name: "invalid options error",
			err: &TruncateError{
				Code:    ErrCodeInvalidOptions,
				Message: "invalid truncation options",
			},
			wantCode: "INVALID_OPTIONS",
			wantMsg:  "invalid truncation options",
		},
		{
			name: "custom error",
			err: &TruncateError{
				Code:    "CUSTOM_ERROR",
				Message: "custom message",
			},
			wantCode: "CUSTOM_ERROR",
			wantMsg:  "custom message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if !strings.Contains(got, tt.wantCode) {
				t.Errorf("TruncateError.Error() = %v, want to contain code %v", got, tt.wantCode)
			}
			if !strings.Contains(got, tt.wantMsg) {
				t.Errorf("TruncateError.Error() = %v, want to contain message %v", got, tt.wantMsg)
			}
		})
	}
}

func TestNewInputTooLargeError(t *testing.T) {
	err := NewInputTooLargeError()

	if err.Code != ErrCodeInputTooLarge {
		t.Errorf("NewInputTooLargeError() code = %v, want %v", err.Code, ErrCodeInputTooLarge)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "input exceeds maximum size limit") {
		t.Errorf("NewInputTooLargeError() message = %v, want to contain 'input exceeds maximum size limit'", errMsg)
	}

	// Verify no sensitive information in error message
	if strings.Contains(errMsg, "bytes") || strings.Contains(errMsg, "MB") {
		t.Errorf("NewInputTooLargeError() message contains size information: %v", errMsg)
	}
}

func TestNewInvalidOptionsError(t *testing.T) {
	tests := []struct {
		name       string
		reason     string
		wantInMsg  string
		wantNotIn  []string
	}{
		{
			name:      "negative MaxLines",
			reason:    "MaxLines must be non-negative",
			wantInMsg: "MaxLines must be non-negative",
		},
		{
			name:      "negative MaxTokens",
			reason:    "MaxTokens must be non-negative",
			wantInMsg: "MaxTokens must be non-negative",
		},
		{
			name:      "negative MaxBytes",
			reason:    "MaxBytes must be non-negative",
			wantInMsg: "MaxBytes must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewInvalidOptionsError(tt.reason)

			if err.Code != ErrCodeInvalidOptions {
				t.Errorf("NewInvalidOptionsError() code = %v, want %v", err.Code, ErrCodeInvalidOptions)
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.wantInMsg) {
				t.Errorf("NewInvalidOptionsError() message = %v, want to contain %v", errMsg, tt.wantInMsg)
			}

			for _, notWant := range tt.wantNotIn {
				if strings.Contains(errMsg, notWant) {
					t.Errorf("NewInvalidOptionsError() message = %v, should not contain %v", errMsg, notWant)
				}
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Verify ErrInputTooLarge constant
	if ErrInputTooLarge.Code != ErrCodeInputTooLarge {
		t.Errorf("ErrInputTooLarge.Code = %v, want %v", ErrInputTooLarge.Code, ErrCodeInputTooLarge)
	}
	if ErrInputTooLarge.Message == "" {
		t.Error("ErrInputTooLarge.Message should not be empty")
	}

	// Verify ErrInvalidOptions constant
	if ErrInvalidOptions.Code != ErrCodeInvalidOptions {
		t.Errorf("ErrInvalidOptions.Code = %v, want %v", ErrInvalidOptions.Code, ErrCodeInvalidOptions)
	}
	if ErrInvalidOptions.Message == "" {
		t.Error("ErrInvalidOptions.Message should not be empty")
	}
}

func TestErrorCodesAreUnique(t *testing.T) {
	codes := map[string]bool{
		ErrCodeInputTooLarge:  true,
		ErrCodeInvalidOptions: true,
	}

	if len(codes) != 2 {
		t.Errorf("Error codes are not unique, got %d unique codes, want 2", len(codes))
	}
}

func TestErrorMessageSafety(t *testing.T) {
	// Verify that error messages don't leak sensitive information
	tests := []struct {
		name          string
		err           *TruncateError
		forbiddenStrs []string
	}{
		{
			name: "input too large - no size leak",
			err:  NewInputTooLargeError(),
			forbiddenStrs: []string{
				"10MB", "10485760", "bytes", "size:", "limit:",
			},
		},
		{
			name: "invalid options with reason",
			err:  NewInvalidOptionsError("MaxLines must be non-negative"),
			// Reason is allowed to be in the message, but not actual values
			forbiddenStrs: []string{
				"-1", "-100", "value:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, forbidden := range tt.forbiddenStrs {
				if strings.Contains(errMsg, forbidden) {
					t.Errorf("Error message contains forbidden string %q: %v", forbidden, errMsg)
				}
			}
		})
	}
}
