package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Create temp config file
	configData := `
server:
  agent_addr: ":9443"
  public_addr: ":9080"
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
  timeouts:
    read: 60s
    write: 60s

limits:
  max_connections: 500
  max_connections_per_account: 5

logging:
  level: "debug"
  format: "text"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configData)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Load config
	config, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify values
	if config.Server.AgentAddr != ":9443" {
		t.Errorf("Expected agent_addr :9443, got %s", config.Server.AgentAddr)
	}
	if config.Server.PublicAddr != ":9080" {
		t.Errorf("Expected public_addr :9080, got %s", config.Server.PublicAddr)
	}
	if !config.Server.TLS.Enabled {
		t.Error("Expected TLS to be enabled")
	}
	if config.Server.Timeouts.Read != 60*time.Second {
		t.Errorf("Expected read timeout 60s, got %v", config.Server.Timeouts.Read)
	}
	if config.Limits.MaxConnections != 500 {
		t.Errorf("Expected max_connections 500, got %d", config.Limits.MaxConnections)
	}
	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", config.Logging.Level)
	}
}

func TestLoad_Defaults(t *testing.T) {
	configData := `
server:
  agent_addr: ":8443"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configData)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	config, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check defaults
	if config.Server.PublicAddr != ":8080" {
		t.Errorf("Expected default public_addr :8080, got %s", config.Server.PublicAddr)
	}
	if config.Limits.MaxConnections != 1000 {
		t.Errorf("Expected default max_connections 1000, got %d", config.Limits.MaxConnections)
	}
	if config.Logging.Level != "info" {
		t.Errorf("Expected default log level info, got %s", config.Logging.Level)
	}
}

func TestValidate_InvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		config      CoreConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "negative max_connections",
			config: CoreConfig{
				Server: ServerConfig{AgentAddr: ":8443", PublicAddr: ":8080"},
				Limits: LimitsConfig{MaxConnections: -1},
			},
			expectError: true,
			errorMsg:    "max_connections must be > 0",
		},
		{
			name: "per_account exceeds global",
			config: CoreConfig{
				Server: ServerConfig{AgentAddr: ":8443", PublicAddr: ":8080"},
				Limits: LimitsConfig{
					MaxConnections:           10,
					MaxConnectionsPerAccount: 20,
				},
			},
			expectError: true,
			errorMsg:    "max_connections_per_account cannot exceed",
		},
		{
			name: "invalid log level",
			config: CoreConfig{
				Server:  ServerConfig{AgentAddr: ":8443", PublicAddr: ":8080"},
				Limits:  LimitsConfig{MaxConnections: 100, MaxConnectionsPerAccount: 10},
				Logging: LoggingConfig{Level: "invalid", Format: "json"},
			},
			expectError: true,
			errorMsg:    "logging.level must be one of",
		},
		{
			name: "TLS enabled without cert",
			config: CoreConfig{
				Server: ServerConfig{
					AgentAddr:  ":8443",
					PublicAddr: ":8080",
					TLS:        TLSConfig{Enabled: true},
				},
				Limits: LimitsConfig{MaxConnections: 100, MaxConnectionsPerAccount: 10},
			},
			expectError: true,
			errorMsg:    "cert_file required when TLS is enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil {
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
