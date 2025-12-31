# Tunnel Core API Documentation

## Connection Manager API

### NewManager

```go
func NewManager(maxConnections int, heartbeatTimeout time.Duration) *Manager
```

Creates a new Connection Manager.

**Parameters:**
- `maxConnections`: Maximum number of agent connections
- `heartbeatTimeout`: Timeout for heartbeat detection

**Returns:** `*Manager`

### RegisterConnection

```go
func (m *Manager) RegisterConnection(connID, agentID string, conn Conn, metadata map[string]string) (*Connection, error)
```

Registers a new agent connection.

**Parameters:**
- `connID`: Unique connection ID
- `agentID`: Agent identifier
- `conn`: Network connection implementing `Conn` interface
- `metadata`: Connection metadata

**Returns:** `*Connection`, `error`

### GetConnection

```go
func (m *Manager) GetConnection(connID string) (*Connection, bool)
```

Gets connection by ID.

**Returns:** `*Connection`, `bool` (exists)

### GetConnectionByAgentID

```go
func (m *Manager) GetConnectionByAgentID(agentID string) (*Connection, bool)
```

Gets connection by agent ID.

**Returns:** `*Connection`, `bool` (exists)

### SetOnConnectionClosed

```go
func (m *Manager) SetOnConnectionClosed(callback func(connID string))
```

Sets callback when connection closes.

### SetOnStreamCreated

```go
func (m *Manager) SetOnStreamCreated(callback func(connID string, streamID uint32))
```

Sets callback when stream is created.

### SetOnStreamClosed

```go
func (m *Manager) SetOnStreamClosed(callback func(connID string, streamID uint32))
```

Sets callback when stream closes.

## Connection API

### SendFrame

```go
func (c *Connection) SendFrame(frame *v1.Frame) error
```

Sends frame to agent.

**Returns:** `error`

### GetStream

```go
func (c *Connection) GetStream(streamID uint32) (*Stream, bool)
```

Gets stream by ID.

**Returns:** `*Stream`, `bool` (exists)

### AllocateStreamID

```go
func (c *Connection) AllocateStreamID() uint32
```

Allocates new stream ID.

**Returns:** `uint32`

### Context

```go
func (c *Connection) Context() context.Context
```

Returns connection context for cancellation.

**Returns:** `context.Context`

### Close

```go
func (c *Connection) Close() error
```

Closes connection and cleanup.

**Returns:** `error`

## Stream API

### GetState

```go
func (s *Stream) GetState() StreamState
```

Gets stream state.

**Returns:** `StreamState`

### DataIn

```go
func (s *Stream) DataIn() <-chan []byte
```

Returns data input channel.

**Returns:** `<-chan []byte`

### CloseCh

```go
func (s *Stream) CloseCh() <-chan struct{}
```

Returns close channel.

**Returns:** `<-chan struct{}`

## Registry API

### NewRegistry

```go
func NewRegistry(baseDomain string) *Registry
```

Creates new Registry.

**Parameters:**
- `baseDomain`: Base domain for tunnels

**Returns:** `*Registry`

### RegisterTunnel

```go
func (r *Registry) RegisterTunnel(domain, subdomain, connectionID, agentID string, metadata map[string]string) (*Tunnel, error)
```

Registers new tunnel.

**Parameters:**
- `domain`: Full domain name
- `subdomain`: Subdomain part
- `connectionID`: Connection ID
- `agentID`: Agent ID
- `metadata`: Tunnel metadata

**Returns:** `*Tunnel`, `error`

### GetTunnel

```go
func (r *Registry) GetTunnel(domain string) (*Tunnel, bool)
```

Gets tunnel by domain.

**Returns:** `*Tunnel`, `bool` (exists)

### UnregisterTunnel

```go
func (r *Registry) UnregisterTunnel(domain string) error
```

Unregisters tunnel.

**Returns:** `error`

### UnregisterConnectionTunnels

```go
func (r *Registry) UnregisterConnectionTunnels(connectionID string)
```

Unregisters all tunnels for connection.

## Handshake/Auth API

### NewAuthenticator

```go
func NewAuthenticator(validateToken func(token string) (agentID string, err error), authTimeout time.Duration) *Authenticator
```

Creates new Authenticator.

**Parameters:**
- `validateToken`: Token validation function
- `authTimeout`: Authentication timeout

**Returns:** `*Authenticator`

### HandleAuth

```go
func (a *Authenticator) HandleAuth(frame *v1.Frame) (agentID string, metadata map[string]string, err error)
```

Handles authentication frame from agent.

**Returns:** `agentID`, `metadata`, `error`

### CreateAuthSuccessResponse

```go
func (a *Authenticator) CreateAuthSuccessResponse(agentID string, config map[string]interface{}) (*v1.Frame, error)
```

Creates success auth response.

**Returns:** `*v1.Frame`, `error`

### CreateAuthErrorResponse

```go
func (a *Authenticator) CreateAuthErrorResponse(errMsg string) (*v1.Frame, error)
```

Creates error auth response.

**Returns:** `*v1.Frame`, `error`

## Router API

### NewRouter

```go
func NewRouter(reg *registry.Registry, connManager *connection.Manager, limiter *quota.Limiter, timeout time.Duration) *Router
```

Creates new Router.

**Parameters:**
- `reg`: Registry instance
- `connManager`: Connection Manager instance
- `limiter`: Quota Limiter instance
- `timeout`: Request timeout

**Returns:** `*Router`

### ServeHTTP

```go
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request)
```

Implements `http.Handler` interface.

## Quota/Limiter API

### NewLimiter

```go
func NewLimiter(maxConnections, maxStreams int) *Limiter
```

Creates new Limiter.

**Parameters:**
- `maxConnections`: Maximum connections globally
- `maxStreams`: Maximum streams globally

**Returns:** `*Limiter`

### SetAgentLimit

```go
func (l *Limiter) SetAgentLimit(agentID string, maxStreams int, maxBandwidth int64, rateLimit int)
```

Sets limits for agent.

**Parameters:**
- `agentID`: Agent identifier
- `maxStreams`: Maximum concurrent streams
- `maxBandwidth`: Maximum bandwidth (bytes/second)
- `rateLimit`: Rate limit (requests/second)

### SetDomainLimit

```go
func (l *Limiter) SetDomainLimit(domain string, maxStreams int, rateLimit int)
```

Sets limits for domain.

**Parameters:**
- `domain`: Domain name
- `maxStreams`: Maximum concurrent streams
- `rateLimit`: Rate limit (requests/second)

### CheckRequest

```go
func (l *Limiter) CheckRequest(agentID, domain string) error
```

Checks all limits for request.

**Returns:** `error` (if limit exceeded)

### AcquireStream

```go
func (l *Limiter) AcquireStream(agentID, domain string) error
```

Acquires stream quota.

**Returns:** `error` (if limit exceeded)

### ReleaseStream

```go
func (l *Limiter) ReleaseStream(agentID, domain string)
```

Releases stream quota.

### GetAgentLimit

```go
func (l *Limiter) GetAgentLimit(agentID string) (*AgentLimit, bool)
```

Gets agent limit configuration.

**Returns:** `*AgentLimit`, `bool` (exists)

### GetDomainLimit

```go
func (l *Limiter) GetDomainLimit(domain string) (*DomainLimit, bool)
```

Gets domain limit configuration.

**Returns:** `*DomainLimit`, `bool` (exists)

## Token Bucket API

### NewTokenBucket

```go
func NewTokenBucket(capacity, refillRate int) *TokenBucket
```

Creates new token bucket.

**Parameters:**
- `capacity`: Maximum tokens
- `refillRate`: Tokens per second

**Returns:** `*TokenBucket`

### Allow

```go
func (tb *TokenBucket) Allow() bool
```

Checks if token available and consumes if yes.

**Returns:** `bool` (allowed)

### AllowN

```go
func (tb *TokenBucket) AllowN(n int) bool
```

Checks if N tokens available and consumes if yes.

**Returns:** `bool` (allowed)

### GetStats

```go
func (tb *TokenBucket) GetStats() (tokens float64, capacity int)
```

Gets token bucket statistics.

**Returns:** `tokens`, `capacity`

