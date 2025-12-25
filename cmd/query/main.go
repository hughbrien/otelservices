package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otelservices/internal/clickhouse"
	"otelservices/internal/config"
	"otelservices/internal/monitoring"

	"github.com/gorilla/mux"
)

const (
	serviceName    = "otel-query-api"
	serviceVersion = "1.0.0"
)

// QueryService provides query APIs for OTEL data
type QueryService struct {
	config      *config.Config
	chClient    *clickhouse.Client
	healthCheck *monitoring.HealthCheck
}

// NewQueryService creates a new query service instance
func NewQueryService(cfg *config.Config, chClient *clickhouse.Client) *QueryService {
	return &QueryService{
		config:      cfg,
		chClient:    chClient,
		healthCheck: monitoring.NewHealthCheck(),
	}
}

// Trace query request/response structures
type TraceQueryRequest struct {
	TraceID   string    `json:"trace_id"`
	ServiceName string  `json:"service_name,omitempty"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	MinDuration int64   `json:"min_duration,omitempty"`
	MaxDuration int64   `json:"max_duration,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

type Span struct {
	TraceID       string            `json:"trace_id"`
	SpanID        string            `json:"span_id"`
	ParentSpanID  string            `json:"parent_span_id"`
	SpanName      string            `json:"span_name"`
	SpanKind      string            `json:"span_kind"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	DurationNs    uint64            `json:"duration_ns"`
	StatusCode    string            `json:"status_code"`
	StatusMessage string            `json:"status_message"`
	ServiceName   string            `json:"service_name"`
	Attributes    map[string]string `json:"attributes"`
}

type TraceQueryResponse struct {
	Spans []Span `json:"spans"`
	Total int    `json:"total"`
}

// Metrics query structures
type MetricsQueryRequest struct {
	MetricName string            `json:"metric_name"`
	ServiceName string           `json:"service_name,omitempty"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Aggregation string           `json:"aggregation,omitempty"` // avg, min, max, sum
	GroupBy    []string          `json:"group_by,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
	Step       string            `json:"step,omitempty"` // 5m, 1h, etc.
}

type MetricDataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type MetricsQueryResponse struct {
	MetricName string            `json:"metric_name"`
	DataPoints []MetricDataPoint `json:"data_points"`
}

// Logs query structures
type LogsQueryRequest struct {
	ServiceName string            `json:"service_name,omitempty"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	Severity    string            `json:"severity,omitempty"`
	SearchText  string            `json:"search_text,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
	Filters     map[string]string `json:"filters,omitempty"`
	Limit       int               `json:"limit,omitempty"`
}

type LogRecord struct {
	Timestamp     time.Time         `json:"timestamp"`
	SeverityText  string            `json:"severity_text"`
	Body          string            `json:"body"`
	ServiceName   string            `json:"service_name"`
	TraceID       string            `json:"trace_id,omitempty"`
	SpanID        string            `json:"span_id,omitempty"`
	Attributes    map[string]string `json:"attributes"`
}

type LogsQueryResponse struct {
	Logs  []LogRecord `json:"logs"`
	Total int         `json:"total"`
}

// HTTP Handlers

// QueryTraces handles trace queries (Jaeger-compatible)
func (s *QueryService) QueryTraces(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		monitoring.QueryDuration.WithLabelValues("traces").Observe(time.Since(start).Seconds())
	}()

	var req TraceQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		monitoring.QueryErrors.WithLabelValues("traces").Inc()
		return
	}

	// Set defaults
	if req.Limit == 0 {
		req.Limit = 100
	}

	ctx := r.Context()
	query := `
		SELECT
			trace_id, span_id, parent_span_id, span_name, span_kind,
			start_time, end_time, duration_ns,
			status_code, status_message, service_name, attributes
		FROM otel_traces
		WHERE 1=1
	`
	args := []interface{}{}

	if req.TraceID != "" {
		query += " AND trace_id = ?"
		args = append(args, req.TraceID)
	}
	if req.ServiceName != "" {
		query += " AND service_name = ?"
		args = append(args, req.ServiceName)
	}
	if !req.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, req.StartTime)
	}
	if !req.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, req.EndTime)
	}
	if req.MinDuration > 0 {
		query += " AND duration_ns >= ?"
		args = append(args, req.MinDuration)
	}
	if req.MaxDuration > 0 {
		query += " AND duration_ns <= ?"
		args = append(args, req.MaxDuration)
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %d", req.Limit)

	rows, err := s.chClient.Query(ctx, query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		monitoring.QueryErrors.WithLabelValues("traces").Inc()
		return
	}
	defer rows.Close()

	spans := []Span{}
	for rows.Next() {
		var span Span
		var attrs map[string]string
		if err := rows.Scan(
			&span.TraceID, &span.SpanID, &span.ParentSpanID, &span.SpanName, &span.SpanKind,
			&span.StartTime, &span.EndTime, &span.DurationNs,
			&span.StatusCode, &span.StatusMessage, &span.ServiceName, &attrs,
		); err != nil {
			log.Printf("Error scanning span: %v", err)
			continue
		}
		span.Attributes = attrs
		spans = append(spans, span)
	}

	response := TraceQueryResponse{
		Spans: spans,
		Total: len(spans),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// QueryMetrics handles metrics queries (Prometheus-compatible)
func (s *QueryService) QueryMetrics(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		monitoring.QueryDuration.WithLabelValues("metrics").Observe(time.Since(start).Seconds())
	}()

	var req MetricsQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		monitoring.QueryErrors.WithLabelValues("metrics").Inc()
		return
	}

	// Default aggregation
	if req.Aggregation == "" {
		req.Aggregation = "avg"
	}

	// Determine which table to query based on time range
	tableName := "otel_metrics"
	if time.Since(req.StartTime) > 90*24*time.Hour {
		tableName = "otel_metrics_1h"
	} else if time.Since(req.StartTime) > 30*24*time.Hour {
		tableName = "otel_metrics_5m"
	}

	ctx := r.Context()
	aggFunc := req.Aggregation
	if tableName != "otel_metrics" {
		// Use pre-aggregated columns
		switch req.Aggregation {
		case "avg":
			aggFunc = "avg(value_avg)"
		case "min":
			aggFunc = "min(value_min)"
		case "max":
			aggFunc = "max(value_max)"
		case "sum":
			aggFunc = "sum(value_sum)"
		}
	} else {
		aggFunc = fmt.Sprintf("%s(value)", req.Aggregation)
	}

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL 5 MINUTE) as ts,
			%s as value
		FROM %s
		WHERE metric_name = ?
		  AND timestamp >= ?
		  AND timestamp <= ?
	`, aggFunc, tableName)

	args := []interface{}{req.MetricName, req.StartTime, req.EndTime}

	if req.ServiceName != "" {
		query += " AND service_name = ?"
		args = append(args, req.ServiceName)
	}

	query += " GROUP BY ts ORDER BY ts"

	rows, err := s.chClient.Query(ctx, query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		monitoring.QueryErrors.WithLabelValues("metrics").Inc()
		return
	}
	defer rows.Close()

	dataPoints := []MetricDataPoint{}
	for rows.Next() {
		var dp MetricDataPoint
		if err := rows.Scan(&dp.Timestamp, &dp.Value); err != nil {
			log.Printf("Error scanning metric: %v", err)
			continue
		}
		dataPoints = append(dataPoints, dp)
	}

	response := MetricsQueryResponse{
		MetricName: req.MetricName,
		DataPoints: dataPoints,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// QueryLogs handles log queries (Loki-compatible)
func (s *QueryService) QueryLogs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		monitoring.QueryDuration.WithLabelValues("logs").Observe(time.Since(start).Seconds())
	}()

	var req LogsQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		monitoring.QueryErrors.WithLabelValues("logs").Inc()
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	ctx := r.Context()
	query := `
		SELECT
			timestamp, severity_text, body, service_name,
			trace_id, span_id, attributes
		FROM otel_logs
		WHERE timestamp >= ?
		  AND timestamp <= ?
	`
	args := []interface{}{req.StartTime, req.EndTime}

	if req.ServiceName != "" {
		query += " AND service_name = ?"
		args = append(args, req.ServiceName)
	}
	if req.Severity != "" {
		query += " AND severity_text = ?"
		args = append(args, req.Severity)
	}
	if req.TraceID != "" {
		query += " AND trace_id = ?"
		args = append(args, req.TraceID)
	}
	if req.SearchText != "" {
		query += " AND body LIKE ?"
		args = append(args, "%"+req.SearchText+"%")
	}

	query += fmt.Sprintf(" ORDER BY timestamp DESC LIMIT %d", req.Limit)

	rows, err := s.chClient.Query(ctx, query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		monitoring.QueryErrors.WithLabelValues("logs").Inc()
		return
	}
	defer rows.Close()

	logs := []LogRecord{}
	for rows.Next() {
		var logRec LogRecord
		var attrs map[string]string
		if err := rows.Scan(
			&logRec.Timestamp, &logRec.SeverityText, &logRec.Body, &logRec.ServiceName,
			&logRec.TraceID, &logRec.SpanID, &attrs,
		); err != nil {
			log.Printf("Error scanning log: %v", err)
			continue
		}
		logRec.Attributes = attrs
		logs = append(logs, logRec)
	}

	response := LogsQueryResponse{
		Logs:  logs,
		Total: len(logs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetServiceStats returns service statistics
func (s *QueryService) GetServiceStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT
			service_name,
			count() as span_count,
			avg(duration_ns) as avg_duration,
			quantile(0.95)(duration_ns) as p95_duration,
			countIf(status_code = 'error') as error_count
		FROM otel_traces
		WHERE timestamp >= now() - INTERVAL 1 HOUR
		GROUP BY service_name
		ORDER BY span_count DESC
	`

	rows, err := s.chClient.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ServiceStat struct {
		ServiceName  string  `json:"service_name"`
		SpanCount    uint64  `json:"span_count"`
		AvgDuration  float64 `json:"avg_duration_ns"`
		P95Duration  float64 `json:"p95_duration_ns"`
		ErrorCount   uint64  `json:"error_count"`
	}

	stats := []ServiceStat{}
	for rows.Next() {
		var stat ServiceStat
		if err := rows.Scan(&stat.ServiceName, &stat.SpanCount, &stat.AvgDuration, &stat.P95Duration, &stat.ErrorCount); err != nil {
			log.Printf("Error scanning stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/query.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize monitoring
	shutdown, err := monitoring.InitTracing(serviceName, serviceVersion, cfg.Monitoring.TraceSampleRate)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer shutdown(context.Background())

	// Start metrics server
	metricsServer := monitoring.StartMetricsServer(cfg.Monitoring.MetricsPort, cfg.Monitoring.MetricsPath)
	defer metricsServer.Shutdown(context.Background())

	// Connect to ClickHouse
	chClient, err := clickhouse.NewClient(&cfg.ClickHouse)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chClient.Close()

	// Create query service
	queryService := NewQueryService(cfg, chClient)
	queryService.healthCheck.SetReady(true)

	// Setup HTTP router
	router := mux.NewRouter()
	router.HandleFunc("/api/v1/traces", queryService.QueryTraces).Methods("POST")
	router.HandleFunc("/api/v1/metrics", queryService.QueryMetrics).Methods("POST")
	router.HandleFunc("/api/v1/logs", queryService.QueryLogs).Methods("POST")
	router.HandleFunc("/api/v1/services/stats", queryService.GetServiceStats).Methods("GET")
	router.HandleFunc(cfg.Monitoring.HealthCheckPath, queryService.healthCheck.LivenessHandler).Methods("GET")
	router.HandleFunc(cfg.Monitoring.ReadyCheckPath, queryService.healthCheck.ReadinessHandler).Methods("GET")

	// Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("Query API server started on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gracefully...")
	queryService.healthCheck.SetReady(false)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}
