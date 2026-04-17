# Deployment Guide

## Overview

This guide covers deploying CloudApp in various environments, from local development to production Kubernetes clusters.

## Local Development

### Docker Compose

The simplest way to run CloudApp locally is with Docker Compose.

#### Mock Mode (Default)

Uses mock providers for ASR, LLM, and TTS — no external API keys required.

```bash
cd infra/compose

# Copy environment file
cp .env.mock .env

# Start all services
docker-compose up --build

# Or run in background
docker-compose up -d --build
```

Services will be available at:

| Service | URL |
|---------|-----|
| Media-Edge | `ws://localhost:8080/ws` |
| Orchestrator | `http://localhost:8081` |
| Provider Gateway | `localhost:50051` (gRPC) |
| Redis | `localhost:6379` |
| PostgreSQL | `localhost:5432` |
| Prometheus | `http://localhost:9090` |

#### vLLM Mode

Uses local vLLM inference for LLM, faster-whisper for ASR.

```bash
cd infra/compose

# Start vLLM first (separate terminal)
docker run --gpus all -p 8000:8000 \
  vllm/vllm-openai:latest \
  --model meta-llama/Meta-Llama-3-8B-Instruct

# Start CloudApp with vLLM config
docker-compose --env-file .env.vllm up
```

#### Cloud Mode

Uses Google Cloud Speech and Groq for production-quality AI.

```bash
cd infra/compose

# Set required environment variables
export GROQ_API_KEY="your-groq-api-key"
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"

# Start with cloud config
docker-compose --env-file .env.cloud up
```

### Docker Compose Profiles

Profiles allow selective service startup:

```bash
# Only core services (no monitoring)
docker-compose --profile core up

# Core + GPU services
docker-compose --profile core --profile gpu up

# Everything including monitoring
docker-compose --profile full up
```

## Building Individual Services

### Media-Edge (Go)

```bash
# Build Docker image
docker build -f infra/docker/Dockerfile.media-edge -t cloudapp/media-edge:latest .

# Or run locally
cd go/media-edge
go run cmd/main.go --config ../../examples/config-mock.yaml
```

### Orchestrator (Go)

```bash
# Build Docker image
docker build -f infra/docker/Dockerfile.orchestrator -t cloudapp/orchestrator:latest .

# Or run locally
cd go/orchestrator
go run cmd/main.go --config ../../examples/config-mock.yaml
```

### Provider Gateway (Python)

```bash
# Build Docker image
docker build -f infra/docker/Dockerfile.provider-gateway -t cloudapp/provider-gateway:latest .

# Or run locally
cd py/provider_gateway
pip install -r requirements.txt
python main.py
```

## Kubernetes Deployment

### Prerequisites

- Kubernetes 1.25+
- kubectl configured
- Helm 3.x (optional, for Helm charts)

### Namespace Setup

```bash
kubectl apply -f infra/k8s/namespace.yaml
```

### Secrets

Create secrets for sensitive configuration:

```bash
# Create secret for PostgreSQL
kubectl create secret generic cloudapp-secrets \
  --namespace=voice-engine \
  --from-literal=postgres-dsn="postgres://user:pass@postgres:5432/voiceengine?sslmode=require"

# Create secret for provider API keys
kubectl create secret generic provider-secrets \
  --namespace=voice-engine \
  --from-literal=groq-api-key="your-groq-key" \
  --from-literal=google-credentials="$(cat /path/to/credentials.json)"
```

### Core Services

Deploy core services in order:

```bash
# 1. Redis (or use managed Redis)
kubectl apply -f infra/k8s/redis.yaml

# 2. PostgreSQL (or use managed PostgreSQL)
kubectl apply -f infra/k8s/postgres.yaml

# 3. Provider Gateway
kubectl apply -f infra/k8s/provider-gateway.yaml

# 4. Orchestrator
kubectl apply -f infra/k8s/orchestrator.yaml

# 5. Media-Edge
kubectl apply -f infra/k8s/media-edge.yaml
```

### Configuration

Update ConfigMap for non-sensitive configuration:

```bash
kubectl apply -f infra/k8s/configmap.yaml
```

Or edit and apply:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cloudapp-config
  namespace: voice-engine
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
    providers:
      defaults:
        asr: "google_speech"
        llm: "groq"
        tts: "google_tts"
```

### Ingress

Enable external access via Ingress:

```bash
# Uncomment and edit ingress section in media-edge.yaml
kubectl apply -f infra/k8s/media-edge.yaml
```

Example Ingress configuration:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: media-edge
  namespace: voice-engine
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/websocket-services: "media-edge"
    cert-manager.io/cluster-issuer: "letsencrypt"
spec:
  tls:
    - hosts:
        - voice.example.com
      secretName: voice-tls
  rules:
    - host: voice.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: media-edge
                port:
                  number: 8080
```

## GPU Considerations

### Provider Gateway with GPU

For local AI inference (vLLM, faster-whisper), GPU is recommended:

```yaml
# In provider-gateway.yaml
spec:
  template:
    spec:
      containers:
        - name: provider-gateway
          resources:
            limits:
              nvidia.com/gpu: 1  # Request 1 GPU
          env:
            - name: NVIDIA_VISIBLE_DEVICES
              value: "all"
```

### GPU Node Selector

Schedule GPU workloads on GPU nodes:

```yaml
spec:
  template:
    spec:
      nodeSelector:
        cloud.google.com/gke-accelerator: nvidia-tesla-t4
      tolerations:
        - key: nvidia.com/gpu
          operator: Exists
          effect: NoSchedule
```

### GPU Drivers

Ensure GPU nodes have NVIDIA drivers installed:

```bash
# For GKE
gcloud container node-pools create gpu-pool \
  --cluster=cloudapp-cluster \
  --accelerator=type=nvidia-tesla-t4,count=1 \
  --enable-autoscaling \
  --min-nodes=0 \
  --max-nodes=3

# Install NVIDIA device plugin
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

## Environment Variables Reference

### Media-Edge

| Variable | Description | Default |
|----------|-------------|---------|
| `CLOUDAPP_CONFIG` | Path to config file | - |
| `CLOUDAPP_SERVER_HOST` | Bind address | `0.0.0.0` |
| `CLOUDAPP_SERVER_PORT` | HTTP port | `8080` |
| `CLOUDAPP_REDIS_ADDR` | Redis address | `localhost:6379` |
| `CLOUDAPP_REDIS_PASSWORD` | Redis password | - |
| `CLOUDAPP_POSTGRES_DSN` | PostgreSQL DSN | - |
| `CLOUDAPP_OBSERVABILITY_LOG_LEVEL` | Log level | `info` |

### Orchestrator

| Variable | Description | Default |
|----------|-------------|---------|
| `CLOUDAPP_CONFIG` | Path to config file | - |
| `CLOUDAPP_SERVER_HOST` | Bind address | `0.0.0.0` |
| `CLOUDAPP_SERVER_PORT` | HTTP port | `8081` |
| `CLOUDAPP_REDIS_ADDR` | Redis address | `localhost:6379` |
| `PROVIDER_GATEWAY_ADDRESS` | Provider gateway gRPC address | `localhost:50051` |

### Provider Gateway

| Variable | Description | Default |
|----------|-------------|---------|
| `PROVIDER_GATEWAY_CONFIG` | Path to config file | - |
| `PROVIDER_GATEWAY_SERVER__HOST` | gRPC bind address | `0.0.0.0` |
| `PROVIDER_GATEWAY_SERVER__PORT` | gRPC port | `50051` |
| `PROVIDER_GATEWAY_TELEMETRY__LOG_LEVEL` | Log level | `INFO` |
| `GROQ_API_KEY` | Groq API key | - |
| `GOOGLE_APPLICATION_CREDENTIALS` | Google credentials path | - |

## Health Check Endpoints

All services expose health endpoints:

### Media-Edge

```bash
# Liveness probe
curl http://media-edge:8080/health
# {"status": "ok"}

# Readiness probe
curl http://media-edge:8080/ready
# {"status": "ready"}
```

### Orchestrator

```bash
# Health
curl http://orchestrator:8081/health
# {"status":"healthy"}

# Readiness (checks Redis)
curl http://orchestrator:8081/ready
# {"status":"ready"}
```

### Provider Gateway

```bash
# gRPC health check (using grpc-health-probe)
grpc_health_probe -addr=provider-gateway:50051
```

## Scaling Considerations

### Horizontal Pod Autoscaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: media-edge
  namespace: voice-engine
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: media-edge
  minReplicas: 2
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Pods
      pods:
        metric:
          name: websocket_connections_active
        target:
          type: AverageValue
          averageValue: "100"
```

### Session Affinity

WebSocket connections require session affinity (sticky sessions):

```yaml
apiVersion: v1
kind: Service
metadata:
  name: media-edge
spec:
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 10800  # 3 hours
```

### Redis Clustering

For high availability, use Redis Cluster or Sentinel:

```yaml
# Redis Cluster configuration
redis:
  addresses:
    - "redis-node-1:6379"
    - "redis-node-2:6379"
    - "redis-node-3:6379"
```

## Monitoring with Prometheus

### Prometheus Configuration

```yaml
# infra/prometheus/prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'media-edge'
    static_configs:
      - targets: ['media-edge:8080']
    metrics_path: /metrics

  - job_name: 'orchestrator'
    static_configs:
      - targets: ['orchestrator:8081']
    metrics_path: /metrics

  - job_name: 'provider-gateway'
    static_configs:
      - targets: ['provider-gateway:9090']
```

### Key Metrics

| Metric | Description |
|--------|-------------|
| `websocket_connections_active` | Current WebSocket connections |
| `websocket_connections_total` | Total connections (counter) |
| `session_duration_seconds` | Session duration histogram |
| `asr_latency_seconds` | ASR processing latency |
| `llm_time_to_first_token_seconds` | LLM TTFT |
| `tts_time_to_first_chunk_seconds` | TTS TTFChunk |
| `interruption_stop_latency_seconds` | Interruption handling latency |

### Grafana Dashboard

Import the example dashboard (when available):

```bash
kubectl create configmap grafana-dashboards \
  --from-file=cloudapp-dashboard.json \
  --namespace=monitoring
```

## Production Checklist

- [ ] Use managed Redis (AWS ElastiCache, GCP Memorystore)
- [ ] Use managed PostgreSQL (AWS RDS, GCP Cloud SQL)
- [ ] Enable TLS for all connections
- [ ] Configure resource limits and requests
- [ ] Set up HPA for all services
- [ ] Configure pod disruption budgets
- [ ] Enable distributed tracing
- [ ] Set up log aggregation (ELK, Loki)
- [ ] Configure backup for PostgreSQL
- [ ] Set up alerting (PagerDuty, Opsgenie)
- [ ] Document runbooks

## Troubleshooting

### Services Not Starting

Check logs:

```bash
# Kubernetes
kubectl logs -n voice-engine deployment/media-edge
kubectl logs -n voice-engine deployment/orchestrator
kubectl logs -n voice-engine deployment/provider-gateway

# Docker Compose
docker-compose logs -f media-edge
```

### Connection Issues

Verify service discovery:

```bash
# From media-edge pod
kubectl exec -it deployment/media-edge -n voice-engine -- sh
nc -zv orchestrator 8081
nc -zv provider-gateway 50051
nc -zv redis 6379
```

### High Latency

Check resource utilization:

```bash
kubectl top pods -n voice-engine
kubectl top nodes
```

### Provider Errors

Check provider gateway logs:

```bash
kubectl logs -n voice-engine deployment/provider-gateway | grep ERROR
```
