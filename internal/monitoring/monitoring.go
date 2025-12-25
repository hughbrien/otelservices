package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

var (
	// Metrics for data ingestion
	ReceivedSpans = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otel_received_spans_total",
			Help: "Total number of spans received",
		},
		[]string{"service"},
	)

	ReceivedMetrics = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otel_received_metrics_total",
			Help: "Total number of metrics received",
		},
		[]string{"service"},
	)

	ReceivedLogs = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otel_received_logs_total",
			Help: "Total number of logs received",
		},
		[]string{"service"},
	)

	// Metrics for storage operations
	StorageWrites = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otel_storage_writes_total",
			Help: "Total number of storage write operations",
		},
		[]string{"table", "status"},
	)

	StorageWriteDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "otel_storage_write_duration_seconds",
			Help:    "Duration of storage write operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"table"},
	)

	BatchSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "otel_batch_size",
			Help:    "Size of batches sent to storage",
			Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000, 50000},
		},
		[]string{"signal_type"},
	)

	// Metrics for queries
	QueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "otel_query_duration_seconds",
			Help:    "Duration of query operations",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10},
		},
		[]string{"query_type"},
	)

	QueryErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "otel_query_errors_total",
			Help: "Total number of query errors",
		},
		[]string{"query_type"},
	)

	// System metrics
	MemoryUsage = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "otel_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		},
	)

	QueueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "otel_queue_size",
			Help: "Current size of processing queues",
		},
		[]string{"signal_type"},
	)
)

// InitTracing initializes OpenTelemetry tracing
func InitTracing(serviceName, serviceVersion string, sampleRate float64) (func(context.Context) error, error) {
	ctx := context.Background()

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRate)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(port int, path string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	return srv
}

// HealthCheck represents a health check handler
type HealthCheck struct {
	ready bool
}

// NewHealthCheck creates a new health check handler
func NewHealthCheck() *HealthCheck {
	return &HealthCheck{ready: false}
}

// SetReady marks the service as ready
func (h *HealthCheck) SetReady(ready bool) {
	h.ready = ready
}

// LivenessHandler handles liveness probe requests
func (h *HealthCheck) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ReadinessHandler handles readiness probe requests
func (h *HealthCheck) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if h.ready {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
	}
}
