-- OpenTelemetry Logs Table
-- Optimized for 1-year retention with daily partitions

CREATE TABLE IF NOT EXISTS otel_logs (
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    observed_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),

    -- Log severity
    severity_number UInt8 CODEC(ZSTD(3)),
    severity_text LowCardinality(String) CODEC(ZSTD(3)),

    -- Log body
    body String CODEC(ZSTD(3)),
    body_type Enum8('string' = 1, 'json' = 2, 'bytes' = 3) DEFAULT 'string' CODEC(ZSTD(3)),

    -- Resource attributes
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    service_namespace LowCardinality(String) CODEC(ZSTD(3)),
    service_instance_id String CODEC(ZSTD(3)),
    deployment_environment LowCardinality(String) CODEC(ZSTD(3)),
    host_name LowCardinality(String) CODEC(ZSTD(3)),

    -- Trace context (for correlation)
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    trace_flags UInt8 CODEC(ZSTD(3)),

    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Metadata
    instrumentation_scope_name LowCardinality(String) CODEC(ZSTD(3)),
    instrumentation_scope_version String CODEC(ZSTD(3)),

    INDEX idx_service_name service_name TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_severity severity_text TYPE set(0) GRANULARITY 4,
    INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_body body TYPE tokenbf_v1(30720, 3, 0) GRANULARITY 4
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, severity_number, service_name)
TTL toDateTime(timestamp) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Aggregated logs table for error tracking
CREATE TABLE IF NOT EXISTS otel_logs_errors_1h (
    timestamp DateTime CODEC(Delta, ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    severity_text LowCardinality(String) CODEC(ZSTD(3)),
    error_count UInt64 CODEC(ZSTD(3)),
    sample_messages Array(String) CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, service_name, severity_text)
TTL timestamp + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

-- Materialized view for hourly error aggregation
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_logs_errors_1h_mv
TO otel_logs_errors_1h
AS SELECT
    toStartOfHour(timestamp) AS timestamp,
    service_name,
    severity_text,
    count() AS error_count,
    groupArray(10)(body) AS sample_messages
FROM otel_logs
WHERE severity_number >= 17  -- ERROR and above
GROUP BY timestamp, service_name, severity_text;
