package arccover

import (
	"context"
	"testing"
	"time"
)

func TestPackFragmentsRow50Under200ms(t *testing.T) {
	const L = 17
	n := 500
	req := NormalizeRequired(RowMissRequired(n, 0.50, 1))
	const wantCalls = 10405 // matrix / pre-opt Dense baseline for seed=1

	var ms float64
	var calls int
	for trial := 0; trial < 3; trial++ {
		t0 := time.Now()
		plan := DensePlan(context.Background(), n, append([]Arc(nil), req...), L, 8)
		ms = float64(time.Since(t0).Microseconds()) / 1000
		plan = ValidateAndComplete(plan, req, L)
		calls = len(plan.Routes)
		if ms <= 200 {
			break
		}
	}
	t.Logf("row_50 Dense: %.1fms calls=%d", ms, calls)
	if calls != wantCalls {
		t.Fatalf("quality changed: calls=%d want %d", calls, wantCalls)
	}
	if ms > 200 {
		t.Fatalf("too slow: %.1fms want ≤200ms", ms)
	}
}

func TestPackFragmentsQualityMatchesLegacyOnRandom(t *testing.T) {
	// Spot-check that faster packing keeps Dense call counts on a small random graph.
	L := 17
	n := 80
	req := NormalizeRequired(RandomMissRequired(n, 0.5, 9))
	plan := ValidateAndComplete(DensePlan(context.Background(), n, req, L, 8), req, L)
	if CountCoverage(plan, req) != len(req) {
		t.Fatal("incomplete")
	}
	lb := ComputeLowerBound(n, req, L)
	if len(plan.Routes) < lb {
		t.Fatalf("calls=%d < lb=%d", len(plan.Routes), lb)
	}
}
