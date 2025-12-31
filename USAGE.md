# Tunnel Core Usage Guide

## Quick Start

### 1. Build Server

```bash
cd tunnel-core
go build ./cmd/tunnel-server
```

### 2. Generate TLS Certificates (for agent connections)

```bash
# Create certs directory
mkdir -p certs

# Generate self-signed certificate for agent connections
openssl req -x509 -newkey rsa:4096 -keyout certs/agent-key.pem \
  -out certs/agent-cert.pem -days 365 -nodes \
  -subj "/CN=tunnel-server/O=Tunnel"
```

### 3. Run Server

```bash
./tunnel-server \
  -agent-addr=:8443 \
  -agent-tls=true \
  -agent-cert=./certs/agent-cert.pem \
  -agent-key=./certs/agent-key.pem \
  -public-addr=:8080 \
  -base-domain=localhost \
  -max-connections=1000 \
  -heartbeat-timeout=30s
```

## Command Line Flags

### Agent Listener

- `-agent-addr`: Address to listen for agent connections (default: `:8443`)
- `-agent-tls`: Enable TLS for agent connections (default: `true`)
- `-agent-cert`: TLS certificate file path (required if `-agent-tls=true`)
- `-agent-key`: TLS key file path (required if `-agent-tls=true`)

### Public Listener

- `-public-addr`: Address to listen for public HTTP requests (default: `:8080`)
- `-public-tls`: Enable TLS for public connections (default: `false`)
- `-public-cert`: TLS certificate file path (required if `-public-tls=true`)
- `-public-key`: TLS key file path (required if `-public-tls=true`)

### Configuration

- `-base-domain`: Base domain for tunnels (default: `localhost`)
- `-max-connections`: Maximum number of agent connections (default: `1000`)
- `-heartbeat-timeout`: Heartbeat timeout duration (default: `30s`)
- `-auth-timeout`: Authentication timeout duration (default: `10s`)

## Architecture Overview

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│   Public    │────────▶│ Tunnel Core  │◀────────│   Agent     │
│   Internet  │  HTTP   │   Server     │ Protocol│  (Client)   │
└─────────────┘         └──────────────┘   v1    └─────────────┘
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐      ┌──────▼──────┐
              │ Registry  │      │  Quota     │
              │           │      │  Limiter   │
              └──────────┘      └─────────────┘
```

## Workflow

### 1. Agent Connection

1. Agent connects to server via TCP/TLS
2. Agent sends `FrameAuth` with token
3. Server validates token and responds with `FrameAuth` (ACK)
4. Connection established, agent can send heartbeats

### 2. Tunnel Registration

1. Agent sends `FrameOpenStream` to register tunnel
2. Server creates tunnel mapping: `domain → connection`
3. Tunnel is now available for public requests

### 3. Public Request Flow

1. Public client sends HTTP request to `subdomain.base-domain`
2. Router looks up domain in Registry
3. Router checks rate limits (Quota/Limiter)
4. Router creates new stream on agent connection
5. Router forwards request to agent
6. Agent processes request and sends response
7. Router forwards response to public client
8. Stream closed

## Rate Limiting

### Setting Agent Limits

```go
limiter.SetAgentLimit(
    "agent-123",           // Agent ID
    100,                   // Max concurrent streams
    10485760,              // Max bandwidth (10 MB/s in bytes)
    100,                   // Rate limit (100 req/s)
)
```

### Setting Domain Limits

```go
limiter.SetDomainLimit(
    "example.com",         // Domain
    50,                    // Max concurrent streams
    50,                    // Rate limit (50 req/s)
)
```

### Checking Limits

Router automatically checks limits before processing requests:
- Returns HTTP 429 (Too Many Requests) if limit exceeded
- Acquires stream quota when request starts
- Releases stream quota when request completes

## Error Handling

### Common Errors

- **Connection Errors**: Connection closed, cleanup streams
- **Stream Errors**: Send `FrameError`, close stream
- **Protocol Errors**: Log, close connection
- **Rate Limit Errors**: HTTP 429 response

### Error Codes

See `internal/*/errors.go` files for error definitions:
- `connection.ErrMaxConnections`
- `connection.ErrStreamNotFound`
- `quota.ErrAgentRateLimitExceeded`
- `quota.ErrDomainStreamLimitExceeded`

## Monitoring

### Connection Status

Monitor active connections through Connection Manager:
- Total connections
- Connections per agent
- Active streams per connection
- Heartbeat status

### Rate Limiting Stats

Get token bucket statistics:
```go
tokens, capacity := tokenBucket.GetStats()
```

## Security Considerations

1. **TLS**: Always use TLS for agent connections
2. **Token Validation**: Implement proper token validation
3. **Rate Limiting**: Configure appropriate limits to prevent abuse
4. **Resource Limits**: Set max connections and streams
5. **Input Validation**: All frames are validated

## Performance Tuning

### Connection Pooling

- Reuse connections from agents
- Multiplex streams on single connection
- Monitor connection health with heartbeats

### Buffer Management

- Use `sync.Pool` for frame buffers (future optimization)
- Reuse buffers per connection
- Monitor GC pressure

### Concurrency

- Each connection = 1 goroutine for frame reading
- Each active stream = 2 goroutines (read/write)
- Public listener = 1 goroutine per request

## Troubleshooting

### Connection Timeout

- Check heartbeat timeout configuration
- Verify network connectivity
- Check TLS certificate validity

### Rate Limit Issues

- Review rate limit configuration
- Check token bucket capacity
- Monitor stream counts

### Stream Not Found

- Verify tunnel registration
- Check connection status
- Review stream lifecycle

## Next Steps

1. Implement configuration file loading
2. Add metrics collection (Prometheus)
3. Add structured logging
4. Implement admin API for management
5. Add health check endpoint

