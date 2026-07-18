# Architecture

## Overview

```
Client (VRP / optimizer)
        │
        ▼
┌───────────────────┐
│  HTTP Handler     │  deadline, tenant, rate limit, validation
└─────────┬─────────┘
          ▼
┌───────────────────┐
│  MatrixEngine     │  probe cache → plan misses → fill N×N
└─────────┬─────────┘
          │
    ┌─────┴──────┬──────────────┐
    ▼            ▼              ▼
┌────────┐  ┌──────────┐  ┌──────────────┐
│EdgeCache│  │EdgeArchive│  │ DensePlanner │
│ (Redis) │  │(MySQL L2) │  │ + batch route│
└────────┘  └──────────┘  └──────┬───────┘
     ▲ optional DSN               ▼
                          ┌──────────────┐
                          │ AmapProvider │  multi-key pool + fallback
                          └──────────────┘
```

`EdgeArchive` is off unless `Persistence.DSN` is set.

## Design principles

1. **Cache edges, not matrices** — directed `(origin → destination)` segments, reused across requests.
2. **Write-through** — provider successes go to Redis immediately; async MySQL when archive enabled. Supports 504 retry.
3. **Hot path is Redis** — MySQL is L2 only (miss → cold read → promote). DDL is offline (`scripts/ddl/`).
4. **Approximation is internal** — haversine fallback is metered; API returns numbers only.
5. **Tenant / method / strategy isolation** — never cross-read car vs truck.

## Cache key model

```
{tenant}:{prefix}:geo
{tenant}:{prefix}:edge:{method:strategy}:{bGeo}:{eGeo}   # HASH, field = WMH timeslot
```

- **Strict**: exact geohash + timeslot  
- **Non-strict**: reverse edge, GEO radius (`geo_wide_m`), timeslot ±1h  

## Matrix computation flow

1. Convert coordinates to GCJ-02  
2. Probe Redis (then MySQL L2 if configured) for every OD  
3. Plan residual misses with **DensePlanner** (`internal/arccover`)  
4. Execute planned walks via provider batch; write-through hits  
5. Fill remaining cells (pair route / reverse / fallback)  
6. Return distance + duration matrices  

## Provider layer

- **Registry** — pluggable (`amap` today)  
- **Amap** — driving direction + [ADCS key pool](./key-pool-algorithm.md)  
- **Fallback** — haversine × 1.5 (Amap provider)  

## Packages

| Package | Role |
|---------|------|
| `internal/handler` | HTTP, metrics, rate limit |
| `internal/engine` | Matrix orchestration |
| `internal/cache` | Redis edge store |
| `internal/persist` | Optional MySQL L2 archive |
| `internal/arccover` | Dense miss planner |
| `internal/planner` | Provider batch / waypoint packs |
| `internal/provider` | Amap + registry |
| `internal/loadbalance` | Key selection |
| `internal/geo` | Coordinates, haversine |
| `internal/middleware` | Per-tenant QPS |

## Out of scope

- Batch / async matrix API  
- Response `meta` / confidence scores  
- MySQL on the hot read path  
- Legacy `/api/*` routes  

See [design spec](./design/enterprise-matrix.md).
