# Operations

## Deploy

### Docker Compose (recommended for dev/staging)

```bash
docker compose up -d redis
docker compose up --build matrix
curl http://localhost:8888/health/ready
```

### Binary

```bash
go build -o matrix matrix.go
./matrix -f etc/matrix.yaml
```

Requires Redis when `Redis.Enabled: true`.

## Health checks

| Endpoint | Use | Pass |
|----------|-----|------|
| `/health/live` | K8s liveness | `200 ok` |
| `/health/ready` | K8s readiness | `200 ready` |

Readiness fails when:

- Redis enabled but not connected
- No Amap keys configured

## Prometheus metrics

Namespace `matrix_*` (go-zero metric registry):

| Metric | Labels | Meaning |
|--------|--------|---------|
| `matrix_api_requests_total` | tenant, status | Request count |
| `matrix_api_request_duration_ms` | tenant | Latency histogram |
| `matrix_engine_fallback_edges_total` | tenant | Haversine fallback edges |
| `matrix_engine_provider_calls_total` | tenant, provider | Provider batch calls |

Wire scrape config to your Prometheus stack. Alert suggestions:

- `rate(matrix_engine_fallback_edges_total[5m]) / rate(matrix_api_requests_total[5m])` > 0.5 — provider degradation
- p95 `matrix_api_request_duration_ms` > SLO threshold

## Logs

Structured info log per matrix request (internal only):

```
matrix tenant=... n=... cache_hit=... fallback=... provider_calls=... elapsed_ms=...
```

## Capacity planning

Rule of thumb from design spec:

```
Provider QPS ≈ Client QPS × avg_points × (1 - cache_hit_ratio)
```

Example: 100 QPS × 50 points × 20% miss = **~1,000 provider calls/s** — validate with load test.

Default limits:

- `TenantQPS`: 50 per tenant on matrix
- `MaxPoints`: 100
- Amap batch: 12 waypoints per call

### Load smoke

```bash
bash scripts/load_matrix.sh
```

### Key health

```bash
bash scripts/verify_amap_keys.sh
go run ./scripts/simulate_key_pool.go
```

## Fallback semantics (important)

When Amap fails (quota, bad key, timeout), the engine returns **200 with approximate distances** (haversine × factor). This keeps optimizers running but **must not be treated as road distance**.

Monitor `matrix_engine_fallback_edges_total`. Document this behavior to downstream teams.

## 504 retry playbook

1. Client receives `504 MATRIX_DEADLINE`
2. Wait ≥500ms (avoid thundering herd)
3. Retry **identical request** (same tenant, points, method, strategy, timeslot)
4. Partial edges in Redis accelerate completion

## Troubleshooting

| Symptom | Check |
|---------|-------|
| All distances ~straight-line | Fallback active — keys, Amap quota, logs |
| 503 ready | Redis addr, keys in yaml |
| 429 storm | Raise `TenantQPS` or backoff client |
| Slow cold matrix | Normal — cache warming; reduce points or raise timeout |
| One key always fails | Run `verify_amap_keys.sh`, remove dead keys |
