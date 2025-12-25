# OpenTelemetry Services Architecture

## Overview

This system implements a high-performance OpenTelemetry data pipeline designed to collect, process, store, and query metrics, logs, and traces with 1-year retention optimization.

## Architecture Diagram

```
┌─────────────────┐
│  Applications   │
│  (OTLP Export)  │
└────────┬────────┘
         │ OTLP/gRPC (4317)
         │ OTLP/HTTP (4318)
         ▼
┌─────────────────────────────────────┐
│      OTEL Collector Service         │
│                                     │
│  ├─ OTLP Receivers (gRPC/HTTP)     │
│  ├─ Batch Processor                │
│  ├─ Memory Limiter                 │
│  └─ Worker Pool (Async Write)      │
└────────┬────────────────────────────┘
         │
         │ Batched Inserts
         ▼
┌─────────────────────────────────────┐
│         ClickHouse Cluster          │
│                                     │
│  ├─ otel_metrics (30d + rollups)   │
│  ├─ otel_logs (30d)                │
│  ├─ otel_traces (30d)              │
│  └─ Materialized Views (1y)        │
└────────┬────────────────────────────┘
         │
         │ SQL Queries
         ▼
┌─────────────────────────────────────┐
│        Query API Service            │
│                                     │
│  ├─ /api/v1/traces (Jaeger-compat) │
│  ├─ /api/v1/metrics (Prom-compat)  │
│  ├─ /api/v1/logs (Loki-compat)     │
│  └─ /api/v1/services/stats         │
└─────────────────────────────────────┘
         │
         │ HTTP/REST
         ▼
┌─────────────────┐
│  Visualization  │
│  (Grafana, etc) │
└─────────────────┘
```

## Components

### 1. OTEL Collector Service

**Responsibilities:**
- Receive OTLP data over gRPC (port 4317) and HTTP (port 4318)
- Process and batch telemetry signals
- Handle backpressure and retry logic
- Write to ClickHouse with optimal batching

**Key Features:**
- Multi-protocol support (gRPC and HTTP/Protobuf)
- Configurable batch sizes (default: 10,000 items or 10 seconds)
- Memory limiting with soft/hard thresholds
- Worker pool for concurrent writes (default: 4 workers)
- Automatic retry with exponential backoff
- Self-instrumentation with Prometheus metrics

**Performance Characteristics:**
- Target: 100,000+ spans/sec per instance
- Memory: <4GB per instance
- CPU: <50% at 50K spans/sec

### 2. ClickHouse Storage Layer

**Schema Design:**

#### Metrics Table (`otel_metrics`)
- **Partitioning:** Daily (`PARTITION BY toYYYYMMDD(timestamp)`)
- **Ordering:** `(timestamp, metric_name, service_name)`
- **TTL:** 30 days (raw data)
- **Rollups:**
  - `otel_metrics_5m`: 5-minute aggregations (90-day retention)
  - `otel_metrics_1h`: 1-hour aggregations (1-year retention)

#### Logs Table (`otel_logs`)
- **Partitioning:** Daily
- **Ordering:** `(timestamp, severity_number, service_name)`
- **TTL:** 30 days
- **Indexes:**
  - Bloom filter on service_name, trace_id
  - Token bloom filter on log body (full-text search)

#### Traces Table (`otel_traces`)
- **Partitioning:** Hourly (`PARTITION BY toYYYYMMDDHH(timestamp)`)
- **Ordering:** `(trace_id, span_id, timestamp)`
- **TTL:** 30 days
- **Auxiliary Tables:**
  - `otel_trace_index`: Fast trace lookup
  - `otel_span_stats_1h`: Hourly span statistics (1-year retention)
  - `otel_service_dependencies_1h`: Service dependency graph

**Optimization Strategies:**
- ZSTD compression (level 3) with ~8-10x compression ratio
- Skip indexes for high-cardinality fields
- Materialized views for automatic aggregation
- Projection of ~500GB for 1 billion spans/year

### 3. Query API Service

**Endpoints:**

#### Traces API (Jaeger-compatible)
```
POST /api/v1/traces
{
  "trace_id": "...",
  "service_name": "...",
  "start_time": "...",
  "end_time": "...",
  "min_duration": 1000000,
  "max_duration": 5000000,
  "limit": 100
}
```

#### Metrics API (Prometheus-compatible)
```
POST /api/v1/metrics
{
  "metric_name": "http_request_duration",
  "service_name": "api-server",
  "start_time": "...",
  "end_time": "...",
  "aggregation": "avg",
  "step": "5m"
}
```

#### Logs API (Loki-compatible)
```
POST /api/v1/logs
{
  "service_name": "api-server",
  "start_time": "...",
  "end_time": "...",
  "severity": "ERROR",
  "search_text": "timeout",
  "limit": 100
}
```

#### Service Stats
```
GET /api/v1/services/stats
```
Returns aggregated statistics for all services.

**Performance Targets:**
- p95 query latency: <500ms for 24-hour queries
- Automatic table selection based on time range
- Query result caching (15-minute TTL)
- Connection pooling (5-50 connections)

## Data Flow

### Ingestion Path

1. **Application** sends OTLP data to Collector
2. **Collector** receives and validates data
3. **Batch Processor** accumulates data until:
   - Batch size reaches 10,000 items, OR
   - 10 seconds have elapsed
4. **Worker Pool** processes batches concurrently
5. **ClickHouse Client** writes batch with compression
6. **Materialized Views** automatically update rollup tables

### Query Path

1. **Client** sends query to Query API
2. **Query Service** determines optimal table:
   - Recent data (<30 days): Raw tables
   - Medium range (30-90 days): 5-minute rollups
   - Long range (>90 days): 1-hour rollups
3. **ClickHouse** executes query with:
   - Partition pruning
   - Skip index filtering
   - Parallel processing
4. **Results** returned to client (with optional caching)

## Scalability

### Horizontal Scaling

**Collector Service:**
- Stateless design allows unlimited horizontal scaling
- Load balance with standard L4/L7 load balancers
- Each instance handles 100K+ spans/sec
- Example: 10 instances = 1M+ spans/sec capacity

**Query Service:**
- Stateless design allows unlimited horizontal scaling
- Load balance for query throughput
- Connection pooling prevents ClickHouse overload

**ClickHouse:**
- Single-node for moderate scale (<100K spans/sec)
- Cluster with replication for high availability
- Distributed tables for >1M spans/sec

### Vertical Scaling

**Collector:**
- Increase worker count for CPU-bound workloads
- Increase memory for larger batch sizes
- Increase queue size for burst handling

**ClickHouse:**
- More RAM = larger cache = faster queries
- More CPU cores = more parallel query execution
- Faster disks = better write throughput

## High Availability

### Collector HA
- Deploy 3+ instances behind load balancer
- No shared state - any instance can fail
- Client retry on connection failure

### ClickHouse HA
- Deploy with replication (ReplicatedMergeTree)
- Use ClickHouse Keeper for coordination
- Automatic failover with ClickHouse cluster

### Query Service HA
- Deploy 2+ instances behind load balancer
- Share read-only access to ClickHouse
- No shared state between instances

## Monitoring

### Prometheus Metrics

**Collector Metrics:**
- `otel_received_spans_total`
- `otel_storage_writes_total{table,status}`
- `otel_storage_write_duration_seconds`
- `otel_batch_size`
- `otel_memory_usage_bytes`
- `otel_queue_size`

**Query Metrics:**
- `otel_query_duration_seconds{query_type}`
- `otel_query_errors_total{query_type}`

### Health Checks

- `/health` - Liveness probe (always returns 200)
- `/ready` - Readiness probe (checks dependencies)

### Grafana Dashboards

- **Collector Dashboard**: Ingestion rates, batch sizes, memory
- **Query Dashboard**: Query performance, error rates
- **ClickHouse Dashboard**: Storage stats, compression ratios

## Security Considerations

1. **Authentication:** Not implemented (add via API gateway)
2. **Encryption:** TLS for OTLP endpoints (configure via reverse proxy)
3. **Authorization:** Not implemented (add via API gateway)
4. **Network Security:** Deploy in private network, expose via gateway
5. **ClickHouse Security:** Use authentication and restrict network access

## Future Enhancements

1. **Adaptive Sampling:** Intelligently sample high-volume traces
2. **Redis Caching:** Cache hot queries for faster response
3. **Cold Storage:** Archive to S3/GCS after 90 days
4. **Trace Exemplars:** Link metrics to example traces
5. **Custom Metrics SDK:** Application-level custom metrics
6. **Multi-tenancy:** Isolate data by tenant/organization
