# Operations

## Deploy

### Docker Compose

```bash
# Full local stack (app + Redis + MySQL on :3306)
docker compose -f docker-compose.dev.yml up -d --build
curl http://127.0.0.1:8888/health/ready
```

| Service | Host port |
|---------|-----------|
| matrix | 8888 |
| redis | 6379 |
| mysql | 3306 |

First MySQL start applies `scripts/ddl/` via `docker-entrypoint-initdb.d`.  
In-compose DSN uses hostname `mysql:3306` ([`etc/matrix.docker.yaml`](../etc/matrix.docker.yaml)).

### Binary

```bash
docker compose -f docker-compose.dev.yml up -d redis mysql
go build -o matrix matrix.go
./matrix -f etc/matrix.dev.yaml   # DSN → 127.0.0.1:3306
```

Requires Redis when `Redis.Enabled: true`. MySQL only when `Persistence.DSN` is set (table must already exist unless `AutoMigrate: true`).

```bash
mysql -h 127.0.0.1 -P 3306 -u root -p < scripts/ddl/001_distance_matrix_edge.sql
```

### Request timeout

YAML `Timeout` is the **business** deadline → HTTP **504** `MATRIX_DEADLINE` with write-through.  
go-zero Rest `TimeoutHandler` is disabled (`config.ForServer`) so it cannot race with plain **503** `Request Timeout`.

## Health checks

| Endpoint | Use | Pass |
|----------|-----|------|
| `/health/live` | Liveness | `200 ok` |
| `/health/ready` | Readiness | `200 ready` |

Ready fails if Redis is enabled but down, or no Amap keys. MySQL down → archive disabled (log), readiness unaffected.

## Prometheus

| Metric | Labels | Meaning |
|--------|--------|---------|
| `matrix_api_requests_total` | tenant, status | Requests |
| `matrix_api_request_duration_ms` | tenant | Latency |
| `matrix_engine_fallback_edges_total` | tenant | Haversine edges |
| `matrix_engine_provider_calls_total` | tenant, provider | Provider batches |

Alert ideas: high fallback ratio; p95 latency vs SLO.

## Logs

```
matrix tenant=... n=... cache_hit=... fallback=... provider_calls=... elapsed_ms=...
```

## Capacity

```
Provider QPS ≈ Client QPS × avg_points × (1 - effective_hit_ratio)
```

Limits: `TenantQPS` 50, `MaxPoints` 100, Amap batch 12.

```bash
bash scripts/load_matrix.sh
bash scripts/verify_amap_keys.sh
go run ./scripts/simulate_key_pool.go
```

## Fallback

Provider failure → **200 with haversine×factor**. Not road distance. Watch `matrix_engine_fallback_edges_total`.

## 504 retry

1. Receive `504 MATRIX_DEADLINE`  
2. Wait ≥500ms  
3. Retry identical request (tenant, points, method, strategy, timeslot)  
4. Redis (and L2) edges from the first attempt speed completion  

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Distances ~straight-line | Fallback — keys, quota, logs |
| 503 ready | Redis addr, Amap keys |
| Archive open failed | DSN, DDL applied, grants (DML only) |
| 429 | `TenantQPS` / client backoff |
| Slow cold matrix | Misses — raise timeout or shrink points |
| One key always fails | `verify_amap_keys.sh`, remove dead keys |
