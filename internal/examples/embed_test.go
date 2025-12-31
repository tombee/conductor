package examples

import (
	"os"
	"path/filepath"
	"testing"
)

func TestList(t *testing.T) {
	examples, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(examples) == 0 {
		t.Fatal("List() returned no examples")
	}

	// Check that minimal example is present
	found := false
	for _, ex := range examples {
		if ex.Name == "minimal" {
			found = true
			if ex.Description == "" {
				t.Error("minimal example has no description")
			}
			break
		}
	}

	if !found {
		t.Error("minimal example not found in list")
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"minimal", false},
		{"nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := Get(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("Get() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Get() unexpected error: %v", err)
				}
				if len(content) == 0 {
					t.Error("Get() returned empty content")
				}
			}
		})
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{"minimal", true},
		{"nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Exists(tt.name)
			if result != tt.expect {
				t.Errorf("Exists(%q) = %v, want %v", tt.name, result, tt.expect)
			}
		})
	}
}

func TestCopyTo(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		destPath string
		wantErr  bool
	}{
		{
			name:     "minimal",
			destPath: filepath.Join(tmpDir, "test.yaml"),
			wantErr:  false,
		},
		{
			name:     "nonexistent",
			destPath: filepath.Join(tmpDir, "nonexistent.yaml"),
			wantErr:  true,
		},
		{
			name:     "minimal",
			destPath: filepath.Join(tmpDir, "subdir", "nested.yaml"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_to_"+filepath.Base(tt.destPath), func(t *testing.T) {
			err := CopyTo(tt.name, tt.destPath)
			if tt.wantErr {
				if err == nil {
					t.Error("CopyTo() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("CopyTo() unexpected error: %v", err)
				}

				// Verify file was created
				if _, err := os.Stat(tt.destPath); os.IsNotExist(err) {
					t.Errorf("CopyTo() did not create file at %s", tt.destPath)
				}

				// Verify content matches
				content, err := os.ReadFile(tt.destPath)
				if err != nil {
					t.Errorf("Failed to read copied file: %v", err)
				}

				original, err := Get(tt.name)
				if err != nil {
					t.Errorf("Failed to get original content: %v", err)
				}

				if string(content) != string(original) {
					t.Error("Copied content does not match original")
				}
			}
		})
	}
}
