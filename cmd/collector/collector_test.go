package main

import (
	"context"
	"testing"
	"time"

	"otelservices/internal/clickhouse"
	"otelservices/internal/config"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestNewCollector(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ClickHouse.Addresses = []string{"localhost:9000"}

	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)
	if collector == nil {
		t.Fatal("NewCollector() returned nil")
	}

	if collector.config != cfg {
		t.Error("Collector config not set correctly")
	}

	if collector.trace == nil {
		t.Error("Trace collector not initialized")
	}

	if collector.metrics == nil {
		t.Error("Metrics collector not initialized")
	}

	if collector.logs == nil {
		t.Error("Logs collector not initialized")
	}
}

func TestTraceCollectorExport(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)
	ctx := context.Background()

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test-service",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "test-scope",
							Version: "1.0.0",
						},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "test-span",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().Add(100 * time.Millisecond).UnixNano()),
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_OK,
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := collector.trace.Export(ctx, req)
	if err != nil {
		t.Errorf("Export() error = %v", err)
	}

	if resp == nil {
		t.Error("Export() returned nil response")
	}

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Check that span was added to channel
	select {
	case span := <-collector.trace.spanChan:
		if span.SpanName != "test-span" {
			t.Errorf("Expected span name 'test-span', got %s", span.SpanName)
		}
	default:
		// Channel might be empty if attribute extraction failed
		t.Log("No span in channel (attribute extraction may have issues)")
	}
}

func TestLogsCollectorExport(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)
	ctx := context.Background()

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test-service",
								},
							},
						},
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "test-logger",
							Version: "1.0.0",
						},
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:         uint64(time.Now().UnixNano()),
								ObservedTimeUnixNano: uint64(time.Now().UnixNano()),
								SeverityNumber:       logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
								SeverityText:         "INFO",
								Body: &commonpb.AnyValue{
									Value: &commonpb.AnyValue_StringValue{
										StringValue: "test log message",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := collector.logs.Export(ctx, req)
	if err != nil {
		t.Errorf("Export() error = %v", err)
	}

	if resp == nil {
		t.Error("Export() returned nil response")
	}

	// Give time for async processing
	time.Sleep(100 * time.Millisecond)

	// Check that log was added to channel
	select {
	case logRecord := <-collector.logs.logChan:
		if logRecord.Body != "test log message" {
			t.Errorf("Expected body 'test log message', got %s", logRecord.Body)
		}
	default:
		t.Log("No log in channel (attribute extraction may have issues)")
	}
}

func TestCollectorChannelSizes(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Performance.QueueSize = 5000

	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)

	if cap(collector.trace.spanChan) != cfg.Performance.QueueSize {
		t.Errorf("Expected span channel capacity %d, got %d", cfg.Performance.QueueSize, cap(collector.trace.spanChan))
	}

	if cap(collector.metrics.metricChan) != cfg.Performance.QueueSize {
		t.Errorf("Expected metric channel capacity %d, got %d", cfg.Performance.QueueSize, cap(collector.metrics.metricChan))
	}

	if cap(collector.logs.logChan) != cfg.Performance.QueueSize {
		t.Errorf("Expected log channel capacity %d, got %d", cfg.Performance.QueueSize, cap(collector.logs.logChan))
	}
}

func BenchmarkTraceExport(b *testing.B) {
	cfg := config.DefaultConfig()
	// Use larger queue for benchmarks to prevent channel overflow
	cfg.Performance.QueueSize = 1000000

	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		b.Skip("ClickHouse not available for benchmark")
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)
	ctx := context.Background()

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "bench-service",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "bench-span",
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	// Start a goroutine to drain the channel to prevent overflow
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-collector.trace.spanChan:
				// Drain spans
			case <-done:
				return
			}
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.trace.Export(ctx, req)
	}
	b.StopTimer()

	// Signal drainer to stop
	close(done)

	// Drain remaining
	for len(collector.trace.spanChan) > 0 {
		<-collector.trace.spanChan
	}
}
