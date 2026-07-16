# Distance Matrix — Documentation

Enterprise OD distance/time matrix service for VRP, dispatch, and route optimization.

## Guides

| Document | Description |
|----------|-------------|
| [API Reference](./api-reference.md) | Endpoints, request/response, errors |
| [OpenAPI](./openapi.yaml) | Machine-readable API spec |
| [Architecture](./architecture.md) | Components, data flow, cache model |
| [Configuration](./configuration.md) | `matrix.yaml` reference |
| [Operations](./operations.md) | Deploy, health, metrics, capacity |
| [Testing](./testing.md) | Unit, e2e, live, manual HTTP |
| [User Stories](./user-stories.md) | E2E acceptance scenarios |

## Design

| Document | Description |
|----------|-------------|
| [Design index](./design/README.md) | Architecture specs and plans |
| [Enterprise Matrix v2](./design/enterprise-matrix-v2.md) | Full v2 design spec |
| [Key pool scheduler](./key-pool-algorithm.md) | Multi-key routing formula and tuning |

## What this service does

- Computes **N×N distance and duration matrices** synchronously over HTTP
- Caches **directed edges** in Redis (not full matrices) with tenant/method/strategy isolation
- Chains provider calls to amortize Amap quota
- Falls back to **haversine × factor** when provider fails (approximation — see ops doc)
- Supports **504 partial progress** with write-through cache for fast retry

## Quick start

```bash
docker compose up -d redis
go run matrix.go -f etc/matrix.yaml
curl -s http://localhost:8888/health/ready
```

See [root README](../README.md) for build and test commands.
