FROM golang:1.19-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sankarea ./cmd/sankarea

# Create runtime image
FROM alpine:3.18

# Add runtime dependencies and security updates
RUN apk --no-cache add ca-certificates tzdata && \
    apk upgrade --no-cache

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/sankarea /app/

# Create directories with appropriate permissions
RUN mkdir -p /app/config /app/data /app/logs /app/dashboard/templates /app/dashboard/static && \
    chmod -R 755 /app

# Copy required files
COPY config/ /app/config/
COPY dashboard/ /app/dashboard/

# Use non-root user for security
RUN adduser -D -h /app appuser && \
    chown -R appuser:appuser /app
USER appuser

# Define volume mount points for persistence
VOLUME ["/app/config", "/app/data", "/app/logs"]

# Command to run
ENTRYPOINT ["/app/sankarea"]
