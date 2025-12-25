//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"otelservices/internal/models"
	"otelservices/tests/testutil"
)

// TestClickHouseMetricsIntegration tests end-to-end metric insertion and retrieval
func TestClickHouseMetricsIntegration(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Create test metrics
	metrics := []models.Metric{
		testutil.CreateTestMetric("api-server", "http_requests_total", 100),
		testutil.CreateTestMetric("api-server", "http_request_duration", 0.5),
		testutil.CreateTestMetric("database", "query_count", 50),
	}

	// Insert metrics
	err := client.InsertMetrics(ctx, metrics)
	if err != nil {
		t.Fatalf("Failed to insert metrics: %v", err)
	}

	// Give ClickHouse time to process
	time.Sleep(1 * time.Second)

	// Query back the metrics
	query := "SELECT metric_name, value, service_name FROM otel_metrics WHERE service_name = 'api-server' ORDER BY metric_name"
	rows, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query metrics: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var metricName string
		var value float64
		var serviceName string

		if err := rows.Scan(&metricName, &value, &serviceName); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		count++
		t.Logf("Found metric: %s = %f from %s", metricName, value, serviceName)
	}

	if count != 2 {
		t.Errorf("Expected 2 metrics for api-server, got %d", count)
	}
}

// TestClickHouseLogsIntegration tests end-to-end log insertion and retrieval
func TestClickHouseLogsIntegration(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Create test logs
	logs := []models.LogRecord{
		testutil.CreateTestLog("api-server", "Request processed successfully", "INFO"),
		testutil.CreateTestLog("api-server", "Connection timeout", "ERROR"),
		testutil.CreateTestLog("database", "Query executed", "DEBUG"),
	}

	// Insert logs
	err := client.InsertLogs(ctx, logs)
	if err != nil {
		t.Fatalf("Failed to insert logs: %v", err)
	}

	// Give ClickHouse time to process
	time.Sleep(1 * time.Second)

	// Query back error logs
	query := "SELECT body, severity_text FROM otel_logs WHERE service_name = 'api-server' AND severity_text = 'ERROR'"
	rows, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query logs: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var body, severity string
		if err := rows.Scan(&body, &severity); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		count++
		if body != "Connection timeout" {
			t.Errorf("Expected 'Connection timeout', got '%s'", body)
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 error log, got %d", count)
	}
}

// TestClickHouseTracesIntegration tests end-to-end trace insertion and retrieval
func TestClickHouseTracesIntegration(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Create test spans
	spans := []models.Span{
		testutil.CreateTestSpan("frontend", "GET /api/users", 100),
		testutil.CreateTestSpan("backend", "SELECT * FROM users", 50),
		testutil.CreateTestSpanWithError("backend", "UPDATE users", "Database connection failed"),
	}

	// Insert spans
	err := client.InsertSpans(ctx, spans)
	if err != nil {
		t.Fatalf("Failed to insert spans: %v", err)
	}

	// Give ClickHouse time to process
	time.Sleep(1 * time.Second)

	// Query back error spans
	query := "SELECT span_name, status_code, status_message FROM otel_traces WHERE status_code = 'error'"
	rows, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query traces: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var spanName, statusCode, statusMessage string
		if err := rows.Scan(&spanName, &statusCode, &statusMessage); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		count++
		if spanName != "UPDATE users" {
			t.Errorf("Expected 'UPDATE users', got '%s'", spanName)
		}
		if statusMessage != "Database connection failed" {
			t.Errorf("Expected error message, got '%s'", statusMessage)
		}
	}

	if count != 1 {
		t.Errorf("Expected 1 error span, got %d", count)
	}
}

// TestClickHouseBatchInsert tests inserting large batches
func TestClickHouseBatchInsert(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Create a large batch of spans
	batchSize := 1000
	spans := make([]models.Span, batchSize)
	for i := 0; i < batchSize; i++ {
		spans[i] = testutil.CreateTestSpan("batch-service", fmt.Sprintf("operation-%d", i), int64(i))
	}

	// Insert batch
	start := time.Now()
	err := client.InsertSpans(ctx, spans)
	if err != nil {
		t.Fatalf("Failed to insert batch: %v", err)
	}
	duration := time.Since(start)

	t.Logf("Inserted %d spans in %v (%.2f spans/sec)", batchSize, duration, float64(batchSize)/duration.Seconds())

	// Give ClickHouse time to process
	time.Sleep(2 * time.Second)

	// Verify count
	query := "SELECT count() FROM otel_traces WHERE service_name = 'batch-service'"
	row := client.QueryRow(ctx, query)

	var count uint64
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}

	if count != uint64(batchSize) {
		t.Errorf("Expected %d spans, got %d", batchSize, count)
	}
}

// TestClickHouseQueryPerformance tests query performance
func TestClickHouseQueryPerformance(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Insert test data
	spans := make([]models.Span, 100)
	for i := 0; i < 100; i++ {
		spans[i] = testutil.CreateTestSpan("perf-service", "operation", 100)
	}

	err := client.InsertSpans(ctx, spans)
	if err != nil {
		t.Fatalf("Failed to insert spans: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Test query performance
	query := "SELECT span_name, duration_ns FROM otel_traces WHERE service_name = 'perf-service' ORDER BY timestamp DESC LIMIT 10"

	start := time.Now()
	rows, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var spanName string
		var duration uint64
		rows.Scan(&spanName, &duration)
		count++
	}

	duration := time.Since(start)
	t.Logf("Query completed in %v, returned %d rows", duration, count)

	if duration > 1*time.Second {
		t.Errorf("Query took too long: %v", duration)
	}
}

// TestClickHouseConnectionPooling tests connection pooling
func TestClickHouseConnectionPooling(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()

	ctx := context.Background()

	// Execute multiple concurrent queries
	const numQueries = 20
	errors := make(chan error, numQueries)

	for i := 0; i < numQueries; i++ {
		go func() {
			err := client.Ping(ctx)
			errors <- err
		}()
	}

	// Collect results
	for i := 0; i < numQueries; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Ping %d failed: %v", i, err)
		}
	}
}

func BenchmarkInsertSpansBatch(b *testing.B) {
	client := testutil.CreateTestClickHouseClient(b)
	defer client.Close()

	ctx := context.Background()

	// Create a batch of 100 spans
	spans := make([]models.Span, 100)
	for i := range spans {
		spans[i] = testutil.CreateTestSpan("bench-service", "bench-operation", 50)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.InsertSpans(ctx, spans)
	}
}
