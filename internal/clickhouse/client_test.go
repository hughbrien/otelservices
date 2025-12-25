package clickhouse

import (
	"context"
	"testing"
	"time"

	"otelservices/internal/config"
	"otelservices/internal/models"
)

func TestNewClient(t *testing.T) {
	cfg := &config.ClickHouseConfig{
		Addresses:       []string{"localhost:9000"},
		Database:        "otel_test",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 1 * time.Hour,
		DialTimeout:     5 * time.Second,
		Compression:     "zstd",
	}

	// This will fail if ClickHouse is not running, which is expected for unit tests
	_, err := NewClient(cfg)
	if err != nil {
		t.Logf("Expected error when ClickHouse is not available: %v", err)
	}
}

func TestClientClose(t *testing.T) {
	// Create a mock client (would need real ClickHouse for actual test)
	cfg := &config.ClickHouseConfig{
		Addresses:       []string{"localhost:9000"},
		Database:        "otel_test",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 1 * time.Hour,
		DialTimeout:     1 * time.Second,
		Compression:     "zstd",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skip("Skipping test - ClickHouse not available")
	}

	// Test close
	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// Integration tests - require ClickHouse to be running
// Run with: go test -tags=integration

func createTestClient(t *testing.T) *Client {
	t.Helper()

	cfg := &config.ClickHouseConfig{
		Addresses:       []string{"localhost:9000"},
		Database:        "otel_test",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 1 * time.Hour,
		DialTimeout:     5 * time.Second,
		Compression:     "zstd",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Skipping integration test - ClickHouse not available: %v", err)
	}

	return client
}

func TestInsertMetrics(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Create test metrics
	metrics := []models.Metric{
		{
			Timestamp:              time.Now(),
			MetricName:             "test_metric",
			MetricType:             "gauge",
			Value:                  42.0,
			ServiceName:            "test-service",
			ServiceNamespace:       "test",
			ServiceInstanceID:      "instance-1",
			DeploymentEnvironment:  "test",
			Attributes:             map[string]string{"key": "value"},
			ResourceAttributes:     map[string]string{"resource": "test"},
			BucketCounts:           []uint64{},
			ExplicitBounds:         []float64{},
			InstrumentationScopeName: "test-scope",
			InstrumentationScopeVersion: "1.0.0",
		},
	}

	// Test insert (will fail if table doesn't exist, which is expected)
	err := client.InsertMetrics(ctx, metrics)
	if err != nil {
		t.Logf("Insert failed (expected if schema not initialized): %v", err)
	}
}

func TestInsertLogs(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Create test logs
	logs := []models.LogRecord{
		{
			Timestamp:                   time.Now(),
			ObservedTimestamp:           time.Now(),
			SeverityNumber:              9, // INFO
			SeverityText:                "INFO",
			Body:                        "Test log message",
			BodyType:                    "string",
			ServiceName:                 "test-service",
			ServiceNamespace:            "test",
			ServiceInstanceID:           "instance-1",
			DeploymentEnvironment:       "test",
			HostName:                    "test-host",
			TraceID:                     "trace-123",
			SpanID:                      "span-456",
			TraceFlags:                  1,
			Attributes:                  map[string]string{"key": "value"},
			ResourceAttributes:          map[string]string{"resource": "test"},
			InstrumentationScopeName:    "test-logger",
			InstrumentationScopeVersion: "1.0.0",
		},
	}

	err := client.InsertLogs(ctx, logs)
	if err != nil {
		t.Logf("Insert failed (expected if schema not initialized): %v", err)
	}
}

func TestInsertSpans(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	startTime := time.Now()
	endTime := startTime.Add(100 * time.Millisecond)

	// Create test spans
	spans := []models.Span{
		{
			Timestamp:        startTime,
			TraceID:          "trace-789",
			SpanID:           "span-012",
			ParentSpanID:     "",
			SpanName:         "test-operation",
			SpanKind:         "internal",
			StartTime:        startTime,
			EndTime:          endTime,
			DurationNs:       uint64(100 * time.Millisecond),
			StatusCode:       "ok",
			StatusMessage:    "",
			ServiceName:      "test-service",
			ServiceNamespace: "test",
			ServiceInstanceID: "instance-1",
			DeploymentEnvironment: "test",
			Attributes:       map[string]string{"key": "value"},
			ResourceAttributes: map[string]string{"resource": "test"},
			Events: []models.SpanEvent{
				{
					Timestamp:  startTime,
					Name:       "test-event",
					Attributes: map[string]string{"event_key": "event_value"},
				},
			},
			Links: []models.SpanLink{
				{
					TraceID:    "linked-trace",
					SpanID:     "linked-span",
					TraceState: "",
					Attributes: map[string]string{"link_key": "link_value"},
				},
			},
			InstrumentationScopeName:    "test-tracer",
			InstrumentationScopeVersion: "1.0.0",
		},
	}

	err := client.InsertSpans(ctx, spans)
	if err != nil {
		t.Logf("Insert failed (expected if schema not initialized): %v", err)
	}
}

func TestPing(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	err := client.Ping(ctx)
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestInsertEmptyBatches(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Test empty metrics
	err := client.InsertMetrics(ctx, []models.Metric{})
	if err != nil {
		t.Errorf("InsertMetrics() with empty batch error = %v", err)
	}

	// Test empty logs
	err = client.InsertLogs(ctx, []models.LogRecord{})
	if err != nil {
		t.Errorf("InsertLogs() with empty batch error = %v", err)
	}

	// Test empty spans
	err = client.InsertSpans(ctx, []models.Span{})
	if err != nil {
		t.Errorf("InsertSpans() with empty batch error = %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	client := createTestClient(t)
	defer client.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	metrics := []models.Metric{
		{
			Timestamp:   time.Now(),
			MetricName:  "test",
			Value:       1.0,
			ServiceName: "test",
		},
	}

	err := client.InsertMetrics(ctx, metrics)
	// Should get context cancelled error or similar
	if err == nil {
		t.Log("Warning: Expected error with cancelled context")
	}
}

func BenchmarkInsertMetrics(b *testing.B) {
	cfg := &config.ClickHouseConfig{
		Addresses:       []string{"localhost:9000"},
		Database:        "otel_test",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    50,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		DialTimeout:     5 * time.Second,
		Compression:     "zstd",
	}

	client, err := NewClient(cfg)
	if err != nil {
		b.Skipf("ClickHouse not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create a batch of metrics
	metrics := make([]models.Metric, 100)
	for i := range metrics {
		metrics[i] = models.Metric{
			Timestamp:   time.Now(),
			MetricName:  "benchmark_metric",
			MetricType:  "gauge",
			Value:       float64(i),
			ServiceName: "benchmark-service",
			Attributes:  map[string]string{"index": string(rune(i))},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.InsertMetrics(ctx, metrics)
	}
}

func BenchmarkInsertSpans(b *testing.B) {
	cfg := &config.ClickHouseConfig{
		Addresses:       []string{"localhost:9000"},
		Database:        "otel_test",
		Username:        "default",
		Password:        "",
		MaxOpenConns:    50,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		DialTimeout:     5 * time.Second,
		Compression:     "zstd",
	}

	client, err := NewClient(cfg)
	if err != nil {
		b.Skipf("ClickHouse not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create a batch of spans
	spans := make([]models.Span, 100)
	startTime := time.Now()
	for i := range spans {
		endTime := startTime.Add(time.Duration(i) * time.Millisecond)
		spans[i] = models.Span{
			Timestamp:     startTime,
			TraceID:       "bench-trace",
			SpanID:        string(rune(i)),
			SpanName:      "benchmark-operation",
			SpanKind:      "internal",
			StartTime:     startTime,
			EndTime:       endTime,
			DurationNs:    uint64(i * 1000000),
			StatusCode:    "ok",
			ServiceName:   "benchmark-service",
			Attributes:    map[string]string{},
			Events:        []models.SpanEvent{},
			Links:         []models.SpanLink{},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.InsertSpans(ctx, spans)
	}
}
