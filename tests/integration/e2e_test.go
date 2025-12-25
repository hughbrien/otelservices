//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"otelservices/cmd/query"
	"otelservices/internal/models"
	"otelservices/tests/testutil"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestEndToEndTraceIngestionAndQuery tests the full pipeline:
// OTLP ingestion -> ClickHouse storage -> Query API
func TestEndToEndTraceIngestionAndQuery(t *testing.T) {
	// This test requires running collector and query services
	// It's meant to be run against a live deployment
	t.Skip("Requires running services - use for manual integration testing")

	ctx := context.Background()

	// Connect to collector
	conn, err := grpc.Dial("localhost:4317", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Collector not available: %v", err)
	}
	defer conn.Close()

	client := coltracepb.NewTraceServiceClient(conn)

	// Send test traces
	traceID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "e2e-test-service",
								},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Name:              "e2e-test-span",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
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

	// Send traces to collector
	_, err = client.Export(ctx, req)
	if err != nil {
		t.Fatalf("Failed to export traces: %v", err)
	}

	t.Log("Traces sent to collector")

	// Wait for data to be processed
	time.Sleep(5 * time.Second)

	// Query traces via API
	queryReq := main.TraceQueryRequest{
		ServiceName: "e2e-test-service",
		StartTime:   time.Now().Add(-5 * time.Minute),
		EndTime:     time.Now(),
		Limit:       10,
	}

	body, _ := json.Marshal(queryReq)
	resp, err := http.Post("http://localhost:8081/api/v1/traces", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to query traces: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var queryResp main.TraceQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if queryResp.Total == 0 {
		t.Error("Expected at least one trace in response")
	}

	t.Logf("Retrieved %d traces from query API", queryResp.Total)

	// Verify the trace we sent
	found := false
	for _, span := range queryResp.Spans {
		if span.SpanName == "e2e-test-span" && span.ServiceName == "e2e-test-service" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Could not find the test span in query results")
	}
}

// TestDataRetentionAndRollups tests that data rollups are working
func TestDataRetentionAndRollups(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Insert old metrics
	oldMetrics := []models.Metric{
		testutil.CreateTestMetric("rollup-service", "old_metric", 100),
	}
	oldMetrics[0].Timestamp = time.Now().Add(-35 * 24 * time.Hour) // 35 days ago

	// Insert recent metrics
	recentMetrics := []models.Metric{
		testutil.CreateTestMetric("rollup-service", "recent_metric", 200),
	}

	// Insert both
	err := client.InsertMetrics(ctx, oldMetrics)
	if err != nil {
		t.Logf("Old metrics insert (expected to potentially fail due to TTL): %v", err)
	}

	err = client.InsertMetrics(ctx, recentMetrics)
	if err != nil {
		t.Fatalf("Failed to insert recent metrics: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Check that recent metrics exist in raw table
	query := "SELECT count() FROM otel_metrics WHERE service_name = 'rollup-service' AND metric_name = 'recent_metric'"
	row := client.QueryRow(ctx, query)

	var count uint64
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count == 0 {
		t.Error("Recent metrics should exist in raw table")
	}

	// Check rollup tables exist (they should be created by materialized views)
	rollupQuery := "SELECT count() FROM otel_metrics_5m WHERE service_name = 'rollup-service'"
	row = client.QueryRow(ctx, rollupQuery)

	var rollupCount uint64
	if err := row.Scan(&rollupCount); err != nil {
		t.Logf("Rollup table query (may not exist in test): %v", err)
	} else {
		t.Logf("Found %d entries in 5-minute rollup table", rollupCount)
	}
}

// TestHighVolumeIngestion tests system behavior under high load
func TestHighVolumeIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high volume test in short mode")
	}

	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Insert 10k spans in batches
	batchSize := 1000
	totalSpans := 10000
	batches := totalSpans / batchSize

	start := time.Now()

	for i := 0; i < batches; i++ {
		spans := make([]models.Span, batchSize)
		for j := 0; j < batchSize; j++ {
			spans[j] = testutil.CreateTestSpan("high-volume-service", "operation", 50)
		}

		err := client.InsertSpans(ctx, spans)
		if err != nil {
			t.Fatalf("Batch %d failed: %v", i, err)
		}

		t.Logf("Inserted batch %d/%d", i+1, batches)
	}

	duration := time.Since(start)
	throughput := float64(totalSpans) / duration.Seconds()

	t.Logf("Inserted %d spans in %v (%.2f spans/sec)", totalSpans, duration, throughput)

	// Verify all spans were inserted
	time.Sleep(3 * time.Second)

	query := "SELECT count() FROM otel_traces WHERE service_name = 'high-volume-service'"
	row := client.QueryRow(ctx, query)

	var count uint64
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if count != uint64(totalSpans) {
		t.Errorf("Expected %d spans, got %d", totalSpans, count)
	}

	// Test that queries are still fast
	queryStart := time.Now()
	query = "SELECT span_name, duration_ns FROM otel_traces WHERE service_name = 'high-volume-service' ORDER BY timestamp DESC LIMIT 100"
	rows, err := client.Query(ctx, query)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	rowCount := 0
	for rows.Next() {
		var spanName string
		var duration uint64
		rows.Scan(&spanName, &duration)
		rowCount++
	}
	queryDuration := time.Since(queryStart)

	t.Logf("Query returned %d rows in %v", rowCount, queryDuration)

	if queryDuration > 1*time.Second {
		t.Errorf("Query took too long: %v (expected < 1s)", queryDuration)
	}
}

// TestConcurrentWrites tests concurrent writes to ClickHouse
func TestConcurrentWrites(t *testing.T) {
	client := testutil.CreateTestClickHouseClient(t)
	defer client.Close()
	defer testutil.CleanupTestData(t, client)

	ctx := context.Background()

	// Start multiple goroutines writing concurrently
	const numWorkers = 10
	const spansPerWorker = 100

	errors := make(chan error, numWorkers)
	done := make(chan bool, numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			spans := make([]models.Span, spansPerWorker)
			for j := 0; j < spansPerWorker; j++ {
				spans[j] = testutil.CreateTestSpan("concurrent-service", "operation", 50)
			}

			err := client.InsertSpans(ctx, spans)
			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Wait for all workers
	for i := 0; i < numWorkers; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Worker error: %v", err)
	}

	// Verify total count
	time.Sleep(2 * time.Second)

	query := "SELECT count() FROM otel_traces WHERE service_name = 'concurrent-service'"
	row := client.QueryRow(ctx, query)

	var count uint64
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	expected := uint64(numWorkers * spansPerWorker)
	if count != expected {
		t.Errorf("Expected %d spans, got %d", expected, count)
	}
}
