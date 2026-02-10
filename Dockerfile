FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy tunnel-protocol first (dependency)
COPY tunnel-protocol/go.mod tunnel-protocol/go.sum* ./tunnel-protocol/

# Copy tunnel-core go.mod files
COPY tunnel-core/go.mod tunnel-core/go.sum ./tunnel-core/

WORKDIR /build/tunnel-core

# Download dependencies
RUN go mod download

# Copy source code
COPY tunnel-protocol/ ../tunnel-protocol/
COPY tunnel-core/ .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /tunnel-server ./cmd/tunnel-server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /tunnel-server .

# Create certs directory
RUN mkdir -p /root/certs

# Expose ports
EXPOSE 8443 8080

# Run
CMD ["./tunnel-server"]
