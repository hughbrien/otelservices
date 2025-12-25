# OpenTelemetry Services

A high-performance OpenTelemetry data pipeline built in Go that collects, processes, and stores metrics, logs, and traces with 1-year retention optimization.

## Features

- **High Performance:** 100,000+ spans/sec per collector instance
- **Optimized Storage:** ClickHouse with ZSTD compression (~10:1 ratio)
- **Long-Term Retention:** Automatic rollups for 1-year data retention
- **Query Performance:** p95 < 500ms for 24-hour queries
- **Compatible APIs:** Jaeger, Prometheus, and Loki compatible query interfaces
- **Production Ready:** Docker Compose for dev, Kubernetes manifests for production
- **Comprehensive Monitoring:** Prometheus metrics and Grafana dashboards

## Architecture

```
Applications → OTLP Collector → ClickHouse → Query API → Visualization
                (4317/4318)     (Optimized    (REST API)   (Grafana)
                                 Storage)
```

### Components

1. **OTLP Collector Service**
   - Receives OTLP gRPC and HTTP traffic
   - Batching, retry logic, and backpressure handling
   - Writes to ClickHouse with optimal compression

2. **ClickHouse Storage**
   - Column-oriented database with ~10:1 compression
   - Automatic data rollups (5m, 1h aggregations)
   - TTL-based retention policies (30d raw, 1y rollups)

3. **Query API Service**
   - Prometheus-compatible metrics queries
   - Loki-compatible log queries
   - Jaeger-compatible trace queries
   - Service statistics endpoints

## Quick Start

### Local Development with Docker Compose

```bash
# Start all services
cd deployments/docker
docker-compose up -d

# Initialize ClickHouse schema
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/001_create_otel_metrics.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/002_create_otel_logs.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/003_create_otel_traces.sql

# Verify services are running
curl http://localhost:8080/health  # Collector
curl http://localhost:8081/health  # Query API

# Access Grafana at http://localhost:3000 (admin/admin)
```

### Production Deployment on Kubernetes

```bash
# Create namespace
kubectl apply -f deployments/k8s/namespace.yaml

# Create schema ConfigMap
kubectl create configmap clickhouse-schema --from-file=schema/ -n otel-system

# Deploy ClickHouse
kubectl apply -f deployments/k8s/clickhouse-statefulset.yaml

# Deploy Collector
kubectl apply -f deployments/k8s/collector-deployment.yaml

# Deploy Query Service
kubectl apply -f deployments/k8s/query-deployment.yaml
```

## Usage

### Sending Data to the Collector

Configure your application to export OTLP data:

```go
import (
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

exporter, _ := otlptracegrpc.New(
    context.Background(),
    otlptracegrpc.WithEndpoint("localhost:4317"),
    otlptracegrpc.WithInsecure(),
)
```

### Querying Data

#### Query Traces

```bash
curl -X POST http://localhost:8081/api/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "my-service",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T23:59:59Z",
    "limit": 100
  }'
```

#### Query Metrics

```bash
curl -X POST http://localhost:8081/api/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "metric_name": "http_request_duration",
    "service_name": "api-server",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T23:59:59Z",
    "aggregation": "avg"
  }'
```

#### Query Logs

```bash
curl -X POST http://localhost:8081/api/v1/logs \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "api-server",
    "start_time": "2024-01-01T00:00:00Z",
    "end_time": "2024-01-01T23:59:59Z",
    "severity": "ERROR",
    "limit": 100
  }'
```

#### Get Service Statistics

```bash
curl http://localhost:8081/api/v1/services/stats
```

## Performance Testing

Run the included load test tool:

```bash
cd benchmarks
go build -o load_test load_test.go

# Test at 100K spans/sec for 5 minutes
./load_test -rate 100000 -duration 5m -workers 20

# Monitor metrics
curl http://localhost:9090/metrics | grep otel_
```

## Monitoring

### Prometheus Metrics

- **Collector:** `http://localhost:9090/metrics`
- **Query Service:** `http://localhost:9091/metrics`

Key metrics:
- `otel_received_spans_total` - Total spans received
- `otel_storage_writes_total` - Storage write operations
- `otel_storage_write_duration_seconds` - Write latency
- `otel_query_duration_seconds` - Query latency

### Grafana Dashboards

Pre-built dashboards in `dashboards/`:
- `otel-collector-dashboard.json` - Collector performance
- `otel-query-dashboard.json` - Query API performance
- `clickhouse-dashboard.json` - Storage statistics

Import into Grafana at http://localhost:3000

## Project Structure

```
otelservices/
├── cmd/
│   ├── collector/      # OTLP Collector service
│   ├── storage/        # (Integrated into collector)
│   └── query/          # Query API service
├── internal/
│   ├── config/         # Configuration management
│   ├── clickhouse/     # ClickHouse client
│   ├── models/         # Data models
│   └── monitoring/     # Prometheus metrics
├── deployments/
│   ├── docker/         # Docker Compose setup
│   └── k8s/            # Kubernetes manifests
├── schema/             # ClickHouse DDL scripts
├── benchmarks/         # Performance testing tools
├── dashboards/         # Grafana dashboards
├── docs/               # Documentation
└── configs/            # Configuration files
```

## Documentation

- [Architecture](docs/ARCHITECTURE.md) - System design and components
- [Deployment Guide](docs/DEPLOYMENT.md) - Installation and deployment
- [Performance Tuning](docs/TUNING.md) - Optimization guidelines

## Performance Targets

- **Ingestion:** 100,000+ spans/sec per instance
- **Query Latency:** p95 < 500ms for 24-hour queries
- **Storage Efficiency:** <1TB for 1 billion spans
- **Memory Usage:** <4GB per collector instance
- **CPU Usage:** <50% utilization at 50K spans/sec

## Configuration

### Collector Configuration

Edit `configs/collector.yaml`:

```yaml
clickhouse:
  addresses: ["localhost:9000"]
  database: "otel"

otlp:
  grpc_port: 4317
  http_port: 4318

performance:
  batch_size: 10000
  batch_timeout: 10s
  worker_count: 4
```

### Query Service Configuration

Edit `configs/query.yaml`:

```yaml
clickhouse:
  addresses: ["localhost:9000"]
  database: "otel"

server:
  port: 8081
```

## Environment Variables

Override configuration with environment variables:

```bash
export CLICKHOUSE_HOST="clickhouse:9000"
export CLICKHOUSE_DATABASE="otel"
export CLICKHOUSE_USERNAME="default"
export CLICKHOUSE_PASSWORD="secret"
export LOG_LEVEL="debug"
export OTLP_GRPC_PORT="4317"
```

## Development

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- ClickHouse 23.8+

### Building

```bash
# Install dependencies
go mod download

# Build collector
go build -o bin/collector ./cmd/collector

# Build query service
go build -o bin/query ./cmd/query

# Build benchmarks
go build -o bin/load_test ./benchmarks
```

### Testing

```bash
# Run unit tests
go test ./...

# Run integration tests (requires running ClickHouse)
go test -tags=integration ./...
```

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please read CONTRIBUTING.md for guidelines.

## Support

- **Documentation:** See `docs/` directory
- **Issues:** GitHub Issues
- **Discussions:** GitHub Discussions

## Acknowledgments

Built with:
- [OpenTelemetry](https://opentelemetry.io/)
- [ClickHouse](https://clickhouse.com/)
- [Prometheus](https://prometheus.io/)
- [Grafana](https://grafana.com/)
