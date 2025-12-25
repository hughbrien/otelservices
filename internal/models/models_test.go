package models

import (
	"testing"
	"time"
)

func TestMetricModel(t *testing.T) {
	now := time.Now()
	metric := Metric{
		Timestamp:                   now,
		MetricName:                  "http_requests_total",
		MetricType:                  "counter",
		Value:                       100.5,
		ServiceName:                 "api-server",
		ServiceNamespace:            "production",
		ServiceInstanceID:           "instance-1",
		DeploymentEnvironment:       "prod",
		Attributes:                  map[string]string{"method": "GET", "status": "200"},
		ResourceAttributes:          map[string]string{"host": "server-1"},
		BucketCounts:                []uint64{10, 20, 30},
		ExplicitBounds:              []float64{0.1, 0.5, 1.0},
		InstrumentationScopeName:    "my-instrumentation",
		InstrumentationScopeVersion: "1.0.0",
	}

	if metric.Timestamp != now {
		t.Errorf("Expected timestamp %v, got %v", now, metric.Timestamp)
	}
	if metric.MetricName != "http_requests_total" {
		t.Errorf("Expected metric name http_requests_total, got %s", metric.MetricName)
	}
	if metric.Value != 100.5 {
		t.Errorf("Expected value 100.5, got %f", metric.Value)
	}
	if len(metric.Attributes) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(metric.Attributes))
	}
	if metric.Attributes["method"] != "GET" {
		t.Errorf("Expected method GET, got %s", metric.Attributes["method"])
	}
	if len(metric.BucketCounts) != 3 {
		t.Errorf("Expected 3 bucket counts, got %d", len(metric.BucketCounts))
	}
}

func TestLogRecordModel(t *testing.T) {
	now := time.Now()
	logRecord := LogRecord{
		Timestamp:                   now,
		ObservedTimestamp:           now.Add(time.Millisecond),
		SeverityNumber:              17, // ERROR
		SeverityText:                "ERROR",
		Body:                        "Connection timeout",
		BodyType:                    "string",
		ServiceName:                 "api-server",
		ServiceNamespace:            "production",
		ServiceInstanceID:           "instance-1",
		DeploymentEnvironment:       "prod",
		HostName:                    "server-1",
		TraceID:                     "trace-123",
		SpanID:                      "span-456",
		TraceFlags:                  1,
		Attributes:                  map[string]string{"error.type": "timeout"},
		ResourceAttributes:          map[string]string{"cloud.provider": "aws"},
		InstrumentationScopeName:    "my-logger",
		InstrumentationScopeVersion: "2.0.0",
	}

	if logRecord.SeverityNumber != 17 {
		t.Errorf("Expected severity number 17, got %d", logRecord.SeverityNumber)
	}
	if logRecord.SeverityText != "ERROR" {
		t.Errorf("Expected severity text ERROR, got %s", logRecord.SeverityText)
	}
	if logRecord.Body != "Connection timeout" {
		t.Errorf("Expected body 'Connection timeout', got %s", logRecord.Body)
	}
	if logRecord.TraceID != "trace-123" {
		t.Errorf("Expected trace ID trace-123, got %s", logRecord.TraceID)
	}
	if logRecord.SpanID != "span-456" {
		t.Errorf("Expected span ID span-456, got %s", logRecord.SpanID)
	}
}

func TestSpanModel(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(100 * time.Millisecond)
	durationNs := uint64(100 * time.Millisecond)

	span := Span{
		Timestamp:                   startTime,
		TraceID:                     "trace-789",
		SpanID:                      "span-012",
		ParentSpanID:                "span-000",
		SpanName:                    "GET /api/users",
		SpanKind:                    "server",
		StartTime:                   startTime,
		EndTime:                     endTime,
		DurationNs:                  durationNs,
		StatusCode:                  "ok",
		StatusMessage:               "",
		ServiceName:                 "api-server",
		ServiceNamespace:            "production",
		ServiceInstanceID:           "instance-1",
		DeploymentEnvironment:       "prod",
		Attributes:                  map[string]string{"http.method": "GET", "http.route": "/api/users"},
		ResourceAttributes:          map[string]string{"service.version": "1.2.3"},
		Events:                      []SpanEvent{{Timestamp: startTime, Name: "processing", Attributes: map[string]string{"step": "1"}}},
		Links:                       []SpanLink{{TraceID: "linked-trace", SpanID: "linked-span", TraceState: "", Attributes: map[string]string{}}},
		InstrumentationScopeName:    "my-tracer",
		InstrumentationScopeVersion: "3.0.0",
	}

	if span.TraceID != "trace-789" {
		t.Errorf("Expected trace ID trace-789, got %s", span.TraceID)
	}
	if span.SpanName != "GET /api/users" {
		t.Errorf("Expected span name 'GET /api/users', got %s", span.SpanName)
	}
	if span.DurationNs != durationNs {
		t.Errorf("Expected duration %d, got %d", durationNs, span.DurationNs)
	}
	if len(span.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(span.Events))
	}
	if span.Events[0].Name != "processing" {
		t.Errorf("Expected event name 'processing', got %s", span.Events[0].Name)
	}
	if len(span.Links) != 1 {
		t.Errorf("Expected 1 link, got %d", len(span.Links))
	}
}

func TestSpanEventModel(t *testing.T) {
	now := time.Now()
	event := SpanEvent{
		Timestamp:  now,
		Name:       "cache_miss",
		Attributes: map[string]string{"key": "user:123", "cache": "redis"},
	}

	if event.Name != "cache_miss" {
		t.Errorf("Expected event name cache_miss, got %s", event.Name)
	}
	if event.Attributes["key"] != "user:123" {
		t.Errorf("Expected key user:123, got %s", event.Attributes["key"])
	}
}

func TestSpanLinkModel(t *testing.T) {
	link := SpanLink{
		TraceID:    "parent-trace-id",
		SpanID:     "parent-span-id",
		TraceState: "congo=t61rcWkgMzE",
		Attributes: map[string]string{"link.type": "parent"},
	}

	if link.TraceID != "parent-trace-id" {
		t.Errorf("Expected trace ID parent-trace-id, got %s", link.TraceID)
	}
	if link.SpanID != "parent-span-id" {
		t.Errorf("Expected span ID parent-span-id, got %s", link.SpanID)
	}
	if link.TraceState != "congo=t61rcWkgMzE" {
		t.Errorf("Expected trace state congo=t61rcWkgMzE, got %s", link.TraceState)
	}
}

func TestTraceIndexModel(t *testing.T) {
	minTime := time.Now()
	maxTime := minTime.Add(5 * time.Second)

	traceIndex := TraceIndex{
		TraceID:          "trace-abc",
		MinTimestamp:     minTime,
		MaxTimestamp:     maxTime,
		ServiceNames:     []string{"frontend", "backend", "database"},
		RootServiceName:  "frontend",
		RootSpanName:     "GET /",
		DurationNs:       5000000000, // 5 seconds
		SpanCount:        15,
		HasErrors:        true,
	}

	if traceIndex.TraceID != "trace-abc" {
		t.Errorf("Expected trace ID trace-abc, got %s", traceIndex.TraceID)
	}
	if len(traceIndex.ServiceNames) != 3 {
		t.Errorf("Expected 3 service names, got %d", len(traceIndex.ServiceNames))
	}
	if traceIndex.RootServiceName != "frontend" {
		t.Errorf("Expected root service frontend, got %s", traceIndex.RootServiceName)
	}
	if traceIndex.SpanCount != 15 {
		t.Errorf("Expected span count 15, got %d", traceIndex.SpanCount)
	}
	if !traceIndex.HasErrors {
		t.Error("Expected HasErrors to be true")
	}
}

func TestMetricWithEmptyAttributes(t *testing.T) {
	metric := Metric{
		Timestamp:          time.Now(),
		MetricName:         "simple_counter",
		Value:              1.0,
		Attributes:         map[string]string{},
		ResourceAttributes: map[string]string{},
	}

	if metric.Attributes == nil {
		t.Error("Expected non-nil attributes map")
	}
	if len(metric.Attributes) != 0 {
		t.Errorf("Expected empty attributes, got %d items", len(metric.Attributes))
	}
}

func TestSpanWithNoEventsOrLinks(t *testing.T) {
	span := Span{
		TraceID:  "trace-xyz",
		SpanID:   "span-xyz",
		SpanName: "simple-operation",
		Events:   []SpanEvent{},
		Links:    []SpanLink{},
	}

	if span.Events == nil {
		t.Error("Expected non-nil events slice")
	}
	if len(span.Events) != 0 {
		t.Errorf("Expected empty events, got %d items", len(span.Events))
	}
	if span.Links == nil {
		t.Error("Expected non-nil links slice")
	}
	if len(span.Links) != 0 {
		t.Errorf("Expected empty links, got %d items", len(span.Links))
	}
}
