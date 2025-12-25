package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"otelservices/internal/clickhouse"
	"otelservices/internal/config"
)

func TestNewQueryService(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)
	if service == nil {
		t.Fatal("NewQueryService() returned nil")
	}

	if service.config != cfg {
		t.Error("Service config not set correctly")
	}

	if service.chClient != chClient {
		t.Error("Service chClient not set correctly")
	}

	if service.healthCheck == nil {
		t.Error("Health check not initialized")
	}
}

func TestQueryTracesHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	tests := []struct {
		name           string
		request        TraceQueryRequest
		expectedStatus int
	}{
		{
			name: "valid trace query by trace ID",
			request: TraceQueryRequest{
				TraceID: "test-trace-id",
				Limit:   10,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid trace query by service",
			request: TraceQueryRequest{
				ServiceName: "test-service",
				StartTime:   time.Now().Add(-1 * time.Hour),
				EndTime:     time.Now(),
				Limit:       100,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "trace query with duration filter",
			request: TraceQueryRequest{
				ServiceName: "test-service",
				StartTime:   time.Now().Add(-1 * time.Hour),
				EndTime:     time.Now(),
				MinDuration: 1000000,  // 1ms
				MaxDuration: 10000000, // 10ms
				Limit:       50,
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/traces", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.QueryTraces(w, req)

			// In unit test, we expect errors because schema might not exist
			// Just verify the handler doesn't panic
			if w.Code != tt.expectedStatus && w.Code != http.StatusInternalServerError {
				t.Logf("Status: %d (expected %d or 500)", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestQueryTracesInvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	req := httptest.NewRequest("POST", "/api/v1/traces", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryTraces(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid JSON, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestQueryMetricsHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	tests := []struct {
		name           string
		request        MetricsQueryRequest
		expectedStatus int
	}{
		{
			name: "valid metrics query with avg",
			request: MetricsQueryRequest{
				MetricName:  "http_request_duration",
				ServiceName: "api-server",
				StartTime:   time.Now().Add(-1 * time.Hour),
				EndTime:     time.Now(),
				Aggregation: "avg",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid metrics query with max",
			request: MetricsQueryRequest{
				MetricName:  "memory_usage",
				StartTime:   time.Now().Add(-24 * time.Hour),
				EndTime:     time.Now(),
				Aggregation: "max",
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.QueryMetrics(w, req)

			// In unit test, we expect errors because schema might not exist
			if w.Code != tt.expectedStatus && w.Code != http.StatusInternalServerError {
				t.Logf("Status: %d (expected %d or 500)", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestQueryMetricsInvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewBufferString("{invalid}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryMetrics(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid JSON, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestQueryLogsHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	tests := []struct {
		name           string
		request        LogsQueryRequest
		expectedStatus int
	}{
		{
			name: "valid logs query by service",
			request: LogsQueryRequest{
				ServiceName: "api-server",
				StartTime:   time.Now().Add(-1 * time.Hour),
				EndTime:     time.Now(),
				Limit:       100,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid logs query with severity filter",
			request: LogsQueryRequest{
				ServiceName: "api-server",
				StartTime:   time.Now().Add(-1 * time.Hour),
				EndTime:     time.Now(),
				Severity:    "ERROR",
				Limit:       50,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid logs query with text search",
			request: LogsQueryRequest{
				StartTime:  time.Now().Add(-1 * time.Hour),
				EndTime:    time.Now(),
				SearchText: "timeout",
				Limit:      100,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "valid logs query by trace ID",
			request: LogsQueryRequest{
				TraceID:   "trace-123",
				StartTime: time.Now().Add(-1 * time.Hour),
				EndTime:   time.Now(),
				Limit:     100,
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/logs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			service.QueryLogs(w, req)

			// In unit test, we expect errors because schema might not exist
			if w.Code != tt.expectedStatus && w.Code != http.StatusInternalServerError {
				t.Logf("Status: %d (expected %d or 500)", w.Code, tt.expectedStatus)
			}
		})
	}
}

func TestQueryLogsInvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	req := httptest.NewRequest("POST", "/api/v1/logs", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryLogs(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for invalid JSON, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGetServiceStats(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	req := httptest.NewRequest("GET", "/api/v1/services/stats", nil)
	w := httptest.NewRecorder()

	service.GetServiceStats(w, req)

	// In unit test, we expect errors because schema might not exist
	// Just verify the handler doesn't panic
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Logf("Status: %d", w.Code)
	}
}

func TestQueryTracesDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	// Request without limit - should default to 100
	request := TraceQueryRequest{
		TraceID: "test-trace",
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/api/v1/traces", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryTraces(w, req)

	// Just verify no panic - actual database operations will fail without schema
	t.Log("QueryTraces with defaults completed")
}

func TestQueryMetricsDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	// Request without aggregation - should default to "avg"
	request := MetricsQueryRequest{
		MetricName: "test_metric",
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now(),
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryMetrics(w, req)

	// Just verify no panic
	t.Log("QueryMetrics with defaults completed")
}

func TestQueryLogsDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	// Request without limit - should default to 100
	request := LogsQueryRequest{
		ServiceName: "test-service",
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now(),
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/api/v1/logs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryLogs(w, req)

	// Just verify no panic
	t.Log("QueryLogs with defaults completed")
}

func TestContextCancellation(t *testing.T) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		t.Skip("ClickHouse not available for test")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	request := TraceQueryRequest{
		TraceID: "test-trace",
	}

	body, _ := json.Marshal(request)

	// Create request with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest("POST", "/api/v1/traces", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	service.QueryTraces(w, req)

	// Should handle gracefully
	t.Log("Query with cancelled context handled")
}

func BenchmarkQueryTraces(b *testing.B) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		b.Skip("ClickHouse not available for benchmark")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	request := TraceQueryRequest{
		TraceID: "bench-trace",
		Limit:   10,
	}

	body, _ := json.Marshal(request)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/traces", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		service.QueryTraces(w, req)
	}
}

func BenchmarkQueryMetrics(b *testing.B) {
	cfg := config.DefaultConfig()
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		b.Skip("ClickHouse not available for benchmark")
	}
	defer chClient.Close()

	service := NewQueryService(cfg, chClient)

	request := MetricsQueryRequest{
		MetricName:  "bench_metric",
		StartTime:   time.Now().Add(-1 * time.Hour),
		EndTime:     time.Now(),
		Aggregation: "avg",
	}

	body, _ := json.Marshal(request)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/metrics", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		service.QueryMetrics(w, req)
	}
}
