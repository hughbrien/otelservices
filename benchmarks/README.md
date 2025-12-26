# Load Testing

Load test tool for performance testing the OTLP collector.

## Quick Start

```bash
# Build
go build -o load_test .

# Or from project root
make build-loadtest

# Run test
./load_test -rate 100000 -duration 5m -workers 20
```

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `-endpoint` | `localhost:4317` | OTLP gRPC endpoint |
| `-duration` | `60s` | Test duration |
| `-rate` | `10000` | Target spans/sec |
| `-workers` | `10` | Concurrent workers |
| `-batch` | `100` | Spans per batch |

## Examples

**High volume test (100K spans/sec):**
```bash
./load_test -rate 100000 -duration 5m -workers 20
```

**Small batches:**
```bash
./load_test -rate 50000 -batch 50 -workers 10
```

**Remote endpoint:**
```bash
./load_test -endpoint otel-collector.example.com:4317 -rate 100000
```

**Quick baseline:**
```bash
./load_test -rate 10000 -duration 1m
```

## Monitoring

**Prometheus Metrics (http://localhost:9090):**
- `otel_received_spans_total`
- `otel_storage_writes_total`
- `otel_storage_write_duration_seconds`

**ClickHouse Queries:**
```sql
-- Ingestion rate
SELECT toStartOfMinute(timestamp) AS minute, count() AS spans_per_minute
FROM otel_traces
WHERE timestamp > now() - INTERVAL 10 MINUTE
GROUP BY minute ORDER BY minute DESC;

-- Storage efficiency
SELECT table, formatReadableSize(sum(bytes)) AS size, sum(rows) AS rows
FROM system.parts
WHERE database = 'otel' AND active
GROUP BY table;

-- Compression ratio
SELECT table,
  round(sum(data_uncompressed_bytes) / sum(data_compressed_bytes), 2) AS ratio
FROM system.parts
WHERE database = 'otel' AND active
GROUP BY table;
```

## Performance Targets

- **Ingestion:** 100K+ spans/sec
- **Query Latency:** p95 < 500ms (24h queries)
- **Storage:** <1TB per 1B spans
- **Memory:** <4GB per collector
- **CPU:** <50% at 50K spans/sec
