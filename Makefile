.PHONY: help build run migrate test clean docker-build docker-up docker-down

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

## Build Commands
build-auth: ## Build auth service
	@echo "Building auth service..."
	go build -o internal/service/auth/auth-service.exe ./internal/service/auth/cmd

build-product: ## Build product service
	@echo "Building product service..."
	go build -o internal/service/product/product-service.exe ./internal/service/product/cmd

build-all: build-auth build-product ## Build all services

build: build-all ## Alias for build-all

## Run Commands
run-auth: ## Run auth service
	@echo "Running auth service on :8081..."
	cd internal/service/auth && go run ./cmd/main.go serve

run-product: ## Run product service
	@echo "Running product service on :8082..."
	cd internal/service/product && go run ./cmd/main.go serve

## Migration Commands
migrate-auth: ## Run auth service migrations
	@echo "Running auth migrations..."
	cd internal/service/auth && go run ./cmd/main.go migrate

migrate-product: ## Run product service migrations
	@echo "Running product migrations..."
	cd internal/service/product && go run ./cmd/main.go migrate

migrate-all: migrate-auth migrate-product ## Run all service migrations

migrate: migrate-all ## Alias for migrate-all

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f server
	rm -f coverage.out coverage.html

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	go mod tidy

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -f deployment/Dockerfile -t myapp:latest .

docker-up: ## Start services with Docker Compose
	@echo "Starting services..."
	docker-compose -f deployment/docker-compose.yml up -d

docker-down: ## Stop services with Docker Compose
	@echo "Stopping services..."
	docker-compose -f deployment/docker-compose.yml down

docker-logs: ## View Docker logs
	docker-compose -f deployment/docker-compose.yml logs -f auth

dev: ## Start development environment (postgres only)
	@echo "Starting development environment..."
	docker-compose -f deployment/docker-compose.yml up -d postgres

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.DEFAULT_GOAL := help

