package loadbalance

import (
	"math"
	"testing"
	"time"
)

func testPool(halfLife time.Duration) *Pool {
	return NewPool(halfLife, 30*time.Second)
}

func fixedPool(halfLife time.Duration) (*Pool, *time.Time) {
	t0 := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	now := t0
	p := NewPool(halfLife, 30*time.Second)
	p.SetVirtualNow(func() time.Time { return now })
	return p, &now
}

func tickPool(p *Pool, now *time.Time, d time.Duration) {
	*now = now.Add(d)
}

func testEffectiveRate(s, fSoft, fHard float64, since time.Duration, nItems int) float64 {
	p := NewPool(time.Hour, 30*time.Second)
	item := &poolItem{successes: s, failuresSoft: fSoft, failuresHard: fHard, baseWeight: 1}
	return p.effectiveRate(item, since, nItems)
}

func TestEffectiveRateFormula(t *testing.T) {
	cases := []struct {
		s, fSoft, fHard float64
		since           time.Duration
		wantMin, wantMax float64
	}{
		{0, 0, 0, time.Hour, 0.95, 1.0},                         // unexplored, gate open
		{3, 7, 0, 0, 0.30, 0.55},                                // mixed soft, gate closed → R≈R₀
		{0, 0, 10, 0, 0, 0.01},                                  // dead probe-only, gate closed
		{0, 0, 10, 200 * time.Second, 0.005, 0.012},             // dead, gate open → ρ_min
	}
	for _, c := range cases {
		r := testEffectiveRate(c.s, c.fSoft, c.fHard, c.since, 2)
		if r < c.wantMin || r > c.wantMax {
			t.Fatalf("effectiveRate(%v,%v,%v,since=%v)=%v want [%v,%v]", c.s, c.fSoft, c.fHard, c.since, r, c.wantMin, c.wantMax)
		}
	}
}

func TestSoftVsHardFailure(t *testing.T) {
	p, _ := fixedPool(time.Hour)
	p.Add("k", 1)
	p.RecordSuccess("k")
	for i := 0; i < 5; i++ {
		p.RecordFailure("k")
	}
	s := p.byKey["k"]
	if s.failuresSoft != 5 || s.failuresHard != 0 {
		t.Fatalf("expected soft failures after success, got soft=%v hard=%v", s.failuresSoft, s.failuresHard)
	}
	p2, _ := fixedPool(time.Hour)
	p2.Add("dead", 1)
	for i := 0; i < 5; i++ {
		p2.RecordFailure("dead")
	}
	d := p2.byKey["dead"]
	if d.failuresHard != 5 || d.failuresSoft != 0 {
		t.Fatalf("expected hard failures without success, got soft=%v hard=%v", d.failuresSoft, d.failuresHard)
	}
}

func TestOccasionalSuccessVsAlwaysFail(t *testing.T) {
	p, now := fixedPool(time.Hour)
	p.Add("good", 1)
	p.Add("dead", 1)

	for i := 0; i < 3; i++ {
		p.RecordSuccess("good")
	}
	for i := 0; i < 7; i++ {
		p.RecordFailure("good")
	}
	for i := 0; i < 10; i++ {
		p.RecordFailure("dead")
	}

	tickPool(p, now, 200*time.Second) // open probe gate once; avoid long decay
	goodN, deadN := 0, 0
	for i := 0; i < 3000; i++ {
		switch k, _ := p.Next().(string); k {
		case "good":
			goodN++
		case "dead":
			deadN++
		}
	}
	goodShare := float64(goodN) / 30
	deadShare := float64(deadN) / 30

	if goodShare <= 0 {
		t.Fatalf("occasionally-successful key should keep weight, got %.1f%%", goodShare)
	}
	if deadShare < 0.5 || deadShare > 15 {
		t.Fatalf("always-fail key should get minimal probe share (~2%%), got %.1f%%", deadShare)
	}
	if goodShare < 75 {
		t.Fatalf("good key ~30%% rate should dominate, got %.1f%%", goodShare)
	}
}

func TestUnexploredKeyGetsFullWeight(t *testing.T) {
	p := testPool(time.Hour)
	p.Add("new", 1)
	p.Add("dead", 1)
	for i := 0; i < 5; i++ {
		p.RecordFailure("dead")
	}
	share := selectionShare(t, p, "new", 2000)
	if share < 85 {
		t.Fatalf("unexplored key should dominate over dead key, got %.1f%%", share)
	}
}

func TestTimeRecoveryRestoresDeadKey(t *testing.T) {
	p, now := fixedPool(300 * time.Second)
	p.Add("k", 1)
	for i := 0; i < 10; i++ {
		p.RecordFailure("k")
	}
	closed := p.EffectiveRate("k")
	tickPool(p, now, 1800*time.Second) // ~6 outcome half-lives → counters fade, key re-enters exploration
	recovered := p.EffectiveRate("k")
	if recovered < 0.9 {
		t.Fatalf("decay should recover dead key toward full weight: closed=%f recovered=%f", closed, recovered)
	}
}

func TestProbeGateCurve(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	T := 30 * time.Second.Seconds() * math.Exp(3.0/cfg.ProbeFailureScale)

	g0 := probeGate(0, 3.0, 30*time.Second, cfg.ProbeFailureScale)
	gMid := probeGate(time.Duration(T/3)*time.Second, 3.0, 30*time.Second, cfg.ProbeFailureScale)
	gFull := probeGate(time.Duration(T)*time.Second, 3.0, 30*time.Second, cfg.ProbeFailureScale)

	if g0 > 0.01 {
		t.Fatalf("g(0) should be ~0, got %f", g0)
	}
	if gMid > 0.35 {
		t.Fatalf("g(T/3) should stay low, got %f", gMid)
	}
	if gFull < 0.62 {
		t.Fatalf("g(T) should be ~1-exp(-1)≈0.63, got %f", gFull)
	}
}

func TestProbeBackoffEffectiveRate(t *testing.T) {
	p := NewPool(time.Hour, 30*time.Second)
	p.Add("dead", 1)
	p.Add("good", 1)
	for i := 0; i < 10; i++ {
		p.RecordFailure("dead")
	}

	rClosed := p.EffectiveRateAfter("dead", 0)
	rOpen := p.EffectiveRateAfter("dead", 200*time.Second)
	if rClosed >= rOpen {
		t.Fatalf("gate should open with time: closed=%f open=%f", rClosed, rOpen)
	}
	if rOpen < 0.005 {
		t.Fatalf("opened rate should approach ρ_min: got %f", rOpen)
	}
}

func TestSingleKeyNeverStopsProbing(t *testing.T) {
	p := testPool(time.Hour)
	p.Add("only", 1)
	for i := 0; i < 20; i++ {
		p.RecordFailure("only")
	}
	if selectionShare(t, p, "only", 500) == 0 {
		t.Fatal("single key must never drop to zero selection weight")
	}
}

func TestAllDeadKeysStillProbe(t *testing.T) {
	p, now := fixedPool(time.Hour)
	p.Add("a", 1)
	p.Add("b", 1)
	p.Add("c", 1)
	for _, k := range []string{"a", "b", "c"} {
		for i := 0; i < 5; i++ {
			p.RecordFailure(k)
		}
	}
	tickPool(p, now, 200*time.Second)
	seen := map[string]bool{}
	for i := 0; i < 3000; i++ {
		if k, _ := p.Next().(string); k != "" {
			seen[k] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("each dead key should be probed at least once, seen=%v", seen)
	}
}

func TestRecordSuccessImprovesPosterior(t *testing.T) {
	p, _ := fixedPool(time.Hour)
	p.Add("k", 1)
	for i := 0; i < 5; i++ {
		p.RecordFailure("k")
	}
	before := p.SuccessRate("k")
	p.RecordSuccess("k")
	p.RecordSuccess("k")
	after := p.SuccessRate("k")
	if after <= before {
		t.Fatalf("success should improve posterior ρ: before=%f after=%f", before, after)
	}
}

func TestOptimisticBetaPrior(t *testing.T) {
	p, _ := fixedPool(time.Hour)
	p.Add("new", 1)
	rho := p.SuccessRate("new")
	cfg := DefaultSchedulerConfig()
	want := cfg.BetaPriorA / (cfg.BetaPriorA + cfg.BetaPriorB)
	if math.Abs(rho-want) > 1e-9 {
		t.Fatalf("unexplored ρ=%f want %f", rho, want)
	}
}

func selectionShare(t *testing.T, p *Pool, key string, rounds int) float64 {
	t.Helper()
	return selectionShareWithTick(t, p, key, rounds, 0, nil)
}

func selectionShareWithTick(t *testing.T, p *Pool, key string, rounds int, tick time.Duration, now *time.Time) float64 {
	t.Helper()
	counts := 0
	for i := 0; i < rounds; i++ {
		if now != nil && tick > 0 {
			*now = now.Add(tick)
		}
		if k, _ := p.Next().(string); k == key {
			counts++
		}
	}
	return float64(counts) / float64(rounds) * 100
}
