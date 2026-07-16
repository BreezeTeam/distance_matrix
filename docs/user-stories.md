# User Stories (E2E Acceptance)

Each story maps to `TestStoryXX_*` in [`test/e2e/stories_test.go`](../test/e2e/stories_test.go).

## US-01 — Cold matrix

**As** a VRP optimizer  
**I want** an N×N distance matrix for N≥2 points  
**So that** I can run routing algorithms

**Acceptance**

- HTTP 200, diagonal zeros, positive off-diagonals
- Provider called on first request

---

## US-02 — Cache write-through

**As** a retrying client  
**I want** identical requests to hit Redis  
**So that** I don't waste Amap quota

**Acceptance**

- Second identical request returns same distance
- Provider call count unchanged
- Redis keys exist under tenant prefix

---

## US-03 — Deadline partial progress

**As** a client with tight SLA  
**I want** 504 on timeout with retry success  
**So that** large matrices complete incrementally

**Acceptance**

- First request: HTTP 504, `MATRIX_DEADLINE`
- Second request (no delay): HTTP 200

---

## US-04 — Method isolation

**As** a fleet with car and truck profiles  
**I want** separate cache by `method`  
**So that** truck distances don't contaminate car cache

**Acceptance**

- `method=0` then `method=1` triggers new provider call

---

## US-05 — Tenant isolation

**As** a platform operator  
**I want** per-tenant cache namespaces  
**So that** customers don't share edge data

**Acceptance**

- Separate Redis key prefixes for tenant-a / tenant-b
- tenant-a retry hits cache (no new provider calls)

---

## US-06 — Rate limiting

**As** ops  
**I want** per-tenant QPS caps on matrix  
**So that** one tenant can't exhaust Amap quota

**Acceptance**

- Burst > `TenantQPS` → 429 `RATE_LIMIT`
- Different tenant unaffected

---

## US-07 — Max points guard

**As** API consumer  
**I want** clear rejection when N > MaxPoints  
**So that** I don't hang the service

**Acceptance**

- 6 points with MaxPoints=5 → HTTP error (not 200)

---

## US-08 — Strict cache retry

**As** optimizer with exact coordinates  
**I want** strict mode to reuse cached edges  
**So that** retries are deterministic

**Acceptance**

- Stub returns distance=1000
- Retry without additional provider calls

---

## US-09 — Multi-waypoint route

**As** dispatch UI  
**I want** K-point route with K−1 legs  
**So that** I show segment breakdown

**Acceptance**

- 4 points → 3 steps, total distance > 0

---

## US-10 — Invalid provider

**As** API consumer  
**I want** error on unknown provider  
**So that** I fail fast

**Acceptance**

- `provider: "nonexistent"` → non-200

---

## Future stories (unit-tested, not yet HTTP e2e)

| Story | Unit location |
|-------|---------------|
| Strict vs reverse reuse | `engine_test.go` |
| Fuzzy geo_wide hit | `cache/store_test.go` |
| Timeslot ±1h | `cache/timeslot_test.go` |
| WGS84 coordinates | `geo/coord_test.go` |
| 504 partial Redis keys | `engine_test.go` |

These are candidates for promotion to HTTP e2e when stub timing allows partial writes.
