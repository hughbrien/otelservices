# OpenTelemetry Services

High-performance OpenTelemetry data pipeline for collecting, storing, and querying metrics, logs, and traces with long-term retention.

## Features

- 100K+ spans/sec ingestion per instance
- ClickHouse storage with ~10:1 compression
- 1-year retention with automatic rollups
- Jaeger, Prometheus, and Loki compatible APIs
- Docker Compose and Kubernetes ready

## Architecture

```
Apps → Collector (4317/4318) → ClickHouse → Query API (8081) → Grafana (3000)
```

## Quick Start

### Deploy with Docker Compose

```bash
cd deployments/docker
docker-compose up -d

# Initialize schema
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/001_create_otel_metrics.sql
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/002_create_otel_logs.sql
docker exec -i otel-clickhouse clickhouse-client --multiquery < ../../schema/003_create_otel_traces.sql

# Verify
curl http://localhost:8080/health  # Collector
curl http://localhost:8081/health  # Query API
```

**Access Points:**
- Collector: `localhost:4317` (gRPC), `localhost:4318` (HTTP)
- Query API: `localhost:8081`
- Grafana: `localhost:3000` (admin/admin)
- Prometheus: `localhost:9092`
- ClickHouse: `localhost:9000` (native), `localhost:8123` (HTTP)

### Send Data

**Using OpenTelemetry SDK (Go):**
```go
import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

exporter, _ := otlptracegrpc.New(
    context.Background(),
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)
```

**Using HTTP/JSON:**
```bash
curl -X POST http://localhost:4318/v1/traces -H "Content-Type: application/json" -d '{
  "resourceSpans": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "my-service"}}]},
    "scopeSpans": [{"spans": [{
      "traceId": "5b8aa5a2d2c872e8321cf37308d69df2",
      "spanId": "051581bf3cb55c13",
      "name": "my-operation",
      "startTimeUnixNano": "'$(date +%s)000000000'",
      "endTimeUnixNano": "'$(date +%s)000000000'"
    }]}]
  }]
}'
```

### Query Data

**Query Traces:**
```bash
curl -X POST http://localhost:8081/api/v1/traces -H "Content-Type: application/json" -d '{
  "service_name": "my-service",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-01T23:59:59Z"
}'
```

**Query Metrics:**
```bash
curl -X POST http://localhost:8081/api/v1/metrics -H "Content-Type: application/json" -d '{
  "metric_name": "http_request_duration",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-01T23:59:59Z",
  "aggregation": "avg"
}'
```

**Query Logs:**
```bash
curl -X POST http://localhost:8081/api/v1/logs -H "Content-Type: application/json" -d '{
  "service_name": "my-service",
  "severity": "ERROR",
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-01T23:59:59Z"
}'
```

## Load Testing

```bash
# Build load test
go build -o bin/load_test ./benchmarks

# Run 100K spans/sec for 5 minutes
./bin/load_test -rate 100000 -duration 5m -workers 20
```

See [benchmarks/README.md](benchmarks/README.md) for more options.

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires ClickHouse)
go test -tags=integration ./tests/integration/...

# With coverage
go test -cover ./...
```

See [tests/README.md](tests/README.md) for detailed testing guide.

## Configuration

**ClickHouse Connection:**
- Username: `default`
- Password: *(empty)*
- Database: `otel`

**Key Config Files:**
- `configs/collector.yaml` - Collector settings
- `configs/query.yaml` - Query API settings
- `deployments/docker/docker-compose.yaml` - Docker setup
- `deployments/k8s/` - Kubernetes manifests

**Environment Variables:**
```bash
CLICKHOUSE_HOST=clickhouse:9000
CLICKHOUSE_DATABASE=otel
CLICKHOUSE_USERNAME=default
CLICKHOUSE_PASSWORD=
LOG_LEVEL=info
```

## Project Structure

```
otelservices/
├── cmd/                # Service entry points
│   ├── collector/      # OTLP Collector
│   └── query/          # Query API
├── internal/           # Shared packages
│   ├── clickhouse/     # Database client
│   ├── config/         # Configuration
│   ├── models/         # Data models
│   └── monitoring/     # Metrics/health
├── deployments/        # Deployment configs
│   ├── docker/         # Docker Compose
│   └── k8s/            # Kubernetes
├── schema/             # Database schema
├── benchmarks/         # Load testing
├── tests/              # Integration tests
├── dashboards/         # Grafana dashboards
└── configs/            # YAML configs
```

## Performance

- **Ingestion:** 100K+ spans/sec
- **Query Latency:** p95 < 500ms (24h queries)
- **Compression:** ~10:1 ratio
- **Retention:** 30d raw, 90d 5m rollups, 1y 1h rollups
- **Memory:** <4GB per collector
- **Storage:** <1TB per billion spans

## Kubernetes Deployment

```bash
kubectl apply -f deployments/k8s/namespace.yaml
kubectl create configmap clickhouse-schema --from-file=schema/ -n otel-system
kubectl apply -f deployments/k8s/clickhouse-statefulset.yaml
kubectl apply -f deployments/k8s/collector-deployment.yaml
kubectl apply -f deployments/k8s/query-deployment.yaml
```

## Monitoring

**Prometheus Metrics:**
- Collector: `http://localhost:9090/metrics`
- Query API: `http://localhost:9091/metrics`

**Key Metrics:**
- `otel_received_spans_total`
- `otel_storage_writes_total`
- `otel_storage_write_duration_seconds`
- `otel_query_duration_seconds`

**Grafana Dashboards:**
Import from `dashboards/` at http://localhost:3000

## Built With

- [OpenTelemetry](https://opentelemetry.io/)
- [ClickHouse](https://clickhouse.com/)
- [Prometheus](https://prometheus.io/)
- [Grafana](https://grafana.com/)
