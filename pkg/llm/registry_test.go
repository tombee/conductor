package llm

import (
	"context"
	"testing"
)

// mockProvider is a simple mock for testing.
type mockProvider struct {
	name         string
	capabilities Capabilities
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Capabilities() Capabilities {
	return m.capabilities
}

func (m *mockProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	return nil, nil
}

func (m *mockProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	provider := &mockProvider{
		name: "test-provider",
		capabilities: Capabilities{
			Streaming: true,
			Tools:     true,
		},
	}

	// Test registration
	err := reg.Register(provider)
	if err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Test retrieval
	retrieved, err := reg.Get("test-provider")
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}

	if retrieved.Name() != "test-provider" {
		t.Errorf("expected provider name 'test-provider', got '%s'", retrieved.Name())
	}
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	reg := NewRegistry()

	provider := &mockProvider{name: "test-provider"}

	// First registration should succeed
	err := reg.Register(provider)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = reg.Register(provider)
	if err == nil {
		t.Error("expected error when registering duplicate provider, got nil")
	}
	// Error is wrapped with fmt.Errorf, so check with errors.Is or substring
	if err.Error() != "provider already registered: test-provider" {
		t.Errorf("expected 'provider already registered' error, got: %v", err)
	}
}

func TestRegistry_GetNonExistent(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Error("expected error when getting non-existent provider, got nil")
	}
	// Error is wrapped with fmt.Errorf
	if err.Error() != "provider not found: nonexistent" {
		t.Errorf("expected 'provider not found' error, got: %v", err)
	}
}

func TestRegistry_SetAndGetDefault(t *testing.T) {
	reg := NewRegistry()

	provider := &mockProvider{name: "default-provider"}
	err := reg.Register(provider)
	if err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	// Set as default
	err = reg.SetDefault("default-provider")
	if err != nil {
		t.Fatalf("failed to set default provider: %v", err)
	}

	// Get default
	defaultProvider, err := reg.GetDefault()
	if err != nil {
		t.Fatalf("failed to get default provider: %v", err)
	}

	if defaultProvider.Name() != "default-provider" {
		t.Errorf("expected default provider name 'default-provider', got '%s'", defaultProvider.Name())
	}
}

func TestRegistry_SetDefaultNonExistent(t *testing.T) {
	reg := NewRegistry()

	err := reg.SetDefault("nonexistent")
	if err == nil {
		t.Error("expected error when setting non-existent provider as default, got nil")
	}
}

func TestRegistry_GetDefaultWhenNotSet(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.GetDefault()
	if err == nil {
		t.Error("expected error when getting default provider before it's set, got nil")
	}
	if err != ErrNoDefaultProvider {
		t.Errorf("expected ErrNoDefaultProvider, got: %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()

	provider1 := &mockProvider{name: "provider-1"}
	provider2 := &mockProvider{name: "provider-2"}

	reg.Register(provider1)
	reg.Register(provider2)

	names := reg.List()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}

	// Check both providers are in the list
	found1, found2 := false, false
	for _, name := range names {
		if name == "provider-1" {
			found1 = true
		}
		if name == "provider-2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Error("not all providers found in list")
	}
}

func TestRegistry_SetFailoverOrder(t *testing.T) {
	reg := NewRegistry()

	provider1 := &mockProvider{name: "provider-1"}
	provider2 := &mockProvider{name: "provider-2"}

	reg.Register(provider1)
	reg.Register(provider2)

	// Set failover order
	err := reg.SetFailoverOrder([]string{"provider-1", "provider-2"})
	if err != nil {
		t.Fatalf("failed to set failover order: %v", err)
	}

	// Get failover order
	order := reg.GetFailoverOrder()
	if len(order) != 2 {
		t.Errorf("expected 2 providers in failover order, got %d", len(order))
	}

	if order[0] != "provider-1" || order[1] != "provider-2" {
		t.Errorf("unexpected failover order: %v", order)
	}
}

func TestRegistry_SetFailoverOrderNonExistent(t *testing.T) {
	reg := NewRegistry()

	err := reg.SetFailoverOrder([]string{"nonexistent"})
	if err == nil {
		t.Error("expected error when setting failover order with non-existent provider, got nil")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := NewRegistry()

	provider := &mockProvider{name: "test-provider"}
	reg.Register(provider)

	// Unregister
	err := reg.Unregister("test-provider")
	if err != nil {
		t.Fatalf("failed to unregister provider: %v", err)
	}

	// Verify it's gone
	_, err = reg.Get("test-provider")
	if err == nil {
		t.Error("expected provider to be unregistered")
	}
}

func TestRegistry_UnregisterDefault(t *testing.T) {
	reg := NewRegistry()

	provider := &mockProvider{name: "default-provider"}
	reg.Register(provider)
	reg.SetDefault("default-provider")

	// Should not be able to unregister default provider
	err := reg.Unregister("default-provider")
	if err == nil {
		t.Error("expected error when unregistering default provider, got nil")
	}
}

func TestRegistry_RegisterNil(t *testing.T) {
	reg := NewRegistry()

	err := reg.Register(nil)
	if err != ErrInvalidProvider {
		t.Errorf("expected ErrInvalidProvider when registering nil, got: %v", err)
	}
}

func TestGlobalRegistry(t *testing.T) {
	// Create a fresh provider for global registry test
	provider := &mockProvider{name: "global-test-provider"}

	// Clean up after test
	defer func() {
		Unregister("global-test-provider")
	}()

	// Test global Register
	err := Register(provider)
	if err != nil {
		t.Fatalf("failed to register provider in global registry: %v", err)
	}

	// Test global Get
	retrieved, err := Get("global-test-provider")
	if err != nil {
		t.Fatalf("failed to get provider from global registry: %v", err)
	}

	if retrieved.Name() != "global-test-provider" {
		t.Errorf("expected provider name 'global-test-provider', got '%s'", retrieved.Name())
	}

	// Test global List
	names := List()
	found := false
	for _, name := range names {
		if name == "global-test-provider" {
			found = true
			break
		}
	}
	if !found {
		t.Error("provider not found in global registry list")
	}
}
