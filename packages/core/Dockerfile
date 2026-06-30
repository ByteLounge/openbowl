# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum from packages/core
COPY packages/core/go.mod packages/core/go.sum ./packages/core/
WORKDIR /app/packages/core
RUN go mod download

# Copy the rest of the core codebase
WORKDIR /app
COPY packages/core/ ./packages/core/

# Compile the Go application as a static binary
WORKDIR /app/packages/core
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o openbowl-server cmd/server/main.go

# Production runtime stage
FROM alpine:latest

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/packages/core/openbowl-server .

# Expose Gin HTTP and WebSockets port
EXPOSE 3010

# Create volume mount point for persistent SQLite database
VOLUME ["/app/data"]

# Configure environment variables
ENV PORT=3010
ENV DATABASE_PATH=/app/data/openbowl.db
ENV WORKSPACE_DIR=/app/workspace
ENV GIN_MODE=release

# Start OpenBowl Sidecar Server
CMD ["./openbowl-server"]
