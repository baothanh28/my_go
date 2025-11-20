.PHONY: help build run migrate test clean docker-build docker-up docker-down supabase-up supabase-down supabase-logs supabase-build

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

## Build Commands
build-auth: ## Build auth service
	@echo "Building auth service..."
	go build -o internal/service/auth/auth-service.exe ./internal/service/auth/cmd

build-notification: ## Build notification service
	@echo "Building notification service..."
	go build -o internal/service/notification/notification-service.exe ./internal/service/notification/cmd

build-all: build-auth build-notification  ## Build all services

build: build-all ## Alias for build-all

## Run Commands
run-auth: ## Run auth service
	@echo "Running auth service on :8081..."
	@set APP_SERVER_PORT=8081&& go run ./internal/service/auth/cmd serve

run-notification: ## Run notification service
	@echo "Running notification service on :8082..."
	@set APP_SERVER_PORT=8082&& go run ./internal/service/notification/cmd serve

run-all: ## Run all services (requires multiple terminals)
	@echo "To run all services, open separate terminals and run:"
	@echo "  make run-auth"
	@echo "  make run-notification"

## Migration Commands
migrate-auth: ## Run auth service migrations
	@echo "Running auth migrations..."
	@set APP_SERVER_PORT=8081&& go run ./internal/service/auth/cmd migrate

migrate-notification: ## Run notification service migrations
	@echo "Running notification migrations..."
	@set APP_SERVER_PORT=8082&& go run ./internal/service/notification/cmd migrate

migrate-all: migrate-auth migrate-notification ## Run all service migrations

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

## Supabase-only (Postgres + Supabase login service)
supabase-build: ## Build Supabase-only images
	@echo "Building Supabase images..."
	docker-compose -f deployment/docker-compose.supabase.yml build

supabase-up: ## Start Supabase-only services
	@echo "Starting Supabase (postgres + supabase-login)..."
	docker-compose -f deployment/docker-compose.supabase.yml up -d

supabase-down: ## Stop Supabase-only services
	@echo "Stopping Supabase services..."
	docker-compose -f deployment/docker-compose.supabase.yml down

supabase-logs: ## View logs for Supabase login service
	docker-compose -f deployment/docker-compose.supabase.yml logs -f supabase-login

dev: ## Start development environment (postgres only)
	@echo "Starting development environment..."
	docker-compose -f deployment/docker-compose.yml up -d postgres

install-tools: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/cosmtrek/air@latest

## Hot Reload Commands (using Air)
dev-auth: ## Run auth service with hot reload
	@echo "Starting auth service with hot reload on :8081..."
	@if not exist tmp\auth mkdir tmp\auth
	@air -c .air.auth.toml

dev-notification: ## Run notification service with hot reload
	@echo "Starting notification service with hot reload on :8082..."
	@if not exist tmp\notification mkdir tmp\notification
	@air -c .air.notification.toml

dev-all: ## Run all services with hot reload (requires multiple terminals)
	@echo "To run all services with hot reload, open separate terminals and run:"
	@echo "  make dev-auth"
	@echo "  make dev-notification"

find-process: ## Find process ID of running services (for debugging)
	@echo "Finding running processes..."
	@powershell -ExecutionPolicy Bypass -File scripts/find-process.ps1 all

.DEFAULT_GOAL := help

