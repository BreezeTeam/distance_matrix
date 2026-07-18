# distance_matrix

[English](./README.md) | [中文](./README.zh-CN.md)

Enterprise **OD distance/time matrix** for VRP, dispatch, and route optimization — cache-first, multi-tenant, Amap-backed, synchronous HTTP N×N.

Most matrix clients burn quota one OD at a time and restart from scratch on timeout. This service caches directed edges, packs misses with Dense Arc Cover, and write-throughs progress so retries finish cheaper and faster.

## Why this

| Highlight | What you get |
|-----------|----------------|
| **Edge cache, not matrix cache** | Redis stores directed OD edges; reuse across requests in your tenant namespace |
| **Dense Arc Cover** | Miss edges planned into continuous walks (≤ L legs) so one provider call covers many pairs |
| **Write-through + resumable 504** | On deadline: `MATRIX_DEADLINE` with edges already in Redis; retry the same request and continue |
| **Fuzzy or strict lookup** | Near-neighbor GEO + reverse reuse when `strict=false`; exact geohash when you need determinism |
| **Tenant / method / strategy isolation** | Car vs truck (and customers) never share cache keys |
| **Multi-key ADCS pool** | Adaptive Amap key selection with soft-failure weighting and dead-key probe backoff |
| **Optional MySQL L2** | Redis stays on the hot path; cold archive is opt-in via DSN |
| **Observable degrade** | Haversine fallback on provider failure; watch `fallback` / `provider_calls` metrics — no junk fields in the API body |

Built for scheduling platforms, VRP solvers, and teams that need road-network matrices with controllable cost and hard multi-tenant boundaries.

## Documentation

| Doc | Content |
|-----|---------|
| [Docs index](./docs/README.md) | Full hub |
| [API](./docs/api-reference.md) | Endpoints, errors |
| [OpenAPI](./docs/openapi.yaml) | Spec |
| [Architecture](./docs/architecture.md) | Cache + L2 + planner |
| [Dense planner](./docs/design/dense-arc-cover-algorithm.md) | Miss-edge packing algorithm |
| [Configuration](./docs/configuration.md) | `matrix.yaml` |
| [Operations](./docs/operations.md) | Deploy, metrics |
| [Key pool](./docs/key-pool-algorithm.md) | Multi-key ADCS |
| [Design](./docs/design/README.md) | Specs |

## Quick start (Docker)

```bash
docker compose -f docker-compose.dev.yml up -d --build
curl -s http://127.0.0.1:8888/health/ready
```

Stack: `distance_matrix_app` (:8888), Redis (:6379), MySQL (:3306).  
App config inside Compose: [`etc/matrix.docker.yaml`](./etc/matrix.docker.yaml) (`redis` / `mysql` hostnames).

```bash
curl -X POST http://127.0.0.1:8888/v1/matrix \
  -H "Content-Type: application/json" \
  -H "X-Tenant-Id: default" \
  -d '{"points":[[116.40,39.90],[116.41,39.91]],"coordinate":"gcj02"}'
```

Host-run binary (deps only in Docker):

```bash
docker compose -f docker-compose.dev.yml up -d redis mysql
go run matrix.go -f etc/matrix.dev.yaml
```

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/matrix` | N×N distance/duration |
| POST | `/v1/route` | Multi-waypoint route |
| GET | `/v1/providers` | Providers |
| GET | `/health/live` | Liveness |
| GET | `/health/ready` | Readiness |

Timeout → **504** `MATRIX_DEADLINE` (partial write-through). Manual: [`test/api.http`](./test/api.http).

## Layout

```
matrix.go
etc/matrix.yaml | matrix.dev.yaml | matrix.docker.yaml
api/service.api
docs/
scripts/
  ddl/ genddl/            # MySQL schema from GORM model
  scenario_cache_matrix.py
  capacity_timeout_stress.py
internal/
  handler/ engine/ cache/ persist/ arccover/
  planner/ provider/ loadbalance/ geo/
test/e2e/
```

## Testing

```bash
go test ./...
go test ./test/e2e/ -v
go test ./test/e2e/ -tags=e2e -run TestLive -v   # live Amap

# Against running Compose stack:
python3 scripts/scenario_cache_matrix.py
python3 scripts/capacity_timeout_stress.py
```

## Architecture

```
HTTP → MatrixEngine → Redis (L1) → MySQL L2 (optional)
                   → DensePlanner → AmapProvider
```

## Deploy

```bash
docker compose -f docker-compose.dev.yml up -d --build
# or: docker compose up --build
```

DDL: edit `internal/persist/model.go`, then `go run ./scripts/genddl -dsn '...'` (or `AutoMigrate: true`). See [Operations](./docs/operations.md).

## License

See [LICENSE](./LICENSE).
