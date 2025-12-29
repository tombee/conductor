package sdk

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "empty options",
			opts:    []Option{},
			wantErr: false,
		},
		{
			name: "with logger",
			opts: []Option{
				WithLogger(nil),
			},
			wantErr: true, // logger cannot be nil
		},
		{
			name: "with cost limit",
			opts: []Option{
				WithCostLimit(10.0),
			},
			wantErr: false,
		},
		{
			name: "negative cost limit",
			opts: []Option{
				WithCostLimit(-1.0),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk, err := New(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if sdk != nil {
				defer sdk.Close()
			}
		})
	}
}

func TestSDK_Close(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close should succeed
	if err := sdk.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Second close should succeed (idempotent)
	if err := sdk.Close(); err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestSDK_OnEvent(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sdk.Close()

	called := false
	sdk.OnEvent(EventWorkflowStarted, func(ctx context.Context, e *Event) {
		called = true
	})

	// Emit event
	sdk.emitEvent(context.Background(), &Event{
		Type: EventWorkflowStarted,
	})

	if !called {
		t.Error("event handler was not called")
	}
}

func TestSDK_OnEvent_PanicRecovery(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sdk.Close()

	// Register handler that panics
	sdk.OnEvent(EventWorkflowStarted, func(ctx context.Context, e *Event) {
		panic("test panic")
	})

	// Register handler that should still be called
	called := false
	sdk.OnEvent(EventWorkflowStarted, func(ctx context.Context, e *Event) {
		called = true
	})

	// Emit event - should not panic
	sdk.emitEvent(context.Background(), &Event{
		Type: EventWorkflowStarted,
	})

	if !called {
		t.Error("second handler was not called after first handler panicked")
	}
}

func TestWorkflowBuilder(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sdk.Close()

	// Build a simple workflow
	wf, err := sdk.NewWorkflow("test").
		Input("name", TypeString).
		Step("greet").LLM().
			Model("claude-sonnet-4-20250514").
			Prompt("Say hello to {{.inputs.name}}").
			Done().
		Build()

	if err != nil {
		t.Errorf("Build() error = %v", err)
	}

	if wf == nil {
		t.Error("Build() returned nil workflow")
	}
}

func TestWorkflowBuilder_Validation(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sdk.Close()

	// No steps - should fail
	_, err = sdk.NewWorkflow("test").Build()
	if err == nil {
		t.Error("Build() should fail with no steps")
	}

	// Duplicate step IDs - should fail
	_, err = sdk.NewWorkflow("test").
		Step("step1").LLM().Prompt("test").Done().
		Step("step1").LLM().Prompt("test").Done().
		Build()
	if err == nil {
		t.Error("Build() should fail with duplicate step IDs")
	}

	// Invalid dependency - should fail
	_, err = sdk.NewWorkflow("test").
		Step("step1").LLM().Prompt("test").DependsOn("nonexistent").Done().
		Build()
	if err == nil {
		t.Error("Build() should fail with invalid dependency")
	}
}

func TestValidateInputs(t *testing.T) {
	sdk, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sdk.Close()

	wf, err := sdk.NewWorkflow("test").
		Input("name", TypeString).
		Input("age", TypeNumber).
		InputWithDefault("greeting", TypeString, "Hello").
		Step("greet").LLM().Prompt("test").Done().
		Build()

	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	tests := []struct {
		name    string
		inputs  map[string]any
		wantErr bool
	}{
		{
			name: "all required inputs provided",
			inputs: map[string]any{
				"name": "Alice",
				"age":  30,
			},
			wantErr: false,
		},
		{
			name: "missing required input",
			inputs: map[string]any{
				"name": "Alice",
			},
			wantErr: true,
		},
		{
			name: "with optional input",
			inputs: map[string]any{
				"name":     "Alice",
				"age":      30,
				"greeting": "Hi",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sdk.ValidateInputs(context.Background(), wf, tt.inputs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
