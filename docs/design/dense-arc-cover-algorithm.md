# Dense Arc Cover Planner — production algorithm

**Package:** `internal/arccover`  
**Production algorithm:** `DensePlanner` (`DensePlan` = DenseRouteFirst + PackFragments)  
**Status:** sole production planner

---

## 1. Problem

Cover directed OD edges missing from Redis while minimizing provider calls (each walk ≤ `L` legs).

Lexicographic objective: calls → bridges → duplicates.

In the engine, `L = Providers.amap.BatchSize - 1` (default BatchSize 12 → **L = 11**). Package `Config.MaxLegs` (default 17) is only a fallback when the caller does not pass `maxLegs`.

---

## 2. Dense algorithm (two phases)

```text
DensePlan
  1. DenseRouteFirst  — greedily grow required-only fragments on the required graph (windowed successor pick)
  2. PackFragments    — best-fit pack fragments into capacity L, insert bridges when needed
```

- Deterministic: same input + config → same output
- Entry: `NewDensePlanner(DefaultConfig()).Plan(...)`
- Wired from `internal/engine` after Redis (and optional MySQL L2) miss probe

---

## 3. Package layout

| File | Role |
|------|------|
| `planner.go` | production entry `DensePlanner` |
| `dense.go` | `DenseRouteFirst` / `DensePlan` |
| `packing.go` | `PackFragments` |
| `graph.go` | mutable graph + successor selection |
| `types.go` | `Arc` / `Plan` / `Config` / interface |
| `required_set.go` | required-edge normalization / sets |
| `lower_bound.go` / `trail.go` | combo LB / τ |
| `validate.go` | validation and completion |
| `util.go` | small helpers |
| `scenario.go` | test scenario generation |

---

## 4. Config

```go
type Config struct {
    MaxLegs              int // default 17 (engine usually overrides via BatchSize-1)
    DenseCandidateWindow int // default 8
}
```

---

## 5. Benchmark

```bash
go test ./internal/arccover/ -run TestDenseBenchmarkMatrix -v -timeout 30m
```
