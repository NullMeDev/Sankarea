FROM golang:1.19-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o sankarea ./cmd/sankarea

# Create final lightweight image
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy application binary from builder
COPY --from=builder /app/sankarea /app/

# Create directories for data persistence
RUN mkdir -p /app/config /app/data /app/logs /app/dashboard

# Copy configs
COPY config/ /app/config/
COPY dashboard/ /app/dashboard/

# Set the entrypoint
ENTRYPOINT ["/app/sankarea"]
