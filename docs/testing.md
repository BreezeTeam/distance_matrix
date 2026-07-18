# Testing

## Test pyramid

```
        ┌─────────────┐
        │ Live e2e    │  -tags=e2e, real Amap keys
        ├─────────────┤
        │ HTTP e2e    │  test/e2e, miniredis + server
        ├─────────────┤
        │ User stories│  test/e2e/stories_test.go
        ├─────────────┤
        │ Unit/integr │  internal/*_test.go
        └─────────────┘
```

## Commands

```bash
# All unit + integration (default CI)
go test ./...

# HTTP e2e (boots server + miniredis)
go test ./test/e2e/ -v

# User stories only
go test ./test/e2e/ -run TestStory -v

# Live probes (requires etc/matrix.yaml keys + network)
go test ./test/e2e/ -tags=e2e -run TestLive -v

# Key verification script
bash scripts/verify_amap_keys.sh

# Key pool simulation
go run ./scripts/simulate_key_pool.go
go run ./scripts/simulate_key_pool.go -live   # calibrate from live good key

# Against Compose stack (app :8888, redis, mysql :3306)
docker compose -f docker-compose.dev.yml up -d --build
python3 scripts/scenario_cache_matrix.py
python3 scripts/capacity_timeout_stress.py
```

## Package coverage

| Package | Focus |
|---------|-------|
| `internal/cache` | Tenant/method isolation, fuzzy geo, timeslot |
| `internal/persist` | Memory archive, async upsert |
| `internal/engine` | Matrix fill, L1/L2, deadline, write-through |
| `internal/arccover` | Dense miss planner |
| `internal/handler` | HTTP codes, 504/429, health |
| `internal/planner` | Batch waypoints |
| `internal/provider` | Amap parsing, key rotation |
| `internal/loadbalance` | Selection formula, probe backoff |
| `internal/middleware` | Rate limit |
| `internal/geo` | GCJ-02 / WGS84, haversine |

## E2E HTTP tests (`test/e2e/matrix_test.go`)

Real go-zero server + miniredis + placeholder Amap key (fallback mode):

- Health live/ready
- Matrix/route fallback
- Cache retry consistency
- Invalid point rejection

## User story tests (`test/e2e/stories_test.go`)

Full stack via handler (stub provider + miniredis):

| ID | Story |
|----|-------|
| US-01 | Cold matrix symmetric |
| US-02 | Cache write-through, no duplicate provider calls |
| US-03 | 504 deadline + retry succeeds |
| US-04 | Method cache isolation |
| US-05 | Tenant Redis namespace isolation |
| US-06 | Rate limit 429 + independent tenants |
| US-07 | MaxPoints rejection |
| US-08 | Strict cache hit on retry |
| US-09 | Multi-waypoint route steps |
| US-10 | Unknown provider error |

Details: [User Stories](./user-stories.md)

## Manual HTTP (`test/api.http`)

Use with VS Code REST Client or IntelliJ HTTP Client. Covers happy paths and error scenarios.

## Live tests (`-tags=e2e`)

Opt-in — not run in default CI:

- `TestLiveAmapKeys` — probe each key in yaml
- `TestLiveMatrixWithProductionConfig` — hit running server on :8888

## CI recommendation

```yaml
- go test ./... -count=1 -timeout 5m
- go test ./test/e2e/ -v -count=1 -timeout 2m
# optional nightly:
- go test ./test/e2e/ -tags=e2e -run TestLive -v
```
