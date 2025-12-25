.PHONY: help test test-unit test-integration test-coverage test-bench clean build run-collector run-query docker-up docker-down lint

# Default target
help:
	@echo "Available targets:"
	@echo "  test              - Run all tests (unit only)"
	@echo "  test-unit         - Run unit tests"
	@echo "  test-integration  - Run integration tests (requires ClickHouse)"
	@echo "  test-coverage     - Generate coverage report"
	@echo "  test-bench        - Run benchmarks"
	@echo "  build             - Build all binaries"
	@echo "  build-collector   - Build collector binary"
	@echo "  build-query       - Build query service binary"
	@echo "  build-loadtest    - Build load test tool"
	@echo "  run-collector     - Run collector service"
	@echo "  run-query         - Run query service"
	@echo "  docker-up         - Start all services with Docker Compose"
	@echo "  docker-down       - Stop all services"
	@echo "  docker-init       - Initialize ClickHouse schema"
	@echo "  lint              - Run linters"
	@echo "  clean             - Clean build artifacts"

# Testing
test: test-unit

test-unit:
	@echo "Running unit tests..."
	go test -v -race -cover ./internal/... ./cmd/...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./tests/integration/...

test-all: test-unit test-integration

test-coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

test-short:
	@echo "Running short tests..."
	go test -short ./...

# Building
build: build-collector build-query build-loadtest

build-collector:
	@echo "Building collector..."
	go build -o bin/collector ./cmd/collector

build-query:
	@echo "Building query service..."
	go build -o bin/query ./cmd/query

build-loadtest:
	@echo "Building load test tool..."
	go build -o bin/load_test ./benchmarks

# Running
run-collector:
	@echo "Running collector..."
	CONFIG_PATH=configs/collector.yaml go run ./cmd/collector

run-query:
	@echo "Running query service..."
	CONFIG_PATH=configs/query.yaml go run ./cmd/query

# Docker
docker-up:
	@echo "Starting services..."
	cd deployments/docker && docker-compose up -d

docker-down:
	@echo "Stopping services..."
	cd deployments/docker && docker-compose down

docker-init:
	@echo "Initializing ClickHouse schema..."
	docker exec -i otel-clickhouse clickhouse-client --multiquery < schema/001_create_otel_metrics.sql
	docker exec -i otel-clickhouse clickhouse-client --multiquery < schema/002_create_otel_logs.sql
	docker exec -i otel-clickhouse clickhouse-client --multiquery < schema/003_create_otel_traces.sql
	@echo "Schema initialized successfully"

docker-logs:
	cd deployments/docker && docker-compose logs -f

docker-restart: docker-down docker-up

# Docker build
docker-build-collector:
	@echo "Building collector Docker image..."
	docker build -f deployments/docker/Dockerfile.collector -t otel-collector:latest .

docker-build-query:
	@echo "Building query Docker image..."
	docker build -f deployments/docker/Dockerfile.query -t otel-query:latest .

docker-build: docker-build-collector docker-build-query

# Code quality
lint:
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

# Dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

deps-tidy:
	@echo "Tidying dependencies..."
	go mod tidy

deps-verify:
	@echo "Verifying dependencies..."
	go mod verify

# Cleaning
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache -testcache

clean-all: clean docker-down
	rm -rf data/

# Development workflow
dev-setup: deps docker-up docker-init
	@echo "Development environment ready!"
	@echo "Run 'make run-collector' in one terminal"
	@echo "Run 'make run-query' in another terminal"

# Quick test cycle
quick-test: fmt vet test-unit

# Full validation
validate: fmt vet lint test-all

# Load testing
loadtest:
	@echo "Running load test..."
	@if [ -f bin/load_test ]; then \
		./bin/load_test -rate 10000 -duration 1m; \
	else \
		echo "Build load test first: make build-loadtest"; \
	fi

loadtest-high:
	@echo "Running high-volume load test..."
	@if [ -f bin/load_test ]; then \
		./bin/load_test -rate 100000 -duration 5m -workers 20; \
	else \
		echo "Build load test first: make build-loadtest"; \
	fi

# ClickHouse utilities
clickhouse-shell:
	docker exec -it otel-clickhouse clickhouse-client

clickhouse-stats:
	@echo "ClickHouse storage statistics:"
	@docker exec -it otel-clickhouse clickhouse-client --query "SELECT table, formatReadableSize(sum(bytes)) AS size, formatReadableQuantity(sum(rows)) AS rows FROM system.parts WHERE database = 'otel' AND active GROUP BY table"

clickhouse-cleanup:
	@echo "Cleaning up test data..."
	@docker exec -it otel-clickhouse clickhouse-client --query "TRUNCATE TABLE IF EXISTS otel_test.otel_metrics"
	@docker exec -it otel-clickhouse clickhouse-client --query "TRUNCATE TABLE IF EXISTS otel_test.otel_logs"
	@docker exec -it otel-clickhouse clickhouse-client --query "TRUNCATE TABLE IF EXISTS otel_test.otel_traces"

# CI/CD targets
ci-test: deps test-unit

ci-integration: deps docker-up docker-init test-integration

ci-validate: validate test-coverage

# Version info
version:
	@echo "Go version:"
	@go version
	@echo ""
	@echo "Module info:"
	@go list -m

# Help for Docker commands
docker-help:
	@echo "Docker commands:"
	@echo "  make docker-up          - Start all services"
	@echo "  make docker-down        - Stop all services"
	@echo "  make docker-init        - Initialize database schema"
	@echo "  make docker-logs        - View service logs"
	@echo "  make docker-restart     - Restart all services"
	@echo "  make clickhouse-shell   - Open ClickHouse CLI"
	@echo "  make clickhouse-stats   - Show storage statistics"
