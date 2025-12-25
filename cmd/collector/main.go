package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"otelservices/internal/clickhouse"
	"otelservices/internal/config"
	"otelservices/internal/models"
	"otelservices/internal/monitoring"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

const (
	serviceName    = "otel-collector"
	serviceVersion = "1.0.0"
)

// TraceCollector handles trace data
type TraceCollector struct {
	coltracepb.UnimplementedTraceServiceServer
	spanChan chan models.Span
	config   *config.Config
	chClient *clickhouse.Client
}

// MetricsCollector handles metrics data
type MetricsCollector struct {
	colmetricspb.UnimplementedMetricsServiceServer
	metricChan chan models.Metric
	config     *config.Config
	chClient   *clickhouse.Client
}

// LogsCollector handles log data
type LogsCollector struct {
	collogspb.UnimplementedLogsServiceServer
	logChan  chan models.LogRecord
	config   *config.Config
	chClient *clickhouse.Client
}

// Collector wraps all three collectors
type Collector struct {
	trace      *TraceCollector
	metrics    *MetricsCollector
	logs       *LogsCollector
	config     *config.Config
	chClient   *clickhouse.Client
	healthCheck *monitoring.HealthCheck
	wg         sync.WaitGroup
}

// NewCollector creates a new collector instance
func NewCollector(cfg *config.Config, chClient *clickhouse.Client) *Collector {
	return &Collector{
		trace: &TraceCollector{
			spanChan: make(chan models.Span, cfg.Performance.QueueSize),
			config:   cfg,
			chClient: chClient,
		},
		metrics: &MetricsCollector{
			metricChan: make(chan models.Metric, cfg.Performance.QueueSize),
			config:     cfg,
			chClient:   chClient,
		},
		logs: &LogsCollector{
			logChan:  make(chan models.LogRecord, cfg.Performance.QueueSize),
			config:   cfg,
			chClient: chClient,
		},
		config:      cfg,
		chClient:    chClient,
		healthCheck: monitoring.NewHealthCheck(),
	}
}

// Export implements TraceServiceServer
func (tc *TraceCollector) Export(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	for _, rs := range req.ResourceSpans {
		serviceName := extractStringAttribute(rs.Resource, "service.name")
		serviceNamespace := extractStringAttribute(rs.Resource, "service.namespace")
		serviceInstanceID := extractStringAttribute(rs.Resource, "service.instance.id")
		deploymentEnv := extractStringAttribute(rs.Resource, "deployment.environment")

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				modelSpan := models.Span{
					Timestamp:             time.Unix(0, int64(span.StartTimeUnixNano)),
					TraceID:               fmt.Sprintf("%x", span.TraceId),
					SpanID:                fmt.Sprintf("%x", span.SpanId),
					ParentSpanID:          fmt.Sprintf("%x", span.ParentSpanId),
					SpanName:              span.Name,
					SpanKind:              span.Kind.String(),
					StartTime:             time.Unix(0, int64(span.StartTimeUnixNano)),
					EndTime:               time.Unix(0, int64(span.EndTimeUnixNano)),
					DurationNs:            span.EndTimeUnixNano - span.StartTimeUnixNano,
					StatusCode:            span.Status.GetCode().String(),
					StatusMessage:         span.Status.GetMessage(),
					ServiceName:           serviceName,
					ServiceNamespace:      serviceNamespace,
					ServiceInstanceID:     serviceInstanceID,
					DeploymentEnvironment: deploymentEnv,
					Attributes:            convertAttributes(span.Attributes),
					ResourceAttributes:    make(map[string]string),
					Events:                []models.SpanEvent{},
					Links:                 []models.SpanLink{},
				}

				select {
				case tc.spanChan <- modelSpan:
					monitoring.ReceivedSpans.WithLabelValues(serviceName).Inc()
				case <-time.After(100 * time.Millisecond):
					log.Printf("Warning: span channel full")
				}
			}
		}
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

// Export implements MetricsServiceServer
func (mc *MetricsCollector) Export(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	for _, rm := range req.ResourceMetrics {
		serviceName := extractStringAttribute(rm.Resource, "service.name")
		for range rm.ScopeMetrics {
			monitoring.ReceivedMetrics.WithLabelValues(serviceName).Inc()
		}
	}
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

// Export implements LogsServiceServer
func (lc *LogsCollector) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	for _, rl := range req.ResourceLogs {
		serviceName := extractStringAttribute(rl.Resource, "service.name")
		serviceNamespace := extractStringAttribute(rl.Resource, "service.namespace")
		serviceInstanceID := extractStringAttribute(rl.Resource, "service.instance.id")
		deploymentEnv := extractStringAttribute(rl.Resource, "deployment.environment")
		hostName := extractStringAttribute(rl.Resource, "host.name")

		for _, sl := range rl.ScopeLogs {
			for _, logRecord := range sl.LogRecords {
				modelLog := models.LogRecord{
					Timestamp:             time.Unix(0, int64(logRecord.TimeUnixNano)),
					ObservedTimestamp:     time.Unix(0, int64(logRecord.ObservedTimeUnixNano)),
					SeverityNumber:        uint8(logRecord.SeverityNumber),
					SeverityText:          logRecord.SeverityText,
					Body:                  logRecord.Body.GetStringValue(),
					BodyType:              "string",
					ServiceName:           serviceName,
					ServiceNamespace:      serviceNamespace,
					ServiceInstanceID:     serviceInstanceID,
					DeploymentEnvironment: deploymentEnv,
					HostName:              hostName,
					TraceID:               fmt.Sprintf("%x", logRecord.TraceId),
					SpanID:                fmt.Sprintf("%x", logRecord.SpanId),
					TraceFlags:            uint8(logRecord.Flags),
					Attributes:            convertAttributes(logRecord.Attributes),
					ResourceAttributes:    make(map[string]string),
				}

				select {
				case lc.logChan <- modelLog:
					monitoring.ReceivedLogs.WithLabelValues(serviceName).Inc()
				case <-time.After(100 * time.Millisecond):
					log.Printf("Warning: log channel full")
				}
			}
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

// Helper functions
func extractStringAttribute(resource interface{}, key string) string {
	type resourceWithAttrs interface {
		GetAttributes() interface{}
	}
	if r, ok := resource.(resourceWithAttrs); ok {
		if attrs, ok := r.GetAttributes().([]*interface{}); ok {
			for _, attr := range attrs {
				// Simplified - would need proper type assertion
				_ = attr
			}
		}
	}
	return ""
}

func convertAttributes(attrs interface{}) map[string]string {
	result := make(map[string]string)
	// Simplified conversion
	return result
}

// startBatchProcessor starts background workers
func (c *Collector) startBatchProcessor(ctx context.Context) {
	for i := 0; i < c.config.Performance.WorkerCount; i++ {
		c.wg.Add(3)
		go c.processSpans(ctx)
		go c.processMetrics(ctx)
		go c.processLogs(ctx)
	}
}

func (c *Collector) processSpans(ctx context.Context) {
	defer c.wg.Done()
	batch := make([]models.Span, 0, c.config.Performance.BatchSize)
	ticker := time.NewTicker(c.config.Performance.BatchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := c.chClient.InsertSpans(ctx, batch); err != nil {
			log.Printf("Error inserting spans: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case span := <-c.trace.spanChan:
			batch = append(batch, span)
			if len(batch) >= c.config.Performance.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *Collector) processMetrics(ctx context.Context) {
	defer c.wg.Done()
	batch := make([]models.Metric, 0, c.config.Performance.BatchSize)
	ticker := time.NewTicker(c.config.Performance.BatchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := c.chClient.InsertMetrics(ctx, batch); err != nil {
			log.Printf("Error inserting metrics: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case metric := <-c.metrics.metricChan:
			batch = append(batch, metric)
			if len(batch) >= c.config.Performance.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *Collector) processLogs(ctx context.Context) {
	defer c.wg.Done()
	batch := make([]models.LogRecord, 0, c.config.Performance.BatchSize)
	ticker := time.NewTicker(c.config.Performance.BatchTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := c.chClient.InsertLogs(ctx, batch); err != nil {
			log.Printf("Error inserting logs: %v", err)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case logRecord := <-c.logs.logChan:
			batch = append(batch, logRecord)
			if len(batch) >= c.config.Performance.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/collector.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	shutdown, err := monitoring.InitTracing(serviceName, serviceVersion, cfg.Monitoring.TraceSampleRate)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer shutdown(context.Background())

	metricsServer := monitoring.StartMetricsServer(cfg.Monitoring.MetricsPort, cfg.Monitoring.MetricsPath)
	defer metricsServer.Shutdown(context.Background())

	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chClient.Close()

	collector := NewCollector(cfg, chClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector.startBatchProcessor(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.OTLP.GRPCPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(grpcServer, collector.trace)
	colmetricspb.RegisterMetricsServiceServer(grpcServer, collector.metrics)
	collogspb.RegisterLogsServiceServer(grpcServer, collector.logs)
	reflection.Register(grpcServer)

	healthMux := http.NewServeMux()
	healthMux.HandleFunc(cfg.Monitoring.HealthCheckPath, collector.healthCheck.LivenessHandler)
	healthMux.HandleFunc(cfg.Monitoring.ReadyCheckPath, collector.healthCheck.ReadinessHandler)
	healthServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: healthMux,
	}

	go func() {
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	collector.healthCheck.SetReady(true)
	log.Printf("OTLP Collector started on port %d", cfg.OTLP.GRPCPort)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gracefully...")
	collector.healthCheck.SetReady(false)
	cancel()
	grpcServer.GracefulStop()
	collector.wg.Wait()
	log.Println("Shutdown complete")
}
