package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"otelservices/internal/clickhouse"
	"otelservices/internal/config"
	"otelservices/internal/models"
)

// CreateTestConfig returns a configuration suitable for testing
func CreateTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	// Use 127.0.0.1 instead of localhost to force IPv4
	cfg.ClickHouse.Addresses = []string{"127.0.0.1:9000"}
	cfg.ClickHouse.Database = "otel"
	cfg.Performance.BatchSize = 100
	cfg.Performance.WorkerCount = 2
	cfg.Performance.QueueSize = 1000
	return cfg
}

// CreateTestClickHouseClient creates a ClickHouse client for testing
// Skips the test if ClickHouse is not available
// Accepts both *testing.T and *testing.B through the testing.TB interface
func CreateTestClickHouseClient(t testing.TB) *clickhouse.Client {
	t.Helper()

	cfg := CreateTestConfig()
	client, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skipf("ClickHouse not available: %v", err)
	}

	return client
}

// CreateTestMetric creates a test metric with default values
func CreateTestMetric(serviceName string, metricName string, value float64) models.Metric {
	return models.Metric{
		Timestamp:                   time.Now(),
		MetricName:                  metricName,
		MetricType:                  "gauge",
		Value:                       value,
		ServiceName:                 serviceName,
		ServiceNamespace:            "test",
		ServiceInstanceID:           "test-instance",
		DeploymentEnvironment:       "test",
		Attributes:                  map[string]string{"test": "true"},
		ResourceAttributes:          map[string]string{"resource": "test"},
		BucketCounts:                []uint64{},
		ExplicitBounds:              []float64{},
		InstrumentationScopeName:    "test-scope",
		InstrumentationScopeVersion: "1.0.0",
	}
}

// CreateTestLog creates a test log record with default values
func CreateTestLog(serviceName string, message string, severity string) models.LogRecord {
	severityMap := map[string]uint8{
		"TRACE": 1,
		"DEBUG": 5,
		"INFO":  9,
		"WARN":  13,
		"ERROR": 17,
		"FATAL": 21,
	}

	severityNum := severityMap[severity]
	if severityNum == 0 {
		severityNum = 9 // Default to INFO
	}

	return models.LogRecord{
		Timestamp:                   time.Now(),
		ObservedTimestamp:           time.Now(),
		SeverityNumber:              severityNum,
		SeverityText:                severity,
		Body:                        message,
		BodyType:                    "string",
		ServiceName:                 serviceName,
		ServiceNamespace:            "test",
		ServiceInstanceID:           "test-instance",
		DeploymentEnvironment:       "test",
		HostName:                    "test-host",
		TraceID:                     generateTraceID(),
		SpanID:                      generateSpanID(),
		TraceFlags:                  1,
		Attributes:                  map[string]string{"test": "true"},
		ResourceAttributes:          map[string]string{"resource": "test"},
		InstrumentationScopeName:    "test-logger",
		InstrumentationScopeVersion: "1.0.0",
	}
}

// CreateTestSpan creates a test span with default values
func CreateTestSpan(serviceName string, spanName string, durationMs int64) models.Span {
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(durationMs) * time.Millisecond)

	return models.Span{
		Timestamp:                   startTime,
		TraceID:                     generateTraceID(),
		SpanID:                      generateSpanID(),
		ParentSpanID:                "",
		SpanName:                    spanName,
		SpanKind:                    "internal",
		StartTime:                   startTime,
		EndTime:                     endTime,
		DurationNs:                  uint64(durationMs * 1000000),
		StatusCode:                  "ok",
		StatusMessage:               "",
		ServiceName:                 serviceName,
		ServiceNamespace:            "test",
		ServiceInstanceID:           "test-instance",
		DeploymentEnvironment:       "test",
		Attributes:                  map[string]string{"test": "true"},
		ResourceAttributes:          map[string]string{"resource": "test"},
		Events:                      []models.SpanEvent{},
		Links:                       []models.SpanLink{},
		InstrumentationScopeName:    "test-tracer",
		InstrumentationScopeVersion: "1.0.0",
	}
}

// CreateTestSpanWithError creates a test span with error status
func CreateTestSpanWithError(serviceName string, spanName string, errorMsg string) models.Span {
	span := CreateTestSpan(serviceName, spanName, 100)
	span.StatusCode = "error"
	span.StatusMessage = errorMsg
	return span
}

// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(t testing.TB, condition func() bool, timeout time.Duration, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("Timeout waiting for condition: %s", message)
		}

		<-ticker.C
	}
}

// CleanupTestData cleans up test data from ClickHouse
func CleanupTestData(t testing.TB, client *clickhouse.Client) {
	t.Helper()

	ctx := context.Background()

	// Clean up test tables (assuming they exist)
	queries := []string{
		"TRUNCATE TABLE IF EXISTS otel_metrics",
		"TRUNCATE TABLE IF EXISTS otel_logs",
		"TRUNCATE TABLE IF EXISTS otel_traces",
	}

	for _, query := range queries {
		_, err := client.Query(ctx, query)
		if err != nil {
			t.Logf("Cleanup warning: %v", err)
		}
	}
}

// Helper functions for ID generation
func generateTraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}

func generateSpanID() string {
	return fmt.Sprintf("span-%d", time.Now().UnixNano())
}

// AssertMetricsEqual checks if two metrics are equal
func AssertMetricsEqual(t testing.TB, expected, actual models.Metric) {
	t.Helper()

	if expected.MetricName != actual.MetricName {
		t.Errorf("MetricName: expected %s, got %s", expected.MetricName, actual.MetricName)
	}
	if expected.Value != actual.Value {
		t.Errorf("Value: expected %f, got %f", expected.Value, actual.Value)
	}
	if expected.ServiceName != actual.ServiceName {
		t.Errorf("ServiceName: expected %s, got %s", expected.ServiceName, actual.ServiceName)
	}
}

// AssertLogsEqual checks if two log records are equal
func AssertLogsEqual(t testing.TB, expected, actual models.LogRecord) {
	t.Helper()

	if expected.Body != actual.Body {
		t.Errorf("Body: expected %s, got %s", expected.Body, actual.Body)
	}
	if expected.SeverityText != actual.SeverityText {
		t.Errorf("SeverityText: expected %s, got %s", expected.SeverityText, actual.SeverityText)
	}
	if expected.ServiceName != actual.ServiceName {
		t.Errorf("ServiceName: expected %s, got %s", expected.ServiceName, actual.ServiceName)
	}
}

// AssertSpansEqual checks if two spans are equal
func AssertSpansEqual(t testing.TB, expected, actual models.Span) {
	t.Helper()

	if expected.SpanName != actual.SpanName {
		t.Errorf("SpanName: expected %s, got %s", expected.SpanName, actual.SpanName)
	}
	if expected.StatusCode != actual.StatusCode {
		t.Errorf("StatusCode: expected %s, got %s", expected.StatusCode, actual.StatusCode)
	}
	if expected.ServiceName != actual.ServiceName {
		t.Errorf("ServiceName: expected %s, got %s", expected.ServiceName, actual.ServiceName)
	}
}
