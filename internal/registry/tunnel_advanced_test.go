package registry

import (
	"fmt"
	"testing"

	"github.com/hydragon2m/tunnel-core/testutil"
)

// TestRegistry_RegisterTunnel_Advanced tests tunnel registration
func TestRegistry_RegisterTunnel_Advanced(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	// Verify tunnel is registered
	tunnel, ok := reg.GetTunnel("app1.localhost")
	testutil.AssertTrue(t, ok, "tunnel should be found")
	testutil.AssertEqual(t, "agent1", tunnel.AgentID)
	testutil.AssertEqual(t, "conn1", tunnel.ConnectionID)
	testutil.AssertEqual(t, "account1", tunnel.AgentID)
}

// TestRegistry_DuplicateTunnel tests duplicate tunnel registration
func TestRegistry_DuplicateTunnel(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	// Try to register same subdomain
	_, err = reg.RegisterTunnel("", "app1", "conn2", "agent2", nil)
	testutil.AssertError(t, err)
}

// TestRegistry_UnregisterTunnel_Advanced tests tunnel unregistration
func TestRegistry_UnregisterTunnel_Advanced(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	err = reg.UnregisterTunnel("app1.localhost")
	testutil.AssertNoError(t, err)

	// Verify tunnel is removed
	_, ok := reg.GetTunnel("app1.localhost")
	testutil.AssertFalse(t, ok, "tunnel should not be found")
}

// TestRegistry_GetTunnelsByConnection_Advanced tests connection-based lookup
func TestRegistry_GetTunnelsByConnection_Advanced(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register multiple tunnels for same connection
	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	_, err = reg.RegisterTunnel("", "app2", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	_, err = reg.RegisterTunnel("", "app3", "conn2", "agent2", nil)
	testutil.AssertNoError(t, err)

	// Get tunnels for conn1
	tunnels := reg.GetConnectionTunnels("conn1")
	testutil.AssertEqual(t, 2, len(tunnels))

	// Verify tunnel domains
	domains := make(map[string]bool)
	for _, tunnel := range tunnels {
		domains[tunnel.Domain] = true
	}

	testutil.AssertTrue(t, domains["app1.localhost"], "app1 should be found")
	testutil.AssertTrue(t, domains["app2.localhost"], "app2 should be found")
}

// TestRegistry_GetAllTunnels tests getting all tunnels
func TestRegistry_GetAllTunnels(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	_, err = reg.RegisterTunnel("", "app2", "conn2", "agent2", nil)
	testutil.AssertNoError(t, err)

	_, err = reg.RegisterTunnel("", "app3", "conn3", "agent3", nil)
	testutil.AssertNoError(t, err)

	tunnels := reg.ListTunnels()
	testutil.AssertEqual(t, 3, len(tunnels))
}

// TestRegistry_ConcurrentAccess_Advanced tests concurrent operations
func TestRegistry_ConcurrentAccess_Advanced(t *testing.T) {
	reg := NewRegistry("localhost")

	numGoroutines := 50

	// Concurrent registrations
	testutil.RunConcurrent(numGoroutines, func(i int) {
		subdomain := fmt.Sprintf("app%d", i)
		agentID := fmt.Sprintf("agent%d", i)
		connID := fmt.Sprintf("conn%d", i)
		accountID := fmt.Sprintf("account%d", i%10) // 10 accounts

		_, err := reg.RegisterTunnel(agentID, subdomain, connID, accountID, nil)
		if err != nil {
			t.Errorf("Failed to register tunnel %s: %v", subdomain, err)
		}
	})

	// Concurrent reads
	testutil.RunConcurrent(numGoroutines, func(i int) {
		domain := fmt.Sprintf("app%d.localhost", i)
		_, _ = reg.GetTunnel(domain)
	})

	// Verify all tunnels were registered
	tunnels := reg.ListTunnels()
	if len(tunnels) != numGoroutines {
		t.Errorf("Expected %d tunnels, got %d", numGoroutines, len(tunnels))
	}
}

// TestRegistry_SubdomainValidation tests subdomain validation
func TestRegistry_SubdomainValidation(t *testing.T) {
	reg := NewRegistry("localhost")

	testCases := []struct {
		subdomain  string
		shouldFail bool
	}{
		{"valid", false},
		{"valid-subdomain", false},
		{"valid123", false},
		{"", true},                  // Empty
		{"invalid_subdomain", true}, // Underscore
		{"invalid.subdomain", true}, // Dot
		{"invalid subdomain", true}, // Space
		{"UPPERCASE", false},        // Should be converted to lowercase
	}

	for _, tc := range testCases {
		_, err := reg.RegisterTunnel("agent", tc.subdomain, "conn", "account", nil)
		if tc.shouldFail {
			testutil.AssertError(t, err)
		} else {
			testutil.AssertNoError(t, err)
			// Cleanup
			domain := fmt.Sprintf("%s.localhost", tc.subdomain)
			reg.UnregisterTunnel(domain)
		}
	}
}

// TestRegistry_DomainLookup tests various domain lookup scenarios
func TestRegistry_DomainLookup(t *testing.T) {
	reg := NewRegistry("localhost")

	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	testCases := []struct {
		domain string
		found  bool
	}{
		{"app1.localhost", true},
		{"APP1.localhost", true},  // Case insensitive
		{"app1.LOCALHOST", true},  // Case insensitive
		{"app2.localhost", false}, // Not registered
		{"localhost", false},      // Base domain only
		{"", false},               // Empty
	}

	for _, tc := range testCases {
		_, ok := reg.GetTunnel(tc.domain)
		if ok != tc.found {
			t.Errorf("Domain %s: expected found=%v, got %v", tc.domain, tc.found, ok)
		}
	}
}

// TestRegistry_UnregisterNonExistent tests unregistering non-existent tunnel
func TestRegistry_UnregisterNonExistent(t *testing.T) {
	reg := NewRegistry("localhost")

	err := reg.UnregisterTunnel("non-existent.localhost")
	testutil.AssertError(t, err)
}

// TestRegistry_ConnectionCleanup tests cleaning up tunnels by connection
func TestRegistry_ConnectionCleanup(t *testing.T) {
	reg := NewRegistry("localhost")

	// Register multiple tunnels for same connection
	_, err := reg.RegisterTunnel("", "app1", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	_, err = reg.RegisterTunnel("", "app2", "conn1", "agent1", nil)
	testutil.AssertNoError(t, err)

	// Unregister all tunnels for connection
	reg.UnregisterConnectionTunnels("conn1")

	// Verify all tunnels are removed
	_, ok := reg.GetTunnel("app1.localhost")
	testutil.AssertFalse(t, ok, "app1 should be removed")

	_, ok = reg.GetTunnel("app2.localhost")
	testutil.AssertFalse(t, ok, "app2 should be removed")
}

// Add missing import
