# Sankarea Discord News Bot Makefile
# Author: NullMeDev
# Generated: 2025-05-22 14:50:06

.PHONY: build run clean test lint help docker docker-run install update

# Variables
BINARY_NAME=sankarea
VERSION=1.0.0
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"
COVER_PROFILE=coverage.out

# Default target
all: clean build

# Build the application
build:
	@echo "Building ${BINARY_NAME}..."
	@go build ${LDFLAGS} -o bin/${BINARY_NAME} cmd/sankarea/*.go

# Run the application
run: build
	@echo "Running ${BINARY_NAME}..."
	@./bin/${BINARY_NAME}

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -f ${COVER_PROFILE}

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=${COVER_PROFILE} ./...
	@go tool cover -html=${COVER_PROFILE}

# Lint the code
lint:
	@if command -v golangci-lint > /dev/null; then \
		echo "Running golangci-lint..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		echo "Running golangci-lint..."; \
		golangci-lint run; \
	fi

# Build Docker image
docker:
	@echo "Building Docker image..."
	@docker build -t ${BINARY_NAME}:${VERSION} .

# Run in Docker
docker-run: docker
	@echo "Running in Docker..."
	@docker run --rm -v $(PWD)/config:/app/config -v $(PWD)/data:/app/data -v $(PWD)/logs:/app/logs ${BINARY_NAME}:${VERSION}

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod tidy

# Update dependencies
update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Validate RSS feeds
validate-feeds:
	@echo "Validating RSS feeds..."
	@go run tools/validate_feeds.go

# Export data
export-data:
	@echo "Exporting data..."
	@go run tools/export_data.go --format csv --table articles

# Open dashboard
dashboard:
	@echo "Opening dashboard..."
	@if command -v python3 > /dev/null; then \
		cd tools && python3 -m http.server 8080; \
	elif command -v python > /dev/null; then \
		cd tools && python -m SimpleHTTPServer 8080; \
	else \
		echo "Python not found. Please install Python or manually open tools/dashboard.html"; \
	fi

# Show help
help:
	@echo "Sankarea Discord News Bot - Makefile Help"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the application"
	@echo "  run           Run the application locally"
	@echo "  clean         Clean build artifacts"
	@echo "  test          Run tests"
	@echo "  test-cover    Run tests with coverage"
	@echo "  lint          Lint the code"
	@echo "  docker        Build Docker image"
	@echo "  docker-run    Run in Docker"
	@echo "  install       Install dependencies"
	@echo "  update        Update dependencies"
	@echo "  validate-feeds Validate all RSS feeds"
	@echo "  export-data   Export data to CSV/JSON"
	@echo "  dashboard     Open the web dashboard"
	@echo "  help          Show this help message"
