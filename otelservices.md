# OpenTelemetry Services Implementation Prompt

## Project Requirements

Build a high-performance OpenTelemetry data pipeline in Go that collects, processes, and stores metrics, logs, and traces with 1-year retention optimization.

### Technical Specifications

**Language & Platform:**
- Go 1.21+ for cross-platform compatibility (Linux, macOS, Windows)
- Single binary distribution with no external runtime dependencies
- Containerized deployment option (Docker/K8s)

**Data Storage:**
- **Primary Database: ClickHouse** (optimal for OTEL data due to column-oriented storage, excellent compression ratios ~10:1, and sub-second query performance on year-long datasets)
- Alternative consideration: TimescaleDB if PostgreSQL ecosystem is required

**Architecture Components:**

1. **OTEL Collector Service** (port 4317/4318)
    - Receives OTLP gRPC and HTTP traffic
    - Implements batching, retry logic, and backpressure handling
    - Processors: batch, memory_limiter, resource detection
    - Exporters: ClickHouse with connection pooling

2. **Storage Service**
    - Three optimized ClickHouse tables with appropriate partitioning:
        - `otel_metrics` - partitioned by day, ordered by (timestamp, metric_name, attributes)
        - `otel_logs` - partitioned by day, ordered by (timestamp, severity, service_name)
        - `otel_traces` - partitioned by hour, ordered by (trace_id, span_id, timestamp)

3. **Query Service** (REST API)
    - Metrics: Prometheus-compatible query API
    - Logs: Loki-compatible query API
    - Traces: Jaeger-compatible query API
    - Custom aggregation endpoints with caching layer

### Performance Optimization Requirements

**1-Year Data Retention Strategy:**

- **TTL Policies:** Implement time-to-live with automatic data cleanup
    - Raw data: 30 days
    - 5-minute rollups: 90 days
    - 1-hour rollups: 1 year

- **Compression:**
    - Use ClickHouse ZSTD compression (level 3)
    - Expected 8-10x compression ratio
    - Projection of ~500GB for 1 billion spans/year

- **Partitioning:**
    - Metrics: Daily partitions with PARTITION BY toYYYYMMDD(timestamp)
    - Logs: Daily partitions
    - Traces: Hourly partitions (high cardinality)

- **Indexing Strategy:**
    - Primary keys on timestamp + high-cardinality fields
    - Skip indexes on service.name, trace_id
    - Bloom filter indexes on low-cardinality attributes

### Implementation Details

**Required Go Packages:**
```
- go.opentelemetry.io/collector
- github.com/ClickHouse/clickhouse-go/v2
- go.opentelemetry.io/otel (instrumentation)
- github.com/prometheus/client_golang (metrics)
```

**Key Features:**

1. **Collector Pipeline:**
    - Batch processor: 10,000 spans or 10 seconds
    - Memory limiter: 80% soft, 90% hard limit
    - Resource detection: cloud, container, host metadata
    - Queue retry: exponential backoff, max 5 retries

2. **Data Schema:**
    - Use OTEL semantic conventions strictly
    - Flatten nested attributes into Map(String, String) columns
    - Separate resource attributes from span/log attributes
    - Denormalize for query performance

3. **Query Optimization:**
    - Materialized views for common aggregations
    - Query result caching (15-minute TTL)
    - Connection pooling (min 5, max 50 connections)
    - Parallel query execution for time-range splits

4. **Monitoring & Observability:**
    - Self-instrumentation with OTEL Go SDK
    - Expose Prometheus metrics endpoint (:9090/metrics)
    - Health checks: /health (liveness), /ready (readiness)
    - Structured logging with configurable levels

5. **Configuration:**
    - YAML-based configuration files
    - Environment variable overrides
    - Hot-reload capability for non-breaking changes
    - Separate configs for dev/staging/prod

### Performance Targets

- **Ingestion:** 100,000+ spans/sec per instance
- **Query Latency:** p95 < 500ms for 24-hour queries
- **Storage Efficiency:** <1TB for 1 billion spans
- **Memory Usage:** <4GB per collector instance
- **CPU Usage:** <50% utilization at 50K spans/sec

### Deliverables

1. Three microservices: collector, storage-writer, query-api
2. ClickHouse schema migration scripts with rollup tables
3. Docker Compose setup for local development
4. Kubernetes manifests for production deployment
5. Performance benchmarking suite
6. Grafana dashboards for service monitoring
7. Documentation: architecture, deployment, tuning guide

### Bonus Optimizations

- Implement adaptive sampling for high-volume traces
- Add Redis caching layer for hot queries
- Support S3/GCS cold storage for archival (90+ days)
- Implement data retention policies with configurable TTL
- Add trace exemplars linking to metrics
- Support OpenTelemetry Metrics SDK for custom metrics

---
Generate the actual Go code for any of these services
Create the ClickHouse schema DDL statements
Design the API specifications for the query service
Build a performance testing framework for this