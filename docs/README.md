# Distance Matrix — Documentation

Enterprise OD distance/time matrix for VRP, dispatch, and route optimization.

## Guides

| Document | Description |
|----------|-------------|
| [API Reference](./api-reference.md) | Endpoints, request/response, errors |
| [OpenAPI](./openapi.yaml) | Machine-readable API spec |
| [Architecture](./architecture.md) | Components, cache, L2 archive |
| [Configuration](./configuration.md) | `matrix.yaml` reference |
| [Operations](./operations.md) | Deploy, health, metrics |
| [Testing](./testing.md) | Unit, e2e, live |
| [User Stories](./user-stories.md) | E2E acceptance scenarios |
| [Key pool scheduler](./key-pool-algorithm.md) | Multi-key ADCS |
| [Design index](./design/README.md) | Specs and Dense planner |

## What this service does

- Synchronous **N×N distance / duration** matrices over HTTP  
- **Edge cache** in Redis (tenant / method / strategy isolation)  
- Optional **MySQL L2** archive when `Persistence.DSN` is set  
- Dense miss planning to amortize provider quota  
- Haversine fallback when provider fails (see ops)  
- **504 + write-through** so retries reuse partial progress  

## Quick start

```bash
docker compose up -d redis          # + mysql if you enable Persistence.DSN
go run matrix.go -f etc/matrix.yaml
curl -s http://localhost:8888/health/ready
```

See [root README](../README.md) for build and tests.
