# Matrix v2 Completion & Cleanup Plan

**Goal:** Finish v2 enterprise matrix service, remove absorbed Python pilot and legacy Go SDK, leave one clean codebase.

**Architecture:** `internal/{cache,provider,planner,engine}` + thin go-zero handlers.

---

## Phase 1 — Cleanup ✅

- [x] Delete nested Python package `distance_matrix/`
- [x] Remove legacy Go SDK: `distance`, `restapi_amap_com`, `baidu`, `base`, `service.go`, `schema.go`, `interface.go`, `routerunner.go`
- [x] Move geo helpers to `internal/geo`, load balancer to `internal/loadbalance`
- [x] Remove legacy `sdk/`, `common/`, `internal/logic/`, `pkg/` (duplicates of `internal/*`)
- [x] Drop legacy `/api/*` routes; canonical API is `/v1/*`
- [x] Update README

## Phase 2 — Complete v2 features ✅

- [x] Align API contract in `api/service.api`
- [x] `docker-compose.yml` (redis + matrix)
- [x] Prometheus metrics (`internal/metrics`)
- [x] Tenant rate limit (`internal/middleware`, `Engine.TenantQPS`)
- [x] E2E tests (`test/e2e/`) — local stack + live Amap key probe (`-tags=e2e`)

## Phase 3 — Hardening (remaining)

- [x] Load test script (`scripts/load_matrix.sh`)
- [x] Key pool simulation (`scripts/simulate_key_pool.go`)
- [ ] Regenerate OpenAPI from `api/service.api` if needed
- [ ] Production runbook (Amap quota vs design §11 capacity)

## Phase 4 — Done criteria

- [x] `go build ./...` && `go test ./internal/...` pass
- [x] No Python subtree; single matrix code path
- [x] Single config story in README

---

## Operator checklist

1. `docker compose up -d redis && go run matrix.go -f etc/matrix.yaml`
2. Cold + retry curl against `/v1/matrix`
3. `bash scripts/verify_amap_keys.sh` — remove permanently invalid keys from config
4. `go run ./scripts/simulate_key_pool.go -live` — validate scheduler tuning
5. Optional: `scripts/load_matrix.sh` before production cutover
