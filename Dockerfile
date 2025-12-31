FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tunnel-server ./cmd/tunnel-server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/tunnel-server .

# Create certs directory
RUN mkdir -p /root/certs

# Expose ports
EXPOSE 8443 8080

# Run
CMD ["./tunnel-server"]

