package approval

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCLIApprover_ApproveYes(t *testing.T) {
	input := strings.NewReader("y\n")
	output := &bytes.Buffer{}
	approver := NewCLIApproverWithIO(input, output)

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "test-tool", "A test tool", map[string]interface{}{
		"param1": "value1",
	})

	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if !approved {
		t.Error("Expected approval for 'y' response")
	}

	// Check that prompt was displayed
	if !strings.Contains(output.String(), "test-tool") {
		t.Error("Expected tool name in prompt")
	}
}

func TestCLIApprover_ApproveNo(t *testing.T) {
	input := strings.NewReader("n\n")
	output := &bytes.Buffer{}
	approver := NewCLIApproverWithIO(input, output)

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "test-tool", "A test tool", nil)

	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if approved {
		t.Error("Expected denial for 'n' response")
	}
}

func TestCLIApprover_ApproveEmpty(t *testing.T) {
	input := strings.NewReader("\n")
	output := &bytes.Buffer{}
	approver := NewCLIApproverWithIO(input, output)

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "test-tool", "A test tool", nil)

	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if approved {
		t.Error("Expected denial for empty response (default is No)")
	}
}

func TestCLIApprover_ApproveAlways(t *testing.T) {
	// First call with "always"
	input := strings.NewReader("always\n")
	output := &bytes.Buffer{}
	approver := NewCLIApproverWithIO(input, output)

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "test-tool", "A test tool", nil)

	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if !approved {
		t.Error("Expected approval for 'always' response")
	}

	// Second call should auto-approve without prompting
	output.Reset()
	// No new input needed - should use cached "always" approval
	approved, err = approver.Approve(ctx, "test-tool", "A test tool", map[string]interface{}{
		"different": "params",
	})

	if err != nil {
		t.Fatalf("Second Approve() error = %v", err)
	}

	if !approved {
		t.Error("Expected auto-approval after 'always'")
	}

	// Should not have prompted again
	if strings.Contains(output.String(), "Approve execution") {
		t.Error("Expected no prompt on second call after 'always'")
	}
}

func TestCLIApprover_AlwaysOnlyForSpecificTool(t *testing.T) {
	input := strings.NewReader("always\nn\n")
	output := &bytes.Buffer{}
	approver := NewCLIApproverWithIO(input, output)

	ctx := context.Background()

	// Approve "tool1" with "always"
	approved, err := approver.Approve(ctx, "tool1", "First tool", nil)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if !approved {
		t.Error("Expected approval for 'always'")
	}

	// Deny "tool2" - should still prompt
	output.Reset()
	approved, err = approver.Approve(ctx, "tool2", "Second tool", nil)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if approved {
		t.Error("Expected denial for 'n'")
	}

	// Verify that tool2 was prompted
	if !strings.Contains(output.String(), "tool2") {
		t.Error("Expected tool2 to be prompted")
	}
}

func TestUnattendedApprover_AutoApprovedTool(t *testing.T) {
	approver := NewUnattendedApprover(map[string]bool{
		"safe-tool": true,
	})

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "safe-tool", "A safe tool", nil)

	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	if !approved {
		t.Error("Expected approval for auto-approved tool")
	}
}

func TestUnattendedApprover_NonAutoApprovedTool(t *testing.T) {
	approver := NewUnattendedApprover(map[string]bool{
		"safe-tool": true,
	})

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "dangerous-tool", "A dangerous tool", nil)

	if err == nil {
		t.Fatal("Expected error for non-auto-approved tool in unattended mode")
	}

	if approved {
		t.Error("Expected denial for non-auto-approved tool")
	}

	if !strings.Contains(err.Error(), "unattended mode") {
		t.Errorf("Expected error message about unattended mode, got: %v", err)
	}
}

func TestUnattendedApprover_EmptyAutoApprovedList(t *testing.T) {
	approver := NewUnattendedApprover(map[string]bool{})

	ctx := context.Background()
	approved, err := approver.Approve(ctx, "any-tool", "Any tool", nil)

	if err == nil {
		t.Fatal("Expected error when no tools are auto-approved")
	}

	if approved {
		t.Error("Expected denial")
	}
}
