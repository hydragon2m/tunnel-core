package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CoreConfig is the main configuration for tunnel-core server
type CoreConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Limits   LimitsConfig   `yaml:"limits"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	AgentAddr  string        `yaml:"agent_addr"`
	PublicAddr string        `yaml:"public_addr"`
	TLS        TLSConfig     `yaml:"tls"`
	Timeouts   TimeoutConfig `yaml:"timeouts"`
}

// TLSConfig contains TLS settings
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// TimeoutConfig contains timeout settings
type TimeoutConfig struct {
	Read      time.Duration `yaml:"read"`
	Write     time.Duration `yaml:"write"`
	Idle      time.Duration `yaml:"idle"`
	Heartbeat time.Duration `yaml:"heartbeat"`
}

// LimitsConfig contains resource limit settings
type LimitsConfig struct {
	MaxConnections           int `yaml:"max_connections"`
	MaxConnectionsPerAccount int `yaml:"max_connections_per_account"`
	MaxStreamsPerConnection  int `yaml:"max_streams_per_connection"`
	MaxRequestSize           int `yaml:"max_request_size"` // bytes
	RateLimit                struct {
		RequestsPerSecond int `yaml:"requests_per_second"`
		BurstSize         int `yaml:"burst_size"`
	} `yaml:"rate_limit"`
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	Auth struct {
		Type      string `yaml:"type"` // "jwt" or "token"
		JWTSecret string `yaml:"jwt_secret"`
		TokenFile string `yaml:"token_file"`
	} `yaml:"auth"`
	IPWhitelist []string `yaml:"ip_whitelist"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	Output string `yaml:"output"` // stdout, stderr, file path
}

// Load loads configuration from a YAML file
func Load(configPath string) (*CoreConfig, error) {
	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config CoreConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for unspecified config fields
func setDefaults(config *CoreConfig) {
	if config.Server.AgentAddr == "" {
		config.Server.AgentAddr = ":8443"
	}
	if config.Server.PublicAddr == "" {
		config.Server.PublicAddr = ":8080"
	}
	if config.Server.Timeouts.Read == 0 {
		config.Server.Timeouts.Read = 30 * time.Second
	}
	if config.Server.Timeouts.Write == 0 {
		config.Server.Timeouts.Write = 30 * time.Second
	}
	if config.Server.Timeouts.Idle == 0 {
		config.Server.Timeouts.Idle = 120 * time.Second
	}
	if config.Server.Timeouts.Heartbeat == 0 {
		config.Server.Timeouts.Heartbeat = 30 * time.Second
	}
	if config.Limits.MaxConnections == 0 {
		config.Limits.MaxConnections = 1000
	}
	if config.Limits.MaxConnectionsPerAccount == 0 {
		config.Limits.MaxConnectionsPerAccount = 10
	}
	if config.Limits.MaxStreamsPerConnection == 0 {
		config.Limits.MaxStreamsPerConnection = 100
	}
	if config.Limits.MaxRequestSize == 0 {
		config.Limits.MaxRequestSize = 100 * 1024 * 1024 // 100MB
	}
	if config.Limits.RateLimit.RequestsPerSecond == 0 {
		config.Limits.RateLimit.RequestsPerSecond = 100
	}
	if config.Limits.RateLimit.BurstSize == 0 {
		config.Limits.RateLimit.BurstSize = 200
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}
	if config.Logging.Output == "" {
		config.Logging.Output = "stdout"
	}
}

// Validate validates the configuration
func (c *CoreConfig) Validate() error {
	// Server validation
	if c.Server.AgentAddr == "" {
		return fmt.Errorf("server.agent_addr cannot be empty")
	}
	if c.Server.PublicAddr == "" {
		return fmt.Errorf("server.public_addr cannot be empty")
	}
	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("server.tls.cert_file required when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("server.tls.key_file required when TLS is enabled")
		}
	}

	// Limits validation
	if c.Limits.MaxConnections <= 0 {
		return fmt.Errorf("limits.max_connections must be > 0")
	}
	if c.Limits.MaxConnectionsPerAccount <= 0 {
		return fmt.Errorf("limits.max_connections_per_account must be > 0")
	}
	if c.Limits.MaxConnectionsPerAccount > c.Limits.MaxConnections {
		return fmt.Errorf("limits.max_connections_per_account cannot exceed limits.max_connections")
	}
	if c.Limits.MaxStreamsPerConnection <= 0 {
		return fmt.Errorf("limits.max_streams_per_connection must be > 0")
	}
	if c.Limits.MaxRequestSize <= 0 {
		return fmt.Errorf("limits.max_request_size must be > 0")
	}
	if c.Limits.RateLimit.RequestsPerSecond < 0 {
		return fmt.Errorf("limits.rate_limit.requests_per_second cannot be negative")
	}
	if c.Limits.RateLimit.BurstSize < 0 {
		return fmt.Errorf("limits.rate_limit.burst_size cannot be negative")
	}

	// Security validation
	if c.Security.Auth.Type != "" {
		if c.Security.Auth.Type != "jwt" && c.Security.Auth.Type != "token" {
			return fmt.Errorf("security.auth.type must be 'jwt' or 'token'")
		}
		if c.Security.Auth.Type == "jwt" && c.Security.Auth.JWTSecret == "" {
			return fmt.Errorf("security.auth.jwt_secret required when auth type is jwt")
		}
		if c.Security.Auth.Type == "token" && c.Security.Auth.TokenFile == "" {
			return fmt.Errorf("security.auth.token_file required when auth type is token")
		}
	}

	// Logging validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("logging.format must be one of: json, text")
	}

	return nil
}
