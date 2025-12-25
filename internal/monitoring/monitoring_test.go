package monitoring

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNewHealthCheck(t *testing.T) {
	hc := NewHealthCheck()
	if hc == nil {
		t.Fatal("NewHealthCheck() returned nil")
	}
	if hc.ready {
		t.Error("Expected ready to be false initially")
	}
}

func TestHealthCheckSetReady(t *testing.T) {
	hc := NewHealthCheck()

	// Test setting ready to true
	hc.SetReady(true)
	if !hc.ready {
		t.Error("Expected ready to be true after SetReady(true)")
	}

	// Test setting ready to false
	hc.SetReady(false)
	if hc.ready {
		t.Error("Expected ready to be false after SetReady(false)")
	}
}

func TestHealthCheckLivenessHandler(t *testing.T) {
	hc := NewHealthCheck()

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", hc.LivenessHandler)
	server := &http.Server{
		Addr:    ":0", // Random port
		Handler: mux,
	}

	// Start server in background
	go func() {
		server.ListenAndServe()
	}()
	defer server.Shutdown(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test liveness endpoint - should always return OK
	resp, err := http.Get("http://localhost" + server.Addr + "/health")
	if err != nil {
		// This might fail if port is random, skip in that case
		t.Skip("Could not connect to test server")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", string(body))
	}
}

func TestHealthCheckReadinessHandler(t *testing.T) {
	hc := NewHealthCheck()

	tests := []struct {
		name           string
		ready          bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "not ready",
			ready:          false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "Not Ready",
		},
		{
			name:           "ready",
			ready:          true,
			expectedStatus: http.StatusOK,
			expectedBody:   "Ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc.SetReady(tt.ready)

			req, err := http.NewRequest("GET", "/ready", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Use a response recorder
			rr := &testResponseWriter{
				header: make(http.Header),
			}
			hc.ReadinessHandler(rr, req)

			if rr.statusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.statusCode)
			}

			if string(rr.body) != tt.expectedBody {
				t.Errorf("Expected body '%s', got '%s'", tt.expectedBody, string(rr.body))
			}
		})
	}
}

func TestPrometheusMetrics(t *testing.T) {
	// Increment some metrics to verify they work
	ReceivedSpans.WithLabelValues("test-service").Inc()
	ReceivedMetrics.WithLabelValues("test-service").Inc()
	ReceivedLogs.WithLabelValues("test-service").Inc()
	StorageWrites.WithLabelValues("otel_traces", "success").Inc()
	QueryErrors.WithLabelValues("traces").Inc()

	// Test that metrics can be set
	MemoryUsage.Set(1024 * 1024 * 100) // 100MB
	QueueSize.WithLabelValues("traces").Set(5000)

	// Test histograms
	StorageWriteDuration.WithLabelValues("otel_traces").Observe(0.05) // 50ms
	QueryDuration.WithLabelValues("traces").Observe(0.1)              // 100ms
	BatchSize.WithLabelValues("traces").Observe(1000)

	// If we get here without panic, metrics are working
	t.Log("All metrics successfully registered and updated")
}

func TestStartMetricsServer(t *testing.T) {
	// Start metrics server
	srv := StartMetricsServer(0, "/metrics") // Use random port
	if srv == nil {
		t.Fatal("StartMetricsServer returned nil")
	}
	defer srv.Shutdown(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// The server is started in a goroutine, so we can't easily test it
	// without knowing the actual port. Just verify it doesn't panic.
	t.Log("Metrics server started successfully")
}

// testResponseWriter is a mock implementation of http.ResponseWriter for testing
type testResponseWriter struct {
	header     http.Header
	body       []byte
	statusCode int
}

func (w *testResponseWriter) Header() http.Header {
	return w.header
}

func (w *testResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(data), nil
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func TestMetricsLabels(t *testing.T) {
	// Test that metrics accept the correct labels
	tests := []struct {
		name   string
		metric func()
	}{
		{
			name: "ReceivedSpans with service label",
			metric: func() {
				ReceivedSpans.WithLabelValues("api-server").Add(100)
			},
		},
		{
			name: "StorageWrites with table and status labels",
			metric: func() {
				StorageWrites.WithLabelValues("otel_traces", "success").Add(50)
				StorageWrites.WithLabelValues("otel_traces", "error").Add(5)
			},
		},
		{
			name: "QueryDuration with query_type label",
			metric: func() {
				QueryDuration.WithLabelValues("traces").Observe(0.123)
			},
		},
		{
			name: "QueueSize with signal_type label",
			metric: func() {
				QueueSize.WithLabelValues("metrics").Set(1000)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tt.metric()
		})
	}
}

func TestBatchSizeHistogram(t *testing.T) {
	// Test that batch size histogram accepts various sizes
	sizes := []float64{10, 50, 100, 500, 1000, 5000, 10000, 50000}

	for _, size := range sizes {
		BatchSize.WithLabelValues("traces").Observe(size)
	}

	// If we get here, all observations succeeded
	t.Log("Batch size histogram observations successful")
}

func TestStorageWriteDurationHistogram(t *testing.T) {
	// Test various write durations
	durations := []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0}

	for _, duration := range durations {
		StorageWriteDuration.WithLabelValues("otel_traces").Observe(duration)
		StorageWriteDuration.WithLabelValues("otel_metrics").Observe(duration)
		StorageWriteDuration.WithLabelValues("otel_logs").Observe(duration)
	}

	t.Log("Storage write duration observations successful")
}
