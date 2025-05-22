.PHONY: build run clean install update test

# Build the application
build:
	go build -o bin/sankarea ./cmd/sankarea

# Run the application
run: build
	./bin/sankarea

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
install:
	go mod download

# Update dependencies
update:
	go get -u all
	go mod tidy

# Run tests
test:
	go test ./...

# Build Docker image
docker-build:
	docker build -t sankarea:latest .

# Run Docker container
docker-run:
	docker run --env-file .env -d --name sankarea sankarea:latest
