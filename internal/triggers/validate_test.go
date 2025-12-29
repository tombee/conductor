package triggers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkflowExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test workflow file
	workflowFile := "test-workflow.yaml"
	workflowPath := filepath.Join(tmpDir, workflowFile)
	if err := os.WriteFile(workflowPath, []byte("name: test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		workflowsDir string
		workflow     string
		wantErr      bool
	}{
		{
			name:         "valid workflow",
			workflowsDir: tmpDir,
			workflow:     workflowFile,
			wantErr:      false,
		},
		{
			name:         "missing workflow",
			workflowsDir: tmpDir,
			workflow:     "nonexistent.yaml",
			wantErr:      true,
		},
		{
			name:         "empty workflows dir",
			workflowsDir: "",
			workflow:     workflowFile,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkflowExists(tt.workflowsDir, tt.workflow)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkflowExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCron(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{
			name:    "valid cron",
			expr:    "0 9 * * *",
			wantErr: false,
		},
		{
			name:    "valid cron with multiple values",
			expr:    "0 9,21 * * *",
			wantErr: false,
		},
		{
			name:    "special @hourly",
			expr:    "@hourly",
			wantErr: false,
		},
		{
			name:    "empty cron",
			expr:    "",
			wantErr: true,
		},
		{
			name:    "invalid cron",
			expr:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCron(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCron() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSONPath(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{
			name:    "simple path",
			expr:    "$.repository.name",
			wantErr: false,
		},
		{
			name:    "array access",
			expr:    "$.items[0].name",
			wantErr: false,
		},
		{
			name:    "wildcard",
			expr:    "$.items[*].name",
			wantErr: false,
		},
		{
			name:    "empty is valid",
			expr:    "",
			wantErr: false,
		},
		{
			name:    "missing $",
			expr:    "repository.name",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONPath(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSONPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSecretRef(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{
			name:    "valid secret",
			secret:  "${GITHUB_SECRET}",
			wantErr: false,
		},
		{
			name:    "valid with underscore",
			secret:  "${MY_WEBHOOK_SECRET}",
			wantErr: false,
		},
		{
			name:    "empty is valid",
			secret:  "",
			wantErr: false,
		},
		{
			name:    "missing braces",
			secret:  "$GITHUB_SECRET",
			wantErr: true,
		},
		{
			name:    "lowercase",
			secret:  "${github_secret}",
			wantErr: true,
		},
		{
			name:    "no dollar sign",
			secret:  "{GITHUB_SECRET}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretRef(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecretRef() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTimezone(t *testing.T) {
	tests := []struct {
		name    string
		tz      string
		wantErr bool
	}{
		{
			name:    "valid timezone",
			tz:      "America/New_York",
			wantErr: false,
		},
		{
			name:    "UTC",
			tz:      "UTC",
			wantErr: false,
		},
		{
			name:    "empty is valid",
			tz:      "",
			wantErr: false,
		},
		{
			name:    "invalid timezone",
			tz:      "Invalid/Timezone",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimezone(tt.tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTimezone() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
