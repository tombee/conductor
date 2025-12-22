package shell

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test with nil config
	sc, err := New(nil)
	if err != nil {
		t.Fatalf("New with nil config failed: %v", err)
	}
	if sc.config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout of 30s, got %v", sc.config.Timeout)
	}

	// Test with custom config
	sc, err = New(&Config{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("New with custom config failed: %v", err)
	}
	if sc.config.Timeout != 10*time.Second {
		t.Errorf("Expected timeout of 10s, got %v", sc.config.Timeout)
	}
}

func TestExecute_Run_StringCommand(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	result, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map response, got %T", result.Response)
	}

	stdout, ok := response["stdout"].(string)
	if !ok {
		t.Fatalf("Expected stdout string, got %T", response["stdout"])
	}
	if stdout != "hello" {
		t.Errorf("Expected 'hello', got %q", stdout)
	}
}

func TestExecute_Run_ArrayCommand(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	result, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": []string{"echo", "hello world"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	response, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map response, got %T", result.Response)
	}

	stdout := response["stdout"].(string)
	if stdout != "hello world" {
		t.Errorf("Expected 'hello world', got %q", stdout)
	}
}

func TestExecute_Run_InterfaceArrayCommand(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	// Test with []interface{} (as would come from YAML parsing)
	result, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": []interface{}{"echo", "test"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	response := result.Response.(map[string]interface{})
	stdout := response["stdout"].(string)
	if stdout != "test" {
		t.Errorf("Expected 'test', got %q", stdout)
	}
}

func TestExecute_Run_MissingCommand(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	_, err := sc.Execute(ctx, "run", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing command")
	}
}

func TestExecute_Run_EmptyArrayCommand(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	_, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": []string{},
	})
	if err == nil {
		t.Error("Expected error for empty command array")
	}
}

func TestExecute_Run_InvalidCommandType(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	_, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": 123, // Invalid type
	})
	if err == nil {
		t.Error("Expected error for invalid command type")
	}
}

func TestExecute_Run_ExitCode(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	// Test successful command (exit code 0)
	result, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": []string{"true"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	response := result.Response.(map[string]interface{})
	exitCode := response["exit_code"].(int)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Test failed command (exit code 1)
	_, err = sc.Execute(ctx, "run", map[string]interface{}{
		"command": []string{"false"},
	})
	if err == nil {
		t.Error("Expected error for failed command")
	}
}

func TestExecute_UnknownOperation(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	_, err := sc.Execute(ctx, "unknown", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unknown operation")
	}
}

func TestExecute_Run_WithEnv(t *testing.T) {
	sc, _ := New(nil)
	ctx := context.Background()

	result, err := sc.Execute(ctx, "run", map[string]interface{}{
		"command": "echo $TEST_VAR",
		"env": map[string]interface{}{
			"TEST_VAR": "test_value",
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	response := result.Response.(map[string]interface{})
	stdout := response["stdout"].(string)
	if stdout != "test_value" {
		t.Errorf("Expected 'test_value', got %q", stdout)
	}
}
