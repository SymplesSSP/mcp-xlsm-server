# MCP XLSM Server Makefile

# Variables
BINARY_NAME=mcp-xlsm-server
DOCKER_IMAGE=mcp-xlsm-server
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: all build clean test coverage deps docker help

# Default target
all: deps test build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd

# Build for different platforms
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux ./cmd

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin ./cmd

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows.exe ./cmd

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)*

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Run linting
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Run with development config
run-dev:
	@echo "Running in development mode..."
	CONFIG_PATH=config.yaml ./$(BINARY_NAME)

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) .
	docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

docker-run:
	@echo "Running Docker container..."
	docker run -p 3000:3000 -p 9090:9090 $(DOCKER_IMAGE):latest

docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/air-verse/air@latest

dev-watch:
	@echo "Starting development server with hot reload..."
	air

# Generate documentation
docs:
	@echo "Generating documentation..."
	$(GOCMD) doc -all > docs/api.md

# Create release
release: clean deps test build-linux build-darwin build-windows
	@echo "Creating release packages..."
	mkdir -p dist
	tar -czf dist/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux config.yaml README.md
	tar -czf dist/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin config.yaml README.md
	zip -r dist/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows.exe config.yaml README.md

# Install for local development
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./cmd

# Security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Performance profiling
profile:
	@echo "Building with profiling enabled..."
	$(GOBUILD) -tags profile $(LDFLAGS) -o $(BINARY_NAME)-profile ./cmd

# Database migration (if needed in future)
migrate-up:
	@echo "Running database migrations..."
	# migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@echo "Rolling back database migrations..."
	# migrate -path migrations -database "$(DATABASE_URL)" down

# Load testing
load-test:
	@echo "Running load tests..."
	# Add load testing command here

# Integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./test/...

# Contract tests
test-contract:
	@echo "Running contract tests..."
	$(GOTEST) -v -tags=contract ./test/...

# Kubernetes deployment
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/

k8s-delete:
	@echo "Removing from Kubernetes..."
	kubectl delete -f k8s/

# Monitoring setup
monitoring-setup:
	@echo "Setting up monitoring stack..."
	docker-compose -f docker-compose.monitoring.yml up -d

monitoring-down:
	@echo "Stopping monitoring stack..."
	docker-compose -f docker-compose.monitoring.yml down

# Help target
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  coverage      - Run tests with coverage"
	@echo "  bench         - Run benchmarks"
	@echo "  deps          - Download dependencies"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  run           - Run the application"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  dev-setup     - Setup development environment"
	@echo "  dev-watch     - Start development server with hot reload"
	@echo "  release       - Create release packages"
	@echo "  help          - Show this help message"