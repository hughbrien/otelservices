-- OpenTelemetry Metrics Table
-- Optimized for 1-year retention with automatic rollups and TTL

CREATE TABLE IF NOT EXISTS otel_metrics (
    timestamp DateTime64(9) CODEC(Delta, ZSTD(3)),
    metric_name LowCardinality(String) CODEC(ZSTD(3)),
    metric_type Enum8('gauge' = 1, 'counter' = 2, 'histogram' = 3, 'summary' = 4) CODEC(ZSTD(3)),
    value Float64 CODEC(ZSTD(3)),

    -- Resource attributes
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    service_namespace LowCardinality(String) CODEC(ZSTD(3)),
    service_instance_id String CODEC(ZSTD(3)),
    deployment_environment LowCardinality(String) CODEC(ZSTD(3)),

    -- Metric attributes (flattened)
    attributes Map(String, String) CODEC(ZSTD(3)),
    resource_attributes Map(String, String) CODEC(ZSTD(3)),

    -- Histogram-specific fields
    bucket_counts Array(UInt64) CODEC(ZSTD(3)),
    explicit_bounds Array(Float64) CODEC(ZSTD(3)),

    -- Metadata
    instrumentation_scope_name LowCardinality(String) CODEC(ZSTD(3)),
    instrumentation_scope_version String CODEC(ZSTD(3)),

    INDEX idx_service_name service_name TYPE bloom_filter(0.01) GRANULARITY 4,
    INDEX idx_metric_name metric_name TYPE bloom_filter(0.01) GRANULARITY 4
)
ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, metric_name, service_name)
TTL toDateTime(timestamp) + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- 5-minute rollup table
CREATE TABLE IF NOT EXISTS otel_metrics_5m (
    timestamp DateTime CODEC(Delta, ZSTD(3)),
    metric_name LowCardinality(String) CODEC(ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    metric_type Enum8('gauge' = 1, 'counter' = 2, 'histogram' = 3, 'summary' = 4) CODEC(ZSTD(3)),

    -- Aggregated values
    value_avg Float64 CODEC(ZSTD(3)),
    value_min Float64 CODEC(ZSTD(3)),
    value_max Float64 CODEC(ZSTD(3)),
    value_sum Float64 CODEC(ZSTD(3)),
    value_count UInt64 CODEC(ZSTD(3)),

    attributes Map(String, String) CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, metric_name, service_name)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- 1-hour rollup table
CREATE TABLE IF NOT EXISTS otel_metrics_1h (
    timestamp DateTime CODEC(Delta, ZSTD(3)),
    metric_name LowCardinality(String) CODEC(ZSTD(3)),
    service_name LowCardinality(String) CODEC(ZSTD(3)),
    metric_type Enum8('gauge' = 1, 'counter' = 2, 'histogram' = 3, 'summary' = 4) CODEC(ZSTD(3)),

    -- Aggregated values
    value_avg Float64 CODEC(ZSTD(3)),
    value_min Float64 CODEC(ZSTD(3)),
    value_max Float64 CODEC(ZSTD(3)),
    value_sum Float64 CODEC(ZSTD(3)),
    value_count UInt64 CODEC(ZSTD(3)),

    attributes Map(String, String) CODEC(ZSTD(3))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (timestamp, metric_name, service_name)
TTL timestamp + INTERVAL 1 YEAR
SETTINGS index_granularity = 8192;

-- Materialized view for 5-minute rollups
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_metrics_5m_mv
TO otel_metrics_5m
AS SELECT
    toStartOfFiveMinutes(timestamp) AS timestamp,
    metric_name,
    service_name,
    metric_type,
    avg(value) AS value_avg,
    min(value) AS value_min,
    max(value) AS value_max,
    sum(value) AS value_sum,
    count() AS value_count,
    attributes
FROM otel_metrics
GROUP BY timestamp, metric_name, service_name, metric_type, attributes;

-- Materialized view for 1-hour rollups
CREATE MATERIALIZED VIEW IF NOT EXISTS otel_metrics_1h_mv
TO otel_metrics_1h
AS SELECT
    toStartOfHour(timestamp) AS timestamp,
    metric_name,
    service_name,
    metric_type,
    avg(value) AS value_avg,
    min(value) AS value_min,
    max(value) AS value_max,
    sum(value) AS value_sum,
    count() AS value_count,
    attributes
FROM otel_metrics
GROUP BY timestamp, metric_name, service_name, metric_type, attributes;