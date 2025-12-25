# Deployment Guide

## Prerequisites

- Go 1.21+
- Docker and Docker Compose (for local development)
- Kubernetes 1.24+ (for production)
- ClickHouse 23.8+

## Local Development with Docker Compose

### Quick Start

1. **Clone and build:**
```bash
cd otelservices
go mod download
```

2. **Start all services:**
```bash
cd deployments/docker
docker-compose up -d
```

3. **Initialize ClickHouse schema:**
```bash
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/001_create_otel_metrics.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/002_create_otel_logs.sql
docker exec -it otel-clickhouse clickhouse-client --multiquery < ../../schema/003_create_otel_traces.sql
```

4. **Verify services:**
```bash
# Check collector health
curl http://localhost:8080/health

# Check query service health
curl http://localhost:8081/health

# Check Prometheus metrics
curl http://localhost:9090/metrics
```

5. **Access Grafana:**
- URL: http://localhost:3000
- Username: admin
- Password: admin

### Service Endpoints

| Service | Port | Description |
|---------|------|-------------|
| OTLP gRPC | 4317 | OpenTelemetry Protocol (gRPC) |
| OTLP HTTP | 4318 | OpenTelemetry Protocol (HTTP) |
| Collector Health | 8080 | Health and readiness checks |
| Query API | 8081 | REST API for queries |
| Collector Metrics | 9090 | Prometheus metrics |
| Query Metrics | 9091 | Prometheus metrics |
| Prometheus | 9092 | Prometheus UI |
| Grafana | 3000 | Grafana dashboards |
| ClickHouse HTTP | 8123 | ClickHouse HTTP interface |
| ClickHouse Native | 9000 | ClickHouse native protocol |

### Stopping Services

```bash
docker-compose down
```

### Cleaning Up Data

```bash
docker-compose down -v  # Remove volumes
```

## Production Deployment on Kubernetes

### Prerequisites

1. **Kubernetes cluster** with:
   - 3+ worker nodes
   - 100GB+ storage per node
   - Network policy support (optional)

2. **kubectl** configured to access cluster

3. **Container registry** access for custom images

### Build and Push Images

1. **Build collector image:**
```bash
docker build -f deployments/docker/Dockerfile.collector -t your-registry/otel-collector:latest .
docker push your-registry/otel-collector:latest
```

2. **Build query service image:**
```bash
docker build -f deployments/docker/Dockerfile.query -t your-registry/otel-query:latest .
docker push your-registry/otel-query:latest
```

3. **Update Kubernetes manifests** with your registry URLs in:
   - `deployments/k8s/collector-deployment.yaml`
   - `deployments/k8s/query-deployment.yaml`

### Deploy to Kubernetes

1. **Create namespace:**
```bash
kubectl apply -f deployments/k8s/namespace.yaml
```

2. **Create ClickHouse schema ConfigMap:**
```bash
kubectl create configmap clickhouse-schema \
  --from-file=schema/ \
  -n otel-system
```

3. **Deploy ClickHouse:**
```bash
kubectl apply -f deployments/k8s/clickhouse-statefulset.yaml
```

Wait for ClickHouse to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=clickhouse -n otel-system --timeout=300s
```

4. **Deploy Collector:**
```bash
kubectl apply -f deployments/k8s/collector-deployment.yaml
```

5. **Deploy Query Service:**
```bash
kubectl apply -f deployments/k8s/query-deployment.yaml
```

### Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n otel-system

# Check services
kubectl get svc -n otel-system

# Check logs
kubectl logs -n otel-system -l app=otel-collector --tail=100
kubectl logs -n otel-system -l app=otel-query --tail=100
```

### Expose Services

#### Option 1: LoadBalancer (Cloud Environments)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: otel-collector-lb
  namespace: otel-system
spec:
  type: LoadBalancer
  selector:
    app: otel-collector
  ports:
    - port: 4317
      name: otlp-grpc
    - port: 4318
      name: otlp-http
```

#### Option 2: Ingress (With Ingress Controller)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: otel-ingress
  namespace: otel-system
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "GRPC"
spec:
  rules:
    - host: otel-collector.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: otel-collector
                port:
                  number: 4317
```

### Scaling

#### Manual Scaling

```bash
# Scale collector
kubectl scale deployment otel-collector -n otel-system --replicas=5

# Scale query service
kubectl scale deployment otel-query -n otel-system --replicas=3
```

#### Auto-scaling (HPA already configured)

The Horizontal Pod Autoscaler is configured to scale based on CPU and memory:
- Min replicas: 3
- Max replicas: 10
- Target CPU: 70%
- Target Memory: 80%

Monitor autoscaling:
```bash
kubectl get hpa -n otel-system -w
```

### Monitoring

#### Prometheus Integration

If you have Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: otel-collector
  namespace: otel-system
spec:
  selector:
    matchLabels:
      app: otel-collector
  endpoints:
    - port: metrics
      interval: 30s
```

#### View Metrics

```bash
# Port-forward Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Visit http://localhost:9090
```

### Backup and Restore

#### Backup ClickHouse Data

```bash
# Create backup using clickhouse-backup
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-backup create

# Copy backup to local
kubectl cp otel-system/clickhouse-0:/var/lib/clickhouse/backup ./backup
```

#### Restore ClickHouse Data

```bash
# Copy backup to pod
kubectl cp ./backup otel-system/clickhouse-0:/var/lib/clickhouse/backup

# Restore
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-backup restore
```

## Configuration Management

### Using ConfigMaps

Update configuration without rebuilding images:

```bash
# Edit ConfigMap
kubectl edit configmap collector-config -n otel-system

# Restart pods to pick up changes
kubectl rollout restart deployment otel-collector -n otel-system
```

### Using Secrets

For sensitive data (passwords, API keys):

```bash
# Create secret
kubectl create secret generic clickhouse-credentials \
  --from-literal=username=default \
  --from-literal=password=yourpassword \
  -n otel-system

# Reference in deployment
env:
  - name: CLICKHOUSE_PASSWORD
    valueFrom:
      secretKeyRef:
        name: clickhouse-credentials
        key: password
```

## Troubleshooting

### Collector Not Receiving Data

1. Check pod logs:
```bash
kubectl logs -n otel-system -l app=otel-collector
```

2. Verify service endpoints:
```bash
kubectl get endpoints -n otel-system otel-collector
```

3. Test connectivity:
```bash
kubectl run -it --rm debug --image=busybox --restart=Never -n otel-system -- telnet otel-collector 4317
```

### ClickHouse Connection Issues

1. Check ClickHouse is running:
```bash
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-client --query "SELECT 1"
```

2. Check network connectivity:
```bash
kubectl exec -it otel-collector-xxx -n otel-system -- nc -zv clickhouse 9000
```

### High Memory Usage

1. Check memory metrics:
```bash
kubectl top pods -n otel-system
```

2. Adjust resource limits:
```yaml
resources:
  limits:
    memory: "8Gi"
  requests:
    memory: "4Gi"
```

3. Tune batch sizes in config:
```yaml
performance:
  batch_size: 5000  # Reduce from 10000
  queue_size: 50000  # Reduce from 100000
```

### Slow Queries

1. Check ClickHouse query log:
```bash
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-client --query "SELECT query, query_duration_ms FROM system.query_log WHERE query_duration_ms > 1000 ORDER BY query_start_time DESC LIMIT 10"
```

2. Enable query profiling:
```sql
SET send_logs_level = 'trace';
```

3. Optimize tables:
```bash
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-client --query "OPTIMIZE TABLE otel_traces FINAL"
```

## Maintenance

### Rolling Updates

```bash
# Update collector image
kubectl set image deployment/otel-collector collector=your-registry/otel-collector:v2.0.0 -n otel-system

# Watch rollout status
kubectl rollout status deployment/otel-collector -n otel-system
```

### Database Maintenance

```bash
# Compact old partitions
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-client --query "OPTIMIZE TABLE otel_traces"

# Check disk usage
kubectl exec -it clickhouse-0 -n otel-system -- clickhouse-client --query "SELECT formatReadableSize(sum(bytes)) FROM system.parts WHERE database = 'otel'"
```

## Performance Tuning

See [TUNING.md](TUNING.md) for detailed performance optimization guidelines.
