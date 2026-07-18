package arccover

import (
	"context"
	"testing"
	"time"
)

func TestDensePlannerCompleteHitsLB(t *testing.T) {
	n := 40
	req := NormalizeRequired(CompleteRequired(n))
	L := 17
	plan, err := NewDensePlanner(DefaultConfig()).Plan(context.Background(), n, req, L)
	if err != nil {
		t.Fatal(err)
	}
	if CountCoverage(plan, req) != len(req) {
		t.Fatal("incomplete")
	}
	lb := ComputeLowerBound(n, req, L)
	if len(plan.Routes) != lb {
		t.Fatalf("calls=%d lb=%d", len(plan.Routes), lb)
	}
	if plan.Stats.Branch != "dense" {
		t.Fatalf("branch=%q", plan.Stats.Branch)
	}
}

func TestDensePlannerPathHitsLB(t *testing.T) {
	n := 50
	req := NormalizeRequired(PathRequired(n))
	L := 17
	plan, err := NewDensePlanner(DefaultConfig()).Plan(context.Background(), n, req, L)
	if err != nil {
		t.Fatal(err)
	}
	lb := ComputeLowerBound(n, req, L)
	if len(plan.Routes) != lb {
		t.Fatalf("calls=%d lb=%d", len(plan.Routes), lb)
	}
}

func TestDensePlannerOutStarHitsLB(t *testing.T) {
	n := 80
	req := NormalizeRequired(OutStarRequired(n))
	L := 17
	plan, err := NewDensePlanner(DefaultConfig()).Plan(context.Background(), n, req, L)
	if err != nil {
		t.Fatal(err)
	}
	lb := ComputeLowerBound(n, req, L)
	if len(plan.Routes) != lb {
		t.Fatalf("calls=%d lb=%d", len(plan.Routes), lb)
	}
}

func TestDensePlannerCoverageRandom(t *testing.T) {
	n := 60
	req := NormalizeRequired(RandomMissRequired(n, 0.4, 3))
	L := 17
	plan, err := NewDensePlanner(DefaultConfig()).Plan(context.Background(), n, req, L)
	if err != nil {
		t.Fatal(err)
	}
	if CountCoverage(plan, req) != len(req) {
		t.Fatal("incomplete")
	}
	lb := ComputeLowerBound(n, req, L)
	if len(plan.Routes) < lb {
		t.Fatalf("calls=%d < lb=%d", len(plan.Routes), lb)
	}
}

func TestDensePlannerDeterministic(t *testing.T) {
	n := 50
	req := NormalizeRequired(RandomMissRequired(n, 0.35, 11))
	L := 17
	p := NewDensePlanner(DefaultConfig())
	a, _ := p.Plan(context.Background(), n, req, L)
	b, _ := p.Plan(context.Background(), n, req, L)
	if len(a.Routes) != len(b.Routes) {
		t.Fatal("nondeterministic calls")
	}
	for i := range a.Routes {
		if len(a.Routes[i].Legs) != len(b.Routes[i].Legs) {
			t.Fatalf("route %d", i)
		}
		for j := range a.Routes[i].Legs {
			if a.Routes[i].Legs[j] != b.Routes[i].Legs[j] {
				t.Fatal("leg mismatch")
			}
		}
	}
}

func TestMinimumTrailCountShapes(t *testing.T) {
	if got := MinimumTrailCount(5, PathRequired(5)); got != 1 {
		t.Fatalf("path τ=%d", got)
	}
	if got := MinimumTrailCount(5, CycleRequired(5)); got != 1 {
		t.Fatalf("cycle τ=%d", got)
	}
	if got := MinimumTrailCount(6, OutStarRequired(6)); got != 5 {
		t.Fatalf("outstar τ=%d", got)
	}
}

func TestFinalizePlanStats(t *testing.T) {
	n := 20
	req := NormalizeRequired(PathRequired(n))
	L := 17
	plan := DensePlan(context.Background(), n, req, L, 8)
	plan.Stats.LowerBound = ComputeLowerBound(n, req, L)
	plan = FinalizePlan(plan, req, L, time.Now())
	if plan.Stats.ProviderCalls != len(plan.Routes) {
		t.Fatal("calls mismatch")
	}
	if CountCoverage(plan, req) != len(req) {
		t.Fatal("incomplete")
	}
}
