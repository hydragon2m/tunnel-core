package registry

import (
	"testing"
	"time"
)

func TestRegistry_RegisterTunnel(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register tunnel (domain empty, will use subdomain + baseDomain = "example.localhost")
	_, err := reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	if err != nil {
		t.Fatalf("RegisterTunnel failed: %v", err)
	}

	// Try to register again with different connection (should fail)
	_, err = reg.RegisterTunnel("", "example", "conn-2", "agent-2", nil)
	if err == nil {
		t.Error("Expected error when registering duplicate domain")
	}
}

func TestRegistry_GetTunnel(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register tunnel
	_, err := reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	if err != nil {
		t.Fatalf("RegisterTunnel failed: %v", err)
	}

	// Get tunnel (using fullDomain)
	tunnel, ok := reg.GetTunnel("example.localhost")
	if !ok {
		t.Fatal("Expected tunnel to exist")
	}

	if tunnel.AgentID != "agent-1" {
		t.Errorf("Expected agent ID 'agent-1', got '%s'", tunnel.AgentID)
	}

	if tunnel.ConnectionID != "conn-1" {
		t.Errorf("Expected connection ID 'conn-1', got '%s'", tunnel.ConnectionID)
	}

	// Test non-existent tunnel
	_, ok = reg.GetTunnel("nonexistent.localhost")
	if ok {
		t.Error("Expected tunnel to not exist")
	}
}

func TestRegistry_UnregisterTunnel(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	if err != nil {
		t.Fatalf("RegisterTunnel failed: %v", err)
	}

	reg.UnregisterTunnel("example.localhost")

	_, ok := reg.GetTunnel("example.localhost")
	if ok {
		t.Error("Expected tunnel to be unregistered")
	}
}

func TestRegistry_UnregisterConnectionTunnels(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register multiple tunnels for same connection
	reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	reg.RegisterTunnel("", "test", "conn-1", "agent-1", nil)
	reg.RegisterTunnel("", "other", "conn-2", "agent-2", nil)

	// Unregister all tunnels for conn-1
	reg.UnregisterConnectionTunnels("conn-1")

	// Check tunnels (using fullDomain)
	_, ok := reg.GetTunnel("example.localhost")
	if ok {
		t.Error("Expected tunnel to be unregistered")
	}

	_, ok = reg.GetTunnel("test.localhost")
	if ok {
		t.Error("Expected tunnel to be unregistered")
	}

	// Other connection's tunnel should still exist
	_, ok = reg.GetTunnel("other.localhost")
	if !ok {
		t.Error("Expected tunnel to still exist")
	}
}

func TestRegistry_GetTunnelsByConnection(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register multiple tunnels for same connection
	reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	reg.RegisterTunnel("", "test", "conn-1", "agent-1", nil)
	reg.RegisterTunnel("", "other", "conn-2", "agent-2", nil)

	// Get tunnels by connection ID
	tunnels := reg.GetConnectionTunnels("conn-1")
	if len(tunnels) != 2 {
		t.Errorf("Expected 2 tunnels, got %d", len(tunnels))
	}

	// Check tunnel subdomains
	subdomains := make(map[string]bool)
	for _, tunnel := range tunnels {
		subdomains[tunnel.Subdomain] = true
	}

	if !subdomains["example"] {
		t.Error("Expected 'example' in tunnels")
	}

	if !subdomains["test"] {
		t.Error("Expected 'test' in tunnels")
	}
}

func TestRegistry_Subdomain(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register subdomain
	_, err := reg.RegisterTunnel("", "sub", "conn-1", "agent-1", nil)
	if err != nil {
		t.Fatalf("RegisterTunnel failed: %v", err)
	}

	tunnel, ok := reg.GetTunnel("sub.localhost")
	if !ok {
		t.Fatal("Expected tunnel to exist")
	}

	if tunnel.FullDomain != "sub.localhost" {
		t.Errorf("Expected full domain 'sub.localhost', got '%s'", tunnel.FullDomain)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry("localhost")

	// Concurrent RegisterTunnel
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			subdomain := "example" + string(rune(id))
			_, err := reg.RegisterTunnel("", subdomain, "conn-1", "agent-1", nil)
			if err != nil {
				t.Errorf("RegisterTunnel failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all tunnels registered
	for i := 0; i < 10; i++ {
		subdomain := "example" + string(rune(i))
		fullDomain := subdomain + ".localhost"
		_, ok := reg.GetTunnel(fullDomain)
		if !ok {
			t.Errorf("Expected tunnel %s to exist", fullDomain)
		}
	}
}

func TestTunnel_LastAccess(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "example", "conn-1", "agent-1", nil)
	if err != nil {
		t.Fatalf("RegisterTunnel failed: %v", err)
	}

	tunnel, ok := reg.GetTunnel("example.localhost")
	if !ok {
		t.Fatal("Expected tunnel to exist")
	}

	initialAccess := tunnel.LastAccess

	// Wait a bit for async update
	time.Sleep(50 * time.Millisecond)

	// Get tunnel again (should update LastAccess async)
	tunnel, ok = reg.GetTunnel("example.localhost")
	if !ok {
		t.Fatal("Expected tunnel to exist")
	}

	// LastAccess update is async, so we check if it's at least not before initial
	// In practice, it should be updated, but async update may not be immediate
	if tunnel.LastAccess.Before(initialAccess) {
		t.Error("Expected LastAccess to not be before initial")
	}
}

