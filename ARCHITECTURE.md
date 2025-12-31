# Tunnel Core Architecture

## Tổng quan

Tunnel Core là server trung tâm nhận persistent connections từ agents và route incoming requests từ public internet đến đúng agent/stream.

## Kiến trúc tổng thể

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│   Public    │────────▶│ Tunnel Core  │◀────────│   Agent     │
│   Internet  │  HTTP   │   Server     │ Protocol│  (Client)   │
└─────────────┘         └──────────────┘   v1    └─────────────┘
                              │
                              │
                    ┌─────────┴─────────┐
                    │                   │
              ┌─────▼─────┐      ┌──────▼──────┐
              │ Registry  │      │  Control    │
              │           │      │   Plane     │
              └──────────┘      └─────────────┘
```

## Components

### 1. Connection Manager (`internal/connection/manager.go`)
**Trách nhiệm:**
- Quản lý persistent connections từ agents
- Mỗi agent = 1 persistent connection (multiplexed với StreamID)
- Track connection state, heartbeat, metadata

**Key Features:**
- Connection pool per agent
- Heartbeat monitoring
- Graceful shutdown
- Connection metadata (agent ID, capabilities, etc.)

### 2. Stream Manager (`internal/stream/manager.go`)
**Trách nhiệm:**
- Multiplexing: quản lý nhiều streams trên 1 connection
- Stream lifecycle: INIT → OPEN → DATA → CLOSED
- Stream state machine

**Key Features:**
- Stream registry per connection
- State transitions
- Cleanup khi stream close
- Stream metadata (request info, timestamps)

### 3. Handshake/Auth (`internal/handshake/auth.go`)
**Trách nhiệm:**
- Xác thực agent khi kết nối
- Exchange credentials, capabilities
- Establish secure session

**Flow:**
1. Agent connect → gửi FrameAuth
2. Server validate → gửi FrameAuth (ACK)
3. Nếu fail → close connection

### 4. Registry (`internal/registry/tunnel.go`)
**Trách nhiệm:**
- Mapping domain/subdomain → agent connection
- Route incoming requests đến đúng agent
- Domain registration từ agents

**Key Features:**
- Domain → Connection mapping
- Subdomain allocation
- Domain validation
- Concurrent access (read-heavy)

### 5. Public Listener (`internal/listener/http.go`)
**Trách nhiệm:**
- Lắng nghe HTTP/HTTPS requests từ public
- Parse Host header để route
- Forward request đến agent qua stream

**Key Features:**
- HTTP/1.1, HTTP/2 support
- TLS termination
- Request parsing
- Response forwarding

### 6. Router (`internal/router/router.go`)
**Trách nhiệm:**
- Route incoming HTTP request → agent connection + stream
- Tạo stream mới trên connection
- Forward request data

**Flow:**
1. Nhận HTTP request từ Public Listener
2. Parse Host → lookup Registry → tìm Connection
3. Tạo stream mới trên Connection
4. Forward request headers/body qua stream
5. Forward response từ stream → HTTP response

### 7. Quota/Limiter (`internal/quota/limiter.go`)
**Trách nhiệm:**
- Rate limiting per agent/domain
- Resource limits (concurrent streams, bandwidth)
- Abuse prevention

**Key Features:**
- Token bucket rate limiting
- Per-agent quotas
- Per-domain limits
- Metrics collection

## Data Flow

### 1. Agent Connection Flow
```
Agent → Connect TCP/TLS
      → Send FrameAuth (StreamID=0)
      → Server validates
      → Send FrameAuth (ACK, StreamID=0)
      → Connection established
      → Send FrameHeartbeat (periodic, StreamID=0)
```

### 2. Incoming Request Flow
```
Public → HTTP Request (Host: example.com)
      → Router: lookup Registry → find Connection
      → Router: create new StreamID
      → Router: send FrameOpenStream (StreamID=N)
      → Agent: receive FrameOpenStream
      → Agent: forward to local service
      → Agent: send FrameData (StreamID=N, request data)
      → Server: forward to Public (HTTP response)
      → Agent: send FrameData (StreamID=N, FlagEndStream)
      → Stream closed
```

### 3. Stream Lifecycle
```
INIT → OPEN → DATA* → CLOSED
  │      │      │        │
  │      │      │        └─ Cleanup
  │      │      └─ Multiple DATA frames
  │      └─ FrameOpenStream
  └─ New stream request
```

## Concurrency Model

- **Per-connection goroutine**: Mỗi agent connection có 1 goroutine đọc frames
- **Per-stream goroutine**: Mỗi active stream có 2 goroutines (read/write)
- **Public listener**: 1 goroutine accept, mỗi request = 1 goroutine
- **Registry**: Thread-safe với sync.RWMutex (read-heavy)

## Error Handling

- **Connection errors**: Close connection, cleanup streams
- **Stream errors**: Send FrameError, close stream
- **Protocol errors**: Log, close connection
- **Timeout**: SetReadDeadline, context cancellation

## Security Considerations

- **TLS**: Mandatory cho agent connections
- **Auth**: Token-based authentication
- **Rate limiting**: Prevent abuse
- **Input validation**: Validate all frames
- **Resource limits**: Max streams, bandwidth per agent

## Performance Optimizations

- **Connection pooling**: Reuse connections
- **Stream multiplexing**: Nhiều streams trên 1 connection
- **Buffer reuse**: sync.Pool cho frame buffers
- **Zero-copy**: Where possible
- **Metrics**: Prometheus metrics

## Dependencies

- `tunnel-protocol/go/v1`: Frame protocol
- Standard library: net, context, sync
- Optional: Prometheus, logging library

