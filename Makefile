.PHONY: help run build clean test docker-up docker-down docker-logs examples

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run the application
	@echo "Running span-links-demo..."
	@go run main.go

build: ## Build the application
	@echo "Building span-links-demo..."
	@go build -o span-links-demo main.go
	@echo "Built: ./span-links-demo"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f span-links-demo
	@go clean
	@echo "Done"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

docker-up: ## Start local SigNoz with Docker Compose
	@echo "Starting SigNoz..."
	@docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@docker-compose ps
	@echo ""
	@echo "SigNoz should be available at: http://localhost:3301"

docker-down: ## Stop local SigNoz
	@echo "Stopping SigNoz..."
	@docker-compose down
	@echo "Done"

docker-logs: ## View SigNoz logs
	@docker-compose logs -f

docker-restart: ## Restart SigNoz
	@make docker-down
	@make docker-up

examples: ## Run example patterns
	@echo "Running examples..."
	@echo ""
	@echo "=== Fan-out Example ==="
	@go run examples/fanout.go
	@echo ""
	@echo "=== Fan-in Example ==="
	@go run examples/fanin.go
	@echo ""
	@echo "=== Retry Example ==="
	@go run examples/retry.go

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Done"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Done"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run || echo "golangci-lint not installed, skipping"

check: fmt lint test ## Run all checks

install-tools: ## Install development tools
	@echo "Installing tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Done"

