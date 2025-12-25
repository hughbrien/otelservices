# Performance Tuning Guide

## Overview

This guide covers optimization strategies to achieve the target performance metrics:
- **Ingestion:** 100,000+ spans/sec per collector instance
- **Query Latency:** p95 < 500ms for 24-hour queries
- **Storage Efficiency:** <1TB for 1 billion spans
- **Memory Usage:** <4GB per collector instance
- **CPU Usage:** <50% utilization at 50K spans/sec

## Collector Tuning

### Batch Processing

**Impact:** Directly affects throughput and latency

```yaml
performance:
  batch_size: 10000          # Number of items before flush
  batch_timeout: 10s         # Max time before flush
  max_batch_bytes: 104857600 # 100MB max batch size
```

**Tuning Guidelines:**
- **Higher throughput:** Increase `batch_size` to 20000-50000
- **Lower latency:** Decrease `batch_timeout` to 5s
- **Memory constrained:** Decrease `batch_size` or `max_batch_bytes`
- **Network limited:** Increase compression but may use more CPU

**Trade-offs:**
- Larger batches = higher throughput but more memory
- Smaller batches = lower latency but more overhead

### Worker Configuration

```yaml
performance:
  worker_count: 4      # Concurrent writers
  queue_size: 100000   # Buffer size
```

**Tuning Guidelines:**
- **CPU cores available:** Set `worker_count` = number of cores - 1
- **High burst traffic:** Increase `queue_size` to 500000+
- **Memory pressure:** Decrease `queue_size`

**Monitoring:**
```promql
# Check queue utilization
otel_queue_size / 100000

# If consistently > 80%, increase queue_size or worker_count
```

### Memory Management

```yaml
performance:
  memory_limit_mib: 3200      # Soft limit (80%)
  memory_spike_limit: 800     # Spike limit (20%)
```

**Calculation:**
```
Total Memory = memory_limit_mib + memory_spike_limit
Example: 3200 + 800 = 4000 MiB = ~4GB
```

**Tuning Guidelines:**
- Set based on container/pod memory limits
- Leave 20% headroom for spikes
- Monitor actual usage and adjust

**Monitoring:**
```promql
otel_memory_usage_bytes / (4 * 1024 * 1024 * 1024)  # As fraction of 4GB
```

### Retry Configuration

```yaml
performance:
  retry_max_attempts: 5
  retry_initial_interval: 1s
  retry_max_interval: 30s
```

**Tuning Guidelines:**
- **Stable network:** Reduce `retry_max_attempts` to 3
- **Unstable network:** Increase to 10
- **Fast failure:** Reduce `retry_max_interval` to 10s

## ClickHouse Tuning

### Table Optimization

#### Partitioning Strategy

**Current Setup:**
- Metrics: Daily partitions
- Logs: Daily partitions
- Traces: Hourly partitions (high cardinality)

**Why hourly for traces?**
- Traces have highest volume
- Faster partition pruning on queries
- Better parallelization

**Tuning:**
```sql
-- For very high volume (>1M spans/sec), consider 15-minute partitions
ALTER TABLE otel_traces MODIFY PARTITION BY toYYYYMMDD(timestamp), toHour(timestamp), toMinute(timestamp) / 15
```

#### Ordering Key Optimization

**Current ordering:**
```sql
ORDER BY (trace_id, span_id, timestamp)  -- For trace lookups
ORDER BY (timestamp, metric_name, service_name)  -- For time-series queries
```

**Tuning Guidelines:**
- Put most selective column first for your query patterns
- For trace-centric queries: `(trace_id, timestamp, span_id)`
- For time-series queries: `(timestamp, service_name, metric_name)`

**Test query performance:**
```sql
-- Enable query profiling
SET send_logs_level = 'trace';

-- Run query and check "rows read"
SELECT count() FROM otel_traces WHERE trace_id = '...'
```

Lower "rows read" = better ordering key.

#### Compression Tuning

**Current: ZSTD(3)** - Good balance of compression ratio and CPU

**Options:**
```sql
-- Maximum compression (slower writes, faster queries)
CODEC(ZSTD(9))

-- Faster writes (lower compression)
CODEC(ZSTD(1))

-- Delta encoding for timestamps
CODEC(Delta, ZSTD(3))

-- LZ4 for less compressible data
CODEC(LZ4)
```

**Recommended per column type:**
- Timestamps: `Delta, ZSTD(3)`
- IDs: `ZSTD(3)`
- Text: `ZSTD(5)`
- Numbers: `Delta, ZSTD(1)`

### Index Optimization

#### Skip Indexes

**When to add:**
- Filtering on non-primary-key columns
- High cardinality fields
- Frequent WHERE clauses

**Example:**
```sql
-- Add bloom filter for error status
ALTER TABLE otel_traces ADD INDEX idx_status status_code TYPE bloom_filter(0.01) GRANULARITY 4;

-- Materialize the index
ALTER TABLE otel_traces MATERIALIZE INDEX idx_status;
```

**Index types:**
- `bloom_filter`: For equality checks (service_name = 'x')
- `set`: For low cardinality (<1000 values)
- `tokenbf_v1`: For full-text search
- `minmax`: For range queries on sorted data

#### Monitoring Index Usage

```sql
SELECT
    table,
    name,
    type,
    formatReadableSize(bytes_on_disk) AS size
FROM system.data_skipping_indices
WHERE database = 'otel';
```

### Query Optimization

#### Use Materialized Views

**Pre-aggregate common queries:**
```sql
-- P95 latency by service (already created)
CREATE MATERIALIZED VIEW otel_span_stats_1h_mv ...

-- Custom: Request rate by endpoint
CREATE MATERIALIZED VIEW http_request_rate_1m
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMMDD(timestamp)
ORDER BY (timestamp, service_name, http_route)
AS SELECT
    toStartOfMinute(timestamp) AS timestamp,
    service_name,
    attributes['http.route'] AS http_route,
    count() AS request_count
FROM otel_traces
WHERE span_kind = 'server'
GROUP BY timestamp, service_name, http_route;
```

#### Query Sampling

For exploratory queries on large datasets:
```sql
-- Sample 10% of data
SELECT * FROM otel_traces SAMPLE 0.1
WHERE timestamp > now() - INTERVAL 1 DAY
```

#### Parallel Query Execution

```sql
-- Set max threads per query
SET max_threads = 8;

-- Enable parallel reading
SET max_parallel_replicas = 2;
```

### Storage Optimization

#### TTL Management

**Monitor partition sizes:**
```sql
SELECT
    partition,
    formatReadableSize(sum(bytes)) AS size,
    sum(rows) AS rows,
    max(modification_time) AS last_modified
FROM system.parts
WHERE database = 'otel' AND table = 'otel_traces'
GROUP BY partition
ORDER BY partition DESC
LIMIT 20;
```

**Verify TTL is working:**
```sql
SELECT
    table,
    delete_ttl_info_min,
    delete_ttl_info_max
FROM system.parts
WHERE database = 'otel' AND active
ORDER BY modification_time DESC;
```

#### Disk Space Monitoring

```bash
# Total space per table
clickhouse-client --query "
SELECT
    table,
    formatReadableSize(sum(bytes)) AS size,
    formatReadableSize(sum(data_compressed_bytes)) AS compressed,
    formatReadableSize(sum(data_uncompressed_bytes)) AS uncompressed,
    round(sum(data_uncompressed_bytes) / sum(data_compressed_bytes), 2) AS ratio
FROM system.parts
WHERE database = 'otel' AND active
GROUP BY table
"
```

**Target compression ratios:**
- Traces: 8-10x
- Logs: 6-8x
- Metrics: 10-15x

#### Optimize Old Partitions

```sql
-- Compact old partitions (frees disk space)
OPTIMIZE TABLE otel_traces PARTITION '20240101' FINAL;

-- Schedule automatic optimization
ALTER TABLE otel_traces
    MODIFY SETTING min_bytes_for_wide_part = 10485760,  -- 10MB
                   min_rows_for_wide_part = 100000;
```

## Query Service Tuning

### Connection Pooling

```yaml
clickhouse:
  max_open_conns: 50    # Maximum connections
  max_idle_conns: 5     # Idle connections to keep
  conn_max_lifetime: 1h # Recycle connections
```

**Tuning Guidelines:**
- **High query volume:** Increase `max_open_conns` to 100
- **Low query volume:** Decrease to 20 to save resources
- **Connection errors:** Increase `max_idle_conns` to 10

### Caching

```yaml
performance:
  cache_ttl: 15m
```

**Implementation (add to query service):**
```go
import "github.com/patrickmn/go-cache"

// Create cache
c := cache.New(15*time.Minute, 30*time.Minute)

// Cache query results
func (s *QueryService) QueryWithCache(key string, query func() (interface{}, error)) (interface{}, error) {
    if cached, found := c.Get(key); found {
        return cached, nil
    }

    result, err := query()
    if err == nil {
        c.Set(key, result, cache.DefaultExpiration)
    }
    return result, err
}
```

### Query Timeouts

```go
ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
defer cancel()
```

**Tuning Guidelines:**
- Long-range queries: 60s timeout
- Real-time queries: 5s timeout
- Aggregations: 30s timeout

## System-Level Tuning

### Linux Kernel

```bash
# Increase file descriptors
ulimit -n 65536

# TCP tuning for high throughput
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"
sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"

# Connection tracking for high connection count
sysctl -w net.netfilter.nf_conntrack_max=1048576
```

### ClickHouse Server Settings

Edit `/etc/clickhouse-server/config.xml`:

```xml
<clickhouse>
    <!-- Increase max connections -->
    <max_connections>1000</max_connections>

    <!-- Memory limits -->
    <max_memory_usage>10000000000</max_memory_usage>  <!-- 10GB -->
    <max_bytes_before_external_group_by>5000000000</max_bytes_before_external_group_by>

    <!-- Thread pools -->
    <max_thread_pool_size>10000</max_thread_pool_size>
    <max_thread_pool_free_size>1000</max_thread_pool_free_size>

    <!-- Query execution -->
    <max_execution_time>300</max_execution_time>  <!-- 5 minutes -->

    <!-- Compression -->
    <compression>
        <case>
            <method>zstd</method>
            <level>3</level>
        </case>
    </compression>
</clickhouse>
```

## Performance Monitoring

### Key Metrics to Watch

```promql
# Ingestion rate
rate(otel_received_spans_total[5m])

# Write latency
histogram_quantile(0.95, rate(otel_storage_write_duration_seconds_bucket[5m]))

# Queue depth (should be < 50% capacity)
otel_queue_size / 100000

# Memory usage (should be < 80% limit)
otel_memory_usage_bytes / (4 * 1024 * 1024 * 1024)

# ClickHouse query duration
histogram_quantile(0.95, rate(otel_query_duration_seconds_bucket[5m]))
```

### Performance Testing

```bash
# Run load test
cd benchmarks
go build -o load_test load_test.go

# Test at target rate
./load_test -rate 100000 -duration 5m -workers 20

# Monitor during test
watch -n 5 'curl -s http://localhost:9090/metrics | grep otel_received_spans_total'
```

## Troubleshooting Performance Issues

### High CPU Usage

**Diagnosis:**
```bash
# Check CPU per pod
kubectl top pods -n otel-system

# Profile Go application
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Solutions:**
- Reduce batch size (less processing per batch)
- Increase worker count (more parallelism)
- Optimize ClickHouse queries
- Consider horizontal scaling

### High Memory Usage

**Diagnosis:**
```bash
# Memory profile
curl http://localhost:6060/debug/pprof/heap > mem.prof
go tool pprof mem.prof
```

**Solutions:**
- Reduce queue size
- Reduce batch size
- Enable memory profiling to find leaks
- Increase memory limits if sustainable

### Slow Writes to ClickHouse

**Diagnosis:**
```sql
-- Check for slow inserts
SELECT
    query,
    query_duration_ms,
    written_rows,
    written_bytes
FROM system.query_log
WHERE type = 'QueryFinish' AND query LIKE 'INSERT%'
ORDER BY query_start_time DESC
LIMIT 10;
```

**Solutions:**
- Increase batch size (more efficient writes)
- Check disk I/O performance
- Optimize table structure
- Add more ClickHouse nodes (if clustered)

### Slow Queries

**Diagnosis:**
```sql
-- Find slow queries
SELECT
    query,
    query_duration_ms,
    read_rows,
    read_bytes,
    memory_usage
FROM system.query_log
WHERE query_duration_ms > 1000
ORDER BY query_start_time DESC
LIMIT 10;
```

**Solutions:**
- Add skip indexes on filtered columns
- Use materialized views for aggregations
- Optimize partition pruning (check WHERE clauses)
- Increase ClickHouse memory limits
- Query smaller time ranges
