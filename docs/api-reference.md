# API Reference

Base URL: `http://<host>:8888` (default port from `matrix.yaml`)

All JSON APIs use envelope `{ "code", "msg", "data?" }` except health endpoints (plain text).

## Headers

| Header | Required | Default | Purpose |
|--------|----------|---------|---------|
| `Content-Type` | POST | — | `application/json` |
| `X-Tenant-Id` | No | `default` | Tenant namespace for cache + rate limit |

---

## POST /v1/matrix

Compute OD distance (meters) and duration (seconds) matrices.

### Request

```json
{
  "points": [[116.40, 39.90], [116.41, 39.91]],
  "coordinate": "gcj02",
  "strategy": 0,
  "method": 0,
  "timeslot": "",
  "strict": false,
  "geo_wide_m": 200,
  "provider": "amap",
  "speed_mps": 0
}
```

| Field | Type | Description |
|-------|------|-------------|
| `points` | `[[lon,lat],...]` | ≥2 points, max `Engine.MaxPoints` (default 100) |
| `coordinate` | string | `gcj02` (default) or `wgs84` |
| `strategy` | int | Amap driving strategy (0=default, 12=shortest, …) |
| `method` | int | Cache context key (e.g. car=0, truck=1) |
| `timeslot` | string | Time bucket override (`WMH` format); empty = current hour |
| `strict` | bool | `true`: exact geohash only; `false`: allow reverse + fuzzy |
| `geo_wide_m` | int | Fuzzy spatial cache radius (meters) |
| `provider` | string | Provider name (default `amap`) |
| `speed_mps` | int | Override speed for duration estimate |

### Success — 200

```json
{
  "code": 200,
  "msg": "OK",
  "data": {
    "distances": [[0, 1200], [1180, 0]],
    "durations": [[0, 180], [175, 0]]
  }
}
```

- Diagonal is always `0`
- Off-diagonal values are **meters** / **seconds**
- Fallback edges use haversine approximation (not road truth)

### Deadline — 504

```json
{ "code": 504, "msg": "MATRIX_DEADLINE" }
```

HTTP status `504`. Partial edges already written to Redis remain; **retry the same request** within cache TTL.

### Rate limit — 429

```json
{ "code": 429, "msg": "RATE_LIMIT" }
```

Per-tenant QPS limit (`Engine.TenantQPS`, default 50).

### Validation errors

HTTP 4xx via go-zero error handler, e.g.:

- `invalid point: need [lon, lat]`
- `need at least 2 points`
- `too many points: max 100`

---

## POST /v1/route

Multi-waypoint route with per-leg steps.

### Request

```json
{
  "points": [[116.40, 39.90], [116.41, 39.91], [116.42, 39.92]],
  "coordinate": "gcj02",
  "strategy": 0,
  "method": 0,
  "provider": "amap",
  "speed_mps": 0
}
```

### Success — 200

```json
{
  "code": 200,
  "msg": "OK",
  "data": {
    "distance": 2500,
    "duration": 360,
    "steps": [
      {
        "origin": [116.40, 39.90],
        "destination": [116.41, 39.91],
        "distance": 1200,
        "duration": 180
      }
    ]
  }
}
```

K points → K−1 steps. Not rate-limited.

---

## GET /v1/providers

### Success — 200

```json
{ "code": 200, "msg": "OK", "data": ["amap"] }
```

---

## GET /health/live

Liveness probe. **200** body: `ok` (plain text)

---

## GET /health/ready

Readiness: Redis connected (if enabled) + at least one provider key configured.

- **200** body: `ready`
- **503** body: `not ready`

---

## Error code summary

| HTTP | code | msg | When |
|------|------|-----|------|
| 200 | 200 | OK | Success |
| 429 | 429 | RATE_LIMIT | Tenant QPS exceeded (matrix only) |
| 504 | 504 | MATRIX_DEADLINE | Request deadline exceeded |
| 4xx | — | — | Validation / unknown provider |

Observability (cache hit ratio, fallback count) is **not** in API responses — see Prometheus metrics and logs.
