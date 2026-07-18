# Dense Arc Cover Planner — production algorithm

**Package:** `internal/arccover`  
**Production algorithm:** `DensePlanner` (`DensePlan` = DenseRouteFirst + PackFragments)  
**Status:** sole production planner

---

## 1. Problem

Cover directed OD edges missing from Redis while minimizing provider calls (each walk ≤ `L` legs, default 17).

Lexicographic objective: calls → bridges → duplicates.

---

## 2. Dense algorithm (two clean phases)

```text
DensePlan
  1. DenseRouteFirst  — greedily grow required-only fragments on the required graph (windowed successor pick)
  2. PackFragments    — best-fit pack fragments into capacity L, insert bridges when needed
```

- Deterministic: same input + config → same output
- Entry: `NewDensePlanner(DefaultConfig()).Plan(...)`

---

## 3. Package layout

| File | Role |
|------|------|
| `planner.go` | production entry `DensePlanner` |
| `dense.go` | `DenseRouteFirst` / `DensePlan` |
| `packing.go` | `PackFragments` |
| `graph.go` | mutable graph + successor selection |
| `lower_bound.go` / `trail.go` | combo LB / τ |
| `validate.go` | validation and completion |
| `scenario.go` | test scenario generation |

---

## 4. Config

```go
type Config struct {
    MaxLegs              int // default 17
    DenseCandidateWindow int // default 8
}
```

---

## 5. Benchmark

```bash
go test ./internal/arccover/ -run TestDenseBenchmarkMatrix -v -timeout 30m
```
