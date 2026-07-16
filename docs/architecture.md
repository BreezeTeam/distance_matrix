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
│  MatrixEngine     │  fill N×N matrix, orchestrate cache + provider
└─────────┬─────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
┌────────┐  ┌──────────────┐
│ EdgeCache│  │ RoutePlanner │
│ (Redis) │  │ chain+batch  │
└────────┘  └──────┬───────┘
                   ▼
            ┌──────────────┐
            │ AmapProvider │  multi-key pool + fallback
            └──────────────┘
```

## Design principles

1. **Cache edges, not matrices** — Each directed segment `(origin → destination)` is stored once and reused across matrix requests.
2. **Write-through** — Successful provider results are written to Redis immediately; 504 retries benefit from partial progress.
3. **Approximation is explicit internally** — Haversine fallback is logged and metered; API returns numbers without quality flags (by design).
4. **Tenant isolation** — Redis keys prefixed `{tenant}:distance_matrix:...`

## Cache key model

```
{tenant}:{prefix}:geo                          GEO index
{tenant}:{prefix}:edge:{method:strategy}:{bGeo}:{eGeo}   HASH per edge bucket
```

Each HASH field is a time slot (`WMH` = weekday+hour). Lookup:

- **Strict**: exact geohash pair + timeslot
- **Non-strict**: may reuse reverse edge; fuzzy GEO radius (`geo_wide_m`); adjacent timeslots ±1h

**Method** and **strategy** are part of the context key — car vs truck never cross-read.

## Matrix computation flow

1. Convert coordinates to GCJ-02
2. Greedy **chain** over distinct points → minimize provider calls
3. For each chain segment: cache lookup → batch route on miss
4. Fill remaining matrix cells (strict / reverse / per-pair route)
5. Return matrices + internal stats

## Provider layer

- **Registry** — pluggable providers (`amap` today)
- **Amap** — v3 driving direction, multi-key pool with [Adaptive Decaying Confidence Scheduler](./key-pool-algorithm.md)
- **Fallback** — haversine distance × `FallbackFactor` (default 1.5), synthetic duration

## Packages

| Package | Role |
|---------|------|
| `internal/handler` | HTTP, metrics, rate limit |
| `internal/engine` | Matrix orchestration |
| `internal/cache` | Redis GEO + HASH store |
| `internal/planner` | Chain ordering, batch routing |
| `internal/provider` | Amap + registry |
| `internal/loadbalance` | Key selection formula |
| `internal/geo` | Coordinates, haversine |
| `internal/middleware` | Per-tenant QPS |

## What is NOT in scope (v2)

- Batch/async matrix API
- Response `meta` / confidence scores
- MySQL hot read path
- Legacy `/api/*` routes (removed)

See [design spec](./design/enterprise-matrix-v2.md) for full rationale.
