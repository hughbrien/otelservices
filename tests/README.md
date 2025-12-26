# Testing

Tests for the OpenTelemetry Services project.

## Quick Start

```bash
# Unit tests
go test ./...

# Integration tests (requires ClickHouse)
docker-compose -f deployments/docker/docker-compose.yaml up -d clickhouse
go test -tags=integration ./tests/integration/...

# With coverage
go test -cover ./...
```

## Test Types

### Unit Tests
No external dependencies. Test individual functions/components.

```bash
# All unit tests
go test ./...

# Specific package
go test ./internal/config/
go test ./cmd/collector/

# Verbose output
go test -v ./...
```

### Integration Tests
Require running ClickHouse instance.

**Setup:**
```bash
cd deployments/docker
docker-compose up -d clickhouse

# Initialize schema
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/001_create_otel_metrics.sql
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/002_create_otel_logs.sql
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/003_create_otel_traces.sql
```

**Run:**
```bash
go test -tags=integration ./tests/integration/...
go test -v -tags=integration ./tests/integration/...
```

### End-to-End Tests
Require all services running.

```bash
# Start services
docker-compose -f deployments/docker/docker-compose.yaml up -d

# Run E2E tests
go test -tags=integration -run TestEndToEnd ./tests/integration/...
```

## Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

## Specific Tests

```bash
# Run specific test
go test -run TestDefaultConfig ./internal/config/

# Run tests matching pattern
go test -run Config ./...
go test -run Integration ./...

# Run specific package
go test ./internal/models/
```

## Benchmarks

```bash
# All benchmarks
go test -bench=. ./...

# Specific benchmark
go test -bench=BenchmarkInsertSpans ./internal/clickhouse/

# With memory profiling
go test -bench=. -benchmem ./internal/clickhouse/

# Multiple runs for accuracy
go test -bench=. -count=5 ./internal/clickhouse/
```

## Test Structure

```
tests/
├── integration/         # Integration and E2E tests
└── testutil/           # Test helpers

Package tests alongside source:
├── internal/config/config_test.go
├── internal/models/models_test.go
├── cmd/collector/collector_test.go
└── cmd/query/query_test.go
```

## Troubleshooting

**"ClickHouse not available":**
```bash
docker-compose up -d clickhouse
docker exec otel-clickhouse clickhouse-client --query "SELECT 1"
```

**Integration tests fail:**
- Verify ClickHouse is running and healthy
- Check schema is initialized
- Check port conflicts (9000, 8123)
- Review logs: `docker logs otel-clickhouse`

**Slow tests:**
```bash
# Run specific tests only
go test ./internal/config/

# Skip long tests
go test -short ./...
```

## CI Example

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      clickhouse:
        image: clickhouse/clickhouse-server:23.8
        ports: [9000:9000]
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with: {go-version: '1.21'}
      - run: cat schema/*.sql | docker exec -i clickhouse clickhouse-client
      - run: go test -v -cover ./...
      - run: go test -v -tags=integration ./tests/integration/...
```
