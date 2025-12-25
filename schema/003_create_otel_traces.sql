-- OpenTelemetry Traces Table
-- Optimized for high cardinality with hourly partitions

CREATE TABLE IF NOT EXISTS otel_traces (
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    trace_id String CODEC(ZSTD(3)),
    span_id String CODEC(ZSTD(3)),
    parent_span_id String CODEC(ZSTD(3)),

    -- Span details
    span_name LowCardinality(String) CODEC(ZSTD(3)),
    span_kind Enum8('internal' = 1, 'server' = 2, 'client' = 3, 'producer' = 4, 'consumer' = 5) CODEC(ZSTD(3)),
    start_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    end_time DateTime64(9) CODEC(Delta, ZSTD(3)),
    duration_ns UInt64 CODEC(ZSTD(3)),

    -- Status
    status_code Enum8('unset' = 0, 'ok' = 1, 'error' = 2) CODEC(ZSTD(3)),
    status_message String CODEC(ZSTD(3)),

    -- Resource attributes
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    service_namespace LowCardinality(String) CODEC(ZSTD(3)),
    service_instance_id String CODEC(ZSTD(3)),
    deployment_environment LowCardinality(String) CODEC(ZSTD(3)),

    -- Attributes
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Events
    events Array(Tuple(
        timestamp DateTime64(9),
        name String,
        attributes Map(String, String)
    )) CODEC(ZSTD(3)),

    -- Links
    links Array(Tuple(
        trace_id String,
        span_id String,
        trace_state String,
        attributes Map(String, String)
    )) CODEC(ZSTD(3)),

    -- Metadata
    instrumentation_scope_name LowCardinality(String) CODEC(ZSTD(3)),
    instrumentation_scope_version String CODEC(ZSTD(3)),

    INDEX idx_trace_id trace_id TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_service_name service_name TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_span_name span_name TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_status status_code TYPE set(0) GRANULARITY 4
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (trace_id, span_id, timestamp)
TTL toDateTime(timestamp) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Trace index table for quick lookups
CREATE TABLE IF NOT EXISTS otel_trace_index (
    trace_id String CODEC(ZSTD(3)),
    min_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    max_timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    service_names Array(String) CODEC(ZSTD(3)),
    root_service_name LowCardinality(String) CODEC(ZSTD(3)),
    root_span_name LowCardinality(String) CODEC(ZSTD(3)),
    duration_ns UInt64 CODEC(ZSTD(3)),
    span_count UInt32 CODEC(ZSTD(3)),
    has_errors UInt8 CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(min_timestamp)
ORDER BY (trace_id, min_timestamp)
TTL toDateTime(min_timestamp) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Materialized view for trace index
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_trace_index_mv
TO otel_trace_index
AS SELECT
    trace_id,
    min(start_time) AS min_timestamp,
    max(end_time) AS max_timestamp,
    groupUniqArray(service_name) AS service_names,
    anyIf(service_name, parent_span_id = '') AS root_service_name,
    anyIf(span_name, parent_span_id = '') AS root_span_name,
    max(duration_ns) AS duration_ns,
    count() AS span_count,
    countIf(status_code = 'error') > 0 AS has_errors
FROM otel_traces
GROUP BY trace_id;

-- Service dependency table (hourly aggregation)
CREATE TABLE IF NOT EXISTS otel_service_dependencies_1h (
    timestamp DateTime CODEC(Delta, ZSTD(3)),
    parent_service LowCardinality(String) CODEC(ZSTD(3)),
    child_service LowCardinality(String) CODEC(ZSTD(3)),
    call_count UInt64 CODEC(ZSTD(3)),
    error_count UInt64 CODEC(ZSTD(3)),
    avg_duration_ns Float64 CODEC(ZSTD(3)),
    p95_duration_ns Float64 CODEC(ZSTD(3)),
    p99_duration_ns Float64 CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, parent_service, child_service)
TTL timestamp + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

-- Span statistics table (hourly aggregation by service and span name)
CREATE TABLE IF NOT EXISTS otel_span_stats_1h (
    timestamp DateTime CODEC(Delta, ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    span_name LowCardinality(String) CODEC(ZSTD(3)),
    span_kind Enum8('internal' = 1, 'server' = 2, 'client' = 3, 'producer' = 4, 'consumer' = 5) CODEC(ZSTD(3)),
    call_count UInt64 CODEC(ZSTD(3)),
    error_count UInt64 CODEC(ZSTD(3)),
    avg_duration_ns Float64 CODEC(ZSTD(3)),
    min_duration_ns UInt64 CODEC(ZSTD(3)),
    max_duration_ns UInt64 CODEC(ZSTD(3)),
    p50_duration_ns Float64 CODEC(ZSTD(3)),
    p95_duration_ns Float64 CODEC(ZSTD(3)),
    p99_duration_ns Float64 CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, service_name, span_name)
TTL timestamp + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

-- Materialized view for span statistics
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_span_stats_1h_mv
TO otel_span_stats_1h
AS SELECT
    toStartOfHour(timestamp) AS timestamp,
    service_name,
    span_name,
    span_kind,
    count() AS call_count,
    countIf(status_code = 'error') AS error_count,
    avg(duration_ns) AS avg_duration_ns,
    min(duration_ns) AS min_duration_ns,
    max(duration_ns) AS max_duration_ns,
    quantile(0.5)(duration_ns) AS p50_duration_ns,
    quantile(0.95)(duration_ns) AS p95_duration_ns,
    quantile(0.99)(duration_ns) AS p99_duration_ns
FROM otel_traces
GROUP BY timestamp, service_name, span_name, span_kind;