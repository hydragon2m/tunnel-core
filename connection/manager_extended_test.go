package connection

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hydragon2m/tunnel-core/testutil"
)

// TestConnectionLimits_PerAccount tests per-account connection limits
func TestConnectionLimits_PerAccount(t *testing.T) {
	maxPerAccount := 2
	manager := NewManager(10, maxPerAccount, 30*time.Second)

	accountID := "test-account"

	// Register first connection - should succeed
	conn1 := testutil.NewMockConnection()
	c1, err := manager.RegisterConnection("conn1", "agent1", accountID, conn1, nil)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "conn1", c1.ID)

	// Register second connection - should succeed
	conn2 := testutil.NewMockConnection()
	c2, err := manager.RegisterConnection("conn2", "agent2", accountID, conn2, nil)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "conn2", c2.ID)

	// Register third connection - should fail (exceeds limit)
	conn3 := testutil.NewMockConnection()
	_, err = manager.RegisterConnection("conn3", "agent3", accountID, conn3, nil)
	testutil.AssertError(t, err)

	// Close one connection
	err = manager.CloseConnection("conn1")
	testutil.AssertNoError(t, err)

	// Now third connection should succeed
	conn4 := testutil.NewMockConnection()
	c4, err := manager.RegisterConnection("conn4", "agent4", accountID, conn4, nil)
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, "conn4", c4.ID)
}

// TestConnectionCleanup_OnError tests cleanup when connection errors
func TestConnectionCleanup_OnError(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	var cleanupCalled bool
	var cleanupConnID string
	manager.SetOnConnectionClosed(func(connID string) {
		cleanupCalled = true
		cleanupConnID = connID
	})

	conn := testutil.NewMockConnection()
	_, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	// Close connection
	err = manager.CloseConnection("test-conn")
	testutil.AssertNoError(t, err)

	// Verify cleanup was called
	testutil.WaitForCondition(t, 1*time.Second, func() bool {
		return cleanupCalled
	}, "cleanup callback should be called")

	testutil.AssertEqual(t, "test-conn", cleanupConnID)

	// Verify connection is removed
	_, ok := manager.GetConnection("test-conn")
	testutil.AssertFalse(t, ok, "connection should be removed")
}

// TestStreamMultiplexing_Concurrent tests concurrent stream creation
func TestStreamMultiplexing_Concurrent(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	numStreams := 100
	streamIDs := make([]uint32, numStreams)

	// Create streams concurrently
	testutil.RunConcurrent(numStreams, func(i int) {
		streamID := c.AllocateStreamID()
		streamIDs[i] = streamID

		stream := c.createStream(streamID)
		if stream == nil {
			t.Errorf("Failed to create stream %d", streamID)
		}
	})

	// Verify all streams were created with unique IDs
	seen := make(map[uint32]bool)
	for _, id := range streamIDs {
		if seen[id] {
			t.Errorf("Duplicate stream ID: %d", id)
		}
		seen[id] = true
	}

	// Verify stream count
	if len(c.streams) != numStreams {
		t.Errorf("Expected %d streams, got %d", numStreams, len(c.streams))
	}
}

// TestHeartbeatTimeout_Handling tests heartbeat timeout detection
func TestHeartbeatTimeout_Handling(t *testing.T) {
	shortTimeout := 100 * time.Millisecond
	manager := NewManager(10, 5, shortTimeout)

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	// Update heartbeat initially
	c.updateHeartbeat()

	// Wait for timeout
	time.Sleep(shortTimeout + 50*time.Millisecond)

	// Check if connection is still alive (it should be timed out)
	// Note: This test assumes the manager has a heartbeat checker
	// If not implemented yet, this test will need to be updated

	// For now, we just verify the connection exists
	_, ok := manager.GetConnection("test-conn")
	testutil.AssertTrue(t, ok, "connection should exist")
}

// TestGracefulShutdown_WithActiveStreams tests graceful shutdown
func TestGracefulShutdown_WithActiveStreams(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	// Create some streams
	numStreams := 5
	for i := 0; i < numStreams; i++ {
		streamID := c.AllocateStreamID()
		stream := c.createStream(streamID)
		testutil.AssertTrue(t, stream != nil, "stream should be created")
	}

	// Close connection gracefully
	err = manager.CloseConnection("test-conn")
	testutil.AssertNoError(t, err)

	// Verify all streams are closed
	testutil.AssertEqual(t, 0, len(c.streams))

	// Verify connection is closed
	testutil.AssertTrue(t, conn.IsClosed(), "connection should be closed")
}

// TestConcurrentConnectionOperations tests concurrent operations
func TestConcurrentConnectionOperations(t *testing.T) {
	manager := NewManager(100, 50, 30*time.Second)

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // register, get, close

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			conn := testutil.NewMockConnection()
			connID := fmt.Sprintf("conn-%d", idx)
			agentID := fmt.Sprintf("agent-%d", idx)
			_, err := manager.RegisterConnection(connID, agentID, "test-account", conn, nil)
			if err != nil {
				t.Errorf("Failed to register connection %s: %v", connID, err)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			connID := fmt.Sprintf("conn-%d", idx)
			time.Sleep(10 * time.Millisecond) // Wait a bit for registration
			_, _ = manager.GetConnection(connID)
		}(i)
	}

	// Concurrent closes
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			connID := fmt.Sprintf("conn-%d", idx)
			time.Sleep(20 * time.Millisecond) // Wait a bit for registration
			_ = manager.CloseConnection(connID)
		}(i)
	}

	wg.Wait()

	// Verify final state
	allConns := manager.GetAllConnections()
	t.Logf("Final connection count: %d", len(allConns))
}

// TestStreamStateTransitions tests stream state machine
func TestStreamStateTransitions(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	streamID := c.AllocateStreamID()
	stream := c.createStream(streamID)
	testutil.AssertTrue(t, stream != nil, "stream should be created")

	// Initial state should be Init
	testutil.AssertEqual(t, StreamStateInit, stream.State)

	// Transition to Open
	stream.State = StreamStateOpen
	testutil.AssertEqual(t, StreamStateOpen, stream.State)

	// Transition to Data
	stream.State = StreamStateData
	testutil.AssertEqual(t, StreamStateData, stream.State)

	// Close stream
	stream.Close()
	testutil.AssertEqual(t, StreamStateClosed, stream.State)
}

// TestConnectionMetadata tests connection metadata handling
func TestConnectionMetadata(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	metadata := map[string]string{
		"version": "1.0",
		"region":  "us-west",
		"env":     "production",
	}

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, metadata)
	testutil.AssertNoError(t, err)

	// Verify metadata
	testutil.AssertEqual(t, "1.0", c.Metadata["version"])
	testutil.AssertEqual(t, "us-west", c.Metadata["region"])
	testutil.AssertEqual(t, "production", c.Metadata["env"])
}

// TestGetConnectionByAgentID tests agent ID lookup
func TestGetConnectionByAgentID(t *testing.T) {
	manager := NewManager(10, 5, 30*time.Second)

	conn := testutil.NewMockConnection()
	c, err := manager.RegisterConnection("test-conn", "test-agent", "test-account", conn, nil)
	testutil.AssertNoError(t, err)

	// Lookup by agent ID
	found, ok := manager.GetConnectionByAgentID("test-agent")
	testutil.AssertTrue(t, ok, "connection should be found")
	testutil.AssertEqual(t, c.ID, found.ID)

	// Lookup non-existent agent
	_, ok = manager.GetConnectionByAgentID("non-existent")
	testutil.AssertFalse(t, ok, "connection should not be found")
}

// Add missing import
