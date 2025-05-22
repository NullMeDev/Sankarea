FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o /sankarea cmd/sankarea/*.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /sankarea /app/sankarea
COPY config /app/config/

# Create necessary directories
RUN mkdir -p data logs

# Set environment variable to indicate running in Docker
ENV SANKAREA_DOCKER=true

# Set the entrypoint
ENTRYPOINT ["/app/sankarea"]
