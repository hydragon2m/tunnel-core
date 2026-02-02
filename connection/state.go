package connection

import "fmt"

// ConnectionState represents the lifecycle state of a connection
type ConnectionState int

const (
	// StateInit: Created but not yet handling traffic
	StateInit ConnectionState = iota
	// StateConnected: Connected, handshake pending
	StateConnected
	// StateAuthenticated: Handshake complete, active
	StateAuthenticated
	// StateClosing: Closing in progress
	StateClosing
	// StateClosed: Connection closed
	StateClosed
)

func (s ConnectionState) String() string {
	switch s {
	case StateInit:
		return "Init"
	case StateConnected:
		return "Connected"
	case StateAuthenticated:
		return "Authenticated"
	case StateClosing:
		return "Closing"
	case StateClosed:
		return "Closed"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// IsValidTransition checks if a transition from oldState to newState is allowed
func IsValidTransition(from, to ConnectionState) bool {
	switch from {
	case StateInit:
		return to == StateConnected || to == StateClosed
	case StateConnected:
		return to == StateAuthenticated || to == StateClosing || to == StateClosed
	case StateAuthenticated:
		return to == StateClosing || to == StateClosed
	case StateClosing:
		return to == StateClosed
	case StateClosed:
		return false // Terminal state
	default:
		return false
	}
}
