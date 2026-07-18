package arccover

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

func BenchmarkDensePlanner(b *testing.B) {
	cases := []struct {
		name string
		n    int
		req  []Arc
	}{
		{"rand_50_n200", 200, RandomMissRequired(200, 0.50, 1)},
		{"row_50_n200", 200, RowMissRequired(200, 0.50, 1)},
		{"complete_n100", 100, CompleteRequired(100)},
	}
	const L = 17
	p := NewDensePlanner(DefaultConfig())
	for _, c := range cases {
		req := NormalizeRequired(c.req)
		b.Run(c.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = p.Plan(context.Background(), c.n, append([]Arc(nil), req...), L)
			}
		})
	}
}

// TestDenseBenchmarkMatrix is the production Dense-only quality/latency matrix.
// Invoke: go test ./internal/arccover/ -run TestDenseBenchmarkMatrix -v -timeout 30m
func TestDenseBenchmarkMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("dense matrix is long-running")
	}
	const L = 17
	const repeats = 5
	p := NewDensePlanner(DefaultConfig())

	type scenario struct {
		name string
		n    int
		req  []Arc
	}
	scenarios := []scenario{
		{"complete_n100", 100, CompleteRequired(100)},
		{"complete_n500", 500, CompleteRequired(500)},
		{"rand_1pct", 500, RandomMissRequired(500, 0.01, 1)},
		{"rand_10pct", 500, RandomMissRequired(500, 0.10, 1)},
		{"rand_50pct", 500, RandomMissRequired(500, 0.50, 1)},
		{"rand_70pct", 500, RandomMissRequired(500, 0.70, 1)},
		{"rand_90pct", 500, RandomMissRequired(500, 0.90, 1)},
		{"rand_99pct", 500, RandomMissRequired(500, 0.99, 1)},
		{"row_10pct", 500, RowMissRequired(500, 0.10, 1)},
		{"row_50pct", 500, RowMissRequired(500, 0.50, 1)},
		{"outstar_n500", 500, OutStarRequired(500)},
		{"instar_n500", 500, InStarRequired(500)},
		{"path_n500", 500, PathRequired(500)},
		{"cycle_chord_n100", 100, CycleWithChordRequired(100)},
	}
	nMulti, eMulti := MultiComponentPaths(10, 15)
	scenarios = append(scenarios, scenario{"multicomp", nMulti, eMulti})

	out := os.Stdout
	fmt.Fprintf(out, "\n=== Dense production matrix (L=%d, repeats=%d) ===\n", L, repeats)

	for _, sc := range scenarios {
		req := NormalizeRequired(sc.req)
		lb := ComputeLowerBound(sc.n, req, L)
		m := len(req)
		fmt.Fprintf(out, "\n-- %s n=%d m=%d lb=%d --\n", sc.name, sc.n, m, lb)

		times := make([]float64, 0, repeats)
		var last Plan
		for r := 0; r < repeats; r++ {
			cp := append([]Arc(nil), req...)
			start := time.Now()
			var err error
			last, err = p.Plan(context.Background(), sc.n, cp, L)
			if err != nil {
				t.Fatalf("%s: %v", sc.name, err)
			}
			times = append(times, float64(time.Since(start).Microseconds())/1000.0)
		}
		last = ValidateAndComplete(last, req, L)
		bridges := 0
		for _, rt := range last.Routes {
			for _, leg := range rt.Legs {
				if leg.Kind == LegBridge {
					bridges++
				}
			}
		}
		sort.Float64s(times)
		med := times[len(times)/2]
		p95 := times[int(float64(len(times)-1)*0.95)]
		calls := len(last.Routes)
		cov := CountCoverage(last, req) == m
		fmt.Fprintf(out, "  Dense  calls=%6d gap=%5d bridges=%6d med=%8.2fms p95=%8.2fms cov=%v branch=%s\n",
			calls, calls-lb, bridges, med, p95, cov, last.Stats.Branch)
		if !cov {
			t.Errorf("%s incomplete coverage", sc.name)
		}
	}
}
