# Performance Benchmarking Suite

## Load Testing Tool

The load test tool generates synthetic OTLP trace data and sends it to the collector to measure performance.

### Building

```bash
cd benchmarks
go build -o load_test load_test.go
```

### Usage

```bash
./load_test [flags]
```

#### Flags

- `-endpoint`: OTLP gRPC endpoint (default: `localhost:4317`)
- `-duration`: Test duration (default: `60s`)
- `-rate`: Target spans per second (default: `10000`)
- `-workers`: Number of concurrent workers (default: `10`)
- `-batch`: Spans per batch (default: `100`)

### Examples

**Test at 100,000 spans/sec for 5 minutes:**
```bash
./load_test -rate 100000 -duration 5m -workers 20
```

**Test with small batches:**
```bash
./load_test -rate 50000 -batch 50 -workers 10
```

**Test against remote endpoint:**
```bash
./load_test -endpoint otel-collector.example.com:4317 -rate 100000
```

## Performance Targets

Based on the specification, the system should achieve:

- **Ingestion:** 100,000+ spans/sec per collector instance
- **Query Latency:** p95 < 500ms for 24-hour queries
- **Storage Efficiency:** <1TB for 1 billion spans
- **Memory Usage:** <4GB per collector instance
- **CPU Usage:** <50% utilization at 50K spans/sec

## Monitoring During Tests

1. **Prometheus Metrics** (http://localhost:9090):
   - `otel_received_spans_total`
   - `otel_storage_writes_total`
   - `otel_storage_write_duration_seconds`
   - `otel_batch_size`
   - `otel_memory_usage_bytes`

2. **ClickHouse Performance**:
```sql
-- Check ingestion rate
SELECT
    toStartOfMinute(timestamp) AS minute,
    count() AS spans_per_minute
FROM otel_traces
WHERE timestamp > now() - INTERVAL 10 MINUTE
GROUP BY minute
ORDER BY minute DESC;

-- Check storage size
SELECT
    table,
    formatReadableSize(sum(bytes)) AS size,
    sum(rows) AS rows,
    formatReadableSize(sum(bytes) / sum(rows)) AS avg_row_size
FROM system.parts
WHERE database = 'otel' AND active
GROUP BY table;

-- Check compression ratio
SELECT
    table,
    formatReadableSize(sum(data_compressed_bytes)) AS compressed,
    formatReadableSize(sum(data_uncompressed_bytes)) AS uncompressed,
    round(sum(data_uncompressed_bytes) / sum(data_compressed_bytes), 2) AS ratio
FROM system.parts
WHERE database = 'otel' AND active
GROUP BY table;
```

## Benchmarking Best Practices

1. **Warm-up Period**: Run tests for at least 30 seconds to stabilize
2. **Monitor Resources**: Watch CPU, memory, and disk I/O during tests
3. **Vary Batch Sizes**: Test with different batch sizes to find optimal configuration
4. **Test Failure Scenarios**: Simulate ClickHouse downtime and measure backpressure handling
5. **Long-running Tests**: Run 24-hour tests to validate retention and TTL policies
6. **Query Performance**: Test queries while ingesting at high rates

## Sample Test Plan

1. **Baseline Test**: 10K spans/sec for 10 minutes
2. **Target Load**: 100K spans/sec for 1 hour
3. **Peak Load**: 200K spans/sec for 10 minutes
4. **Sustained Load**: 100K spans/sec for 24 hours
5. **Burst Test**: Alternate between 10K and 200K spans/sec every 5 minutes
