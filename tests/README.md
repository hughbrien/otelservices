# Testing Guide

This directory contains tests for the OpenTelemetry Services project.

## Test Structure

```
tests/
├── integration/         # Integration tests (require ClickHouse)
│   ├── clickhouse_integration_test.go
│   └── e2e_test.go
└── testutil/           # Test utilities and helpers
    └── helpers.go
```

Individual package tests are located alongside the source code:
- `internal/config/config_test.go`
- `internal/models/models_test.go`
- `internal/monitoring/monitoring_test.go`
- `internal/clickhouse/client_test.go`
- `cmd/collector/collector_test.go`
- `cmd/query/query_test.go`

## Test Categories

### Unit Tests

Unit tests don't require external dependencies and test individual functions/components in isolation.

**Run unit tests:**
```bash
go test ./...
```

**Run unit tests with coverage:**
```bash
go test -cover ./...
```

**Run unit tests with verbose output:**
```bash
go test -v ./...
```

### Integration Tests

Integration tests require a running ClickHouse instance and test components working together.

**Prerequisites:**
1. Start ClickHouse:
```bash
cd deployments/docker
docker-compose up -d clickhouse
```

2. Initialize schema:
```bash
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/001_create_otel_metrics.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/002_create_otel_logs.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/003_create_otel_traces.sql
```

**Run integration tests:**
```bash
go test -tags=integration ./tests/integration/...
```

**Run integration tests with verbose output:**
```bash
go test -v -tags=integration ./tests/integration/...
```

### End-to-End Tests

E2E tests require all services (collector, query, ClickHouse) to be running.

**Prerequisites:**
1. Start all services:
```bash
cd deployments/docker
docker-compose up -d
```

2. Initialize schema (if not already done)

**Run E2E tests:**
```bash
go test -tags=integration -run TestEndToEnd ./tests/integration/...
```

## Running Specific Tests

### Run tests for a specific package:
```bash
go test ./internal/config/
go test ./internal/models/
go test ./internal/monitoring/
go test ./cmd/collector/
go test ./cmd/query/
```

### Run a specific test function:
```bash
go test -run TestDefaultConfig ./internal/config/
go test -run TestMetricModel ./internal/models/
go test -run TestHealthCheck ./internal/monitoring/
```

### Run tests matching a pattern:
```bash
go test -run Config ./...          # All tests with "Config" in the name
go test -run Integration ./...     # All integration tests
```

## Benchmarks

Run benchmarks to measure performance:

```bash
# Run all benchmarks
go test -bench=. ./...

# Run specific benchmark
go test -bench=BenchmarkInsertSpans ./internal/clickhouse/

# Run benchmark with memory profiling
go test -bench=BenchmarkInsertSpans -benchmem ./internal/clickhouse/

# Run benchmark multiple times for accuracy
go test -bench=BenchmarkInsertSpans -count=5 ./internal/clickhouse/
```

## Test Coverage

### Generate coverage report:
```bash
go test -coverprofile=coverage.out ./...
```

### View coverage in terminal:
```bash
go tool cover -func=coverage.out
```

### Generate HTML coverage report:
```bash
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Check coverage by package:
```bash
go test -cover ./internal/config/
go test -cover ./internal/models/
go test -cover ./internal/monitoring/
go test -cover ./internal/clickhouse/
```

## Test Utilities

The `testutil` package provides helper functions for tests:

### Creating Test Data

```go
import "otelservices/tests/testutil"

// Create test metric
metric := testutil.CreateTestMetric("service-name", "metric-name", 42.0)

// Create test log
log := testutil.CreateTestLog("service-name", "Log message", "ERROR")

// Create test span
span := testutil.CreateTestSpan("service-name", "operation-name", 100) // 100ms duration

// Create test span with error
span := testutil.CreateTestSpanWithError("service-name", "operation", "Error message")
```

### Test Configuration

```go
// Get test configuration
cfg := testutil.CreateTestConfig()

// Create test ClickHouse client (skips if not available)
client := testutil.CreateTestClickHouseClient(t)
defer client.Close()
```

### Assertions

```go
// Assert metrics are equal
testutil.AssertMetricsEqual(t, expected, actual)

// Assert logs are equal
testutil.AssertLogsEqual(t, expected, actual)

// Assert spans are equal
testutil.AssertSpansEqual(t, expected, actual)
```

### Utilities

```go
// Wait for a condition with timeout
testutil.WaitForCondition(t, func() bool {
    return someCondition()
}, 5*time.Second, "waiting for condition")

// Cleanup test data
testutil.CleanupTestData(t, client)
```

## Continuous Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - name: Run unit tests
        run: go test -v -cover ./...

  integration-tests:
    runs-on: ubuntu-latest
    services:
      clickhouse:
        image: clickhouse/clickhouse-server:23.8
        ports:
          - 9000:9000
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - name: Initialize schema
        run: |
          cat schema/*.sql | docker exec -i clickhouse clickhouse-client
      - name: Run integration tests
        run: go test -v -tags=integration ./tests/integration/...
```

## Testing Best Practices

1. **Isolation**: Each test should be independent and not rely on other tests
2. **Cleanup**: Always clean up resources (close connections, delete test data)
3. **Skip When Needed**: Use `t.Skip()` when dependencies are unavailable
4. **Descriptive Names**: Test names should clearly describe what they test
5. **Table-Driven**: Use table-driven tests for multiple similar test cases
6. **Coverage**: Aim for >80% code coverage, but focus on meaningful tests
7. **Fast Tests**: Keep unit tests fast (<100ms), integration tests reasonable (<5s)
8. **Error Cases**: Test both success and failure scenarios
9. **Concurrency**: Test concurrent access where applicable
10. **Documentation**: Add comments explaining complex test setups

## Troubleshooting

### Tests Skip with "ClickHouse not available"

Make sure ClickHouse is running:
```bash
docker-compose up -d clickhouse
docker exec -it otel-clickhouse clickhouse-client --query "SELECT 1"
```

### Integration Tests Fail

1. Check ClickHouse is running and healthy
2. Verify schema is initialized
3. Check for port conflicts (9000, 8123)
4. Review ClickHouse logs:
```bash
docker logs otel-clickhouse
```

### Tests Are Slow

1. Run only specific tests instead of all tests
2. Use `-short` flag to skip long-running tests:
```bash
go test -short ./...
```
3. Increase ClickHouse resources in docker-compose.yaml

### Coverage Not Generated

Make sure you're in the correct directory and have write permissions:
```bash
cd /path/to/otelservices
go test -coverprofile=coverage.out ./...
```

## Performance Testing

See `benchmarks/README.md` for load testing and performance benchmarking.

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Go Test Coverage](https://blog.golang.org/cover)
- [ClickHouse Testing](https://clickhouse.com/docs/en/development/tests)
