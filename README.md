# Tunnel Core

Core server cho reverse tunnel system - nhận persistent connections từ agents và route incoming requests từ public internet đến đúng agent/stream.

## Quick Start

### 1. Setup Environment

**Windows (PowerShell):**
```powershell
$env:GOPRIVATE="github.com/hydragon2m/*"
```

**Linux/Mac:**
```bash
export GOPRIVATE=github.com/hydragon2m/*
```

### 2. Build

```bash
go build ./cmd/tunnel-server
```

### 3. Generate TLS Certificates

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/agent-key.pem \
  -out certs/agent-cert.pem -days 365 -nodes \
  -subj "/CN=tunnel-server/O=Tunnel"
```

### 4. Run Server

```bash
./tunnel-server \
  -agent-addr=:8443 \
  -agent-tls=true \
  -agent-cert=./certs/agent-cert.pem \
  -agent-key=./certs/agent-key.pem \
  -public-addr=:8080 \
  -base-domain=localhost
```

## Documentation

- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: Kiến trúc tổng quan và components
- **[USAGE.md](./USAGE.md)**: Hướng dẫn sử dụng chi tiết
- **[API.md](./API.md)**: API documentation cho các components
- **[IMPLEMENTATION.md](./IMPLEMENTATION.md)**: Implementation status
- **[config.example.yaml](./config.example.yaml)**: Example configuration file

## Features

✅ **Connection Management**: Quản lý persistent connections từ agents  
✅ **Stream Multiplexing**: Nhiều streams trên 1 connection  
✅ **Authentication**: Token-based authentication  
✅ **Domain Registry**: Mapping domain → agent connection  
✅ **HTTP/HTTPS Server**: Public listener cho incoming requests  
✅ **Request Routing**: Route requests đến đúng agent/stream  
✅ **Rate Limiting**: Token bucket algorithm cho rate limiting  
✅ **Quota Management**: Per-agent và per-domain limits  
✅ **Graceful Shutdown**: Clean shutdown với context cancellation  

## Components

- **Connection Manager**: Quản lý agent connections và streams
- **Handshake/Auth**: Authentication handshake
- **Registry**: Domain → Connection mapping
- **Public Listener**: HTTP/HTTPS server
- **Router**: Request routing và forwarding
- **Quota/Limiter**: Rate limiting và resource quotas

## Dependencies

- `github.com/hydragon2m/tunnel-protocol v0.1.1`: Protocol definitions

## Architecture

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

## Command Line Flags

### Agent Listener
- `-agent-addr`: Address for agent connections (default: `:8443`)
- `-agent-tls`: Enable TLS (default: `true`)
- `-agent-cert`: TLS certificate file
- `-agent-key`: TLS key file

### Public Listener
- `-public-addr`: Address for public HTTP requests (default: `:8080`)
- `-public-tls`: Enable TLS (default: `false`)
- `-public-cert`: TLS certificate file
- `-public-key`: TLS key file

### Configuration
- `-base-domain`: Base domain for tunnels (default: `localhost`)
- `-max-connections`: Max agent connections (default: `1000`)
- `-heartbeat-timeout`: Heartbeat timeout (default: `30s`)
- `-auth-timeout`: Authentication timeout (default: `10s`)

## Example Usage

### Setting Rate Limits

```go
limiter.SetAgentLimit("agent-123", 100, 10485760, 100) // 100 streams, 10MB/s, 100 req/s
limiter.SetDomainLimit("example.com", 50, 50)           // 50 streams, 50 req/s
```

## Status

✅ All core components implemented  
✅ Build successful  
✅ Ready for testing and deployment  

Xem [IMPLEMENTATION.md](./IMPLEMENTATION.md) để biết chi tiết implementation status.

