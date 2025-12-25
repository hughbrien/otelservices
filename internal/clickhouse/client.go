package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"otelservices/internal/config"
	"otelservices/internal/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps a ClickHouse connection
type Client struct {
	conn   driver.Conn
	config *config.ClickHouseConfig
}

// NewClient creates a new ClickHouse client
func NewClient(cfg *config.ClickHouseConfig) (*Client, error) {
	opts := &clickhouse.Options{
		Addr: cfg.Addresses,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:      cfg.DialTimeout,
		MaxOpenConns:     cfg.MaxOpenConns,
		MaxIdleConns:     cfg.MaxIdleConns,
		ConnMaxLifetime:  cfg.ConnMaxLifetime,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionZSTD,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
	}

	// Only configure TLS if explicitly needed
	// For local Docker deployments without TLS, leave it nil
	if cfg.TLSEnabled {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
		}
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return &Client{
		conn:   conn,
		config: cfg,
	}, nil
}

// Close closes the ClickHouse connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// InsertMetrics inserts a batch of metrics into ClickHouse
func (c *Client) InsertMetrics(ctx context.Context, metrics []models.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO otel_metrics (
			timestamp, metric_name, metric_type, value,
			service_name, service_namespace, service_instance_id, deployment_environment,
			attributes, resource_attributes,
			bucket_counts, explicit_bounds,
			instrumentation_scope_name, instrumentation_scope_version
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, m := range metrics {
		err := batch.Append(
			m.Timestamp,
			m.MetricName,
			m.MetricType,
			m.Value,
			m.ServiceName,
			m.ServiceNamespace,
			m.ServiceInstanceID,
			m.DeploymentEnvironment,
			m.Attributes,
			m.ResourceAttributes,
			m.BucketCounts,
			m.ExplicitBounds,
			m.InstrumentationScopeName,
			m.InstrumentationScopeVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to append metric: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

// InsertLogs inserts a batch of logs into ClickHouse
func (c *Client) InsertLogs(ctx context.Context, logs []models.LogRecord) error {
	if len(logs) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO otel_logs (
			timestamp, observed_timestamp, severity_number, severity_text,
			body, body_type,
			service_name, service_namespace, service_instance_id, deployment_environment, host_name,
			trace_id, span_id, trace_flags,
			attributes, resource_attributes,
			instrumentation_scope_name, instrumentation_scope_version
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, l := range logs {
		err := batch.Append(
			l.Timestamp,
			l.ObservedTimestamp,
			l.SeverityNumber,
			l.SeverityText,
			l.Body,
			l.BodyType,
			l.ServiceName,
			l.ServiceNamespace,
			l.ServiceInstanceID,
			l.DeploymentEnvironment,
			l.HostName,
			l.TraceID,
			l.SpanID,
			l.TraceFlags,
			l.Attributes,
			l.ResourceAttributes,
			l.InstrumentationScopeName,
			l.InstrumentationScopeVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to append log: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

// InsertSpans inserts a batch of spans into ClickHouse
func (c *Client) InsertSpans(ctx context.Context, spans []models.Span) error {
	if len(spans) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO otel_traces (
			timestamp, trace_id, span_id, parent_span_id,
			span_name, span_kind, start_time, end_time, duration_ns,
			status_code, status_message,
			service_name, service_namespace, service_instance_id, deployment_environment,
			attributes, resource_attributes,
			events, links,
			instrumentation_scope_name, instrumentation_scope_version
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, s := range spans {
		// Convert events to ClickHouse tuples
		events := make([]interface{}, len(s.Events))
		for i, e := range s.Events {
			events[i] = []interface{}{e.Timestamp, e.Name, e.Attributes}
		}

		// Convert links to ClickHouse tuples
		links := make([]interface{}, len(s.Links))
		for i, l := range s.Links {
			links[i] = []interface{}{l.TraceID, l.SpanID, l.TraceState, l.Attributes}
		}

		err := batch.Append(
			s.Timestamp,
			s.TraceID,
			s.SpanID,
			s.ParentSpanID,
			s.SpanName,
			s.SpanKind,
			s.StartTime,
			s.EndTime,
			s.DurationNs,
			s.StatusCode,
			s.StatusMessage,
			s.ServiceName,
			s.ServiceNamespace,
			s.ServiceInstanceID,
			s.DeploymentEnvironment,
			s.Attributes,
			s.ResourceAttributes,
			events,
			links,
			s.InstrumentationScopeName,
			s.InstrumentationScopeVersion,
		)
		if err != nil {
			return fmt.Errorf("failed to append span: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

// Ping checks the connection to ClickHouse
func (c *Client) Ping(ctx context.Context) error {
	return c.conn.Ping(ctx)
}

// Query executes a query and returns rows
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return c.conn.Query(ctx, query, args...)
}

// QueryRow executes a query that returns a single row
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return c.conn.QueryRow(ctx, query, args...)
}
