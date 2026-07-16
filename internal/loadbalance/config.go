package loadbalance

import "math"

// SchedulerConfig tunes the Adaptive Decaying Confidence Scheduler.
type SchedulerConfig struct {
	PriorSuccessRate  float64 // π
	MinProbeRate      float64 // ρ_min
	BetaPriorA        float64 // Beta(a,b) → ρ = (S+a)/(S+F+a+b)
	BetaPriorB        float64
	ConfidenceTau     float64 // τ in α = 1 - exp(-η/τ)
	FailureSoftWeight float64 // w_f in η = S + w_f·F
	ProbeFailureScale float64 // φ in T(F_b)
	EpsilonScale      float64 // ε = exp(-N/k); k = EpsilonScale (default 4)

	// UseBeta false → legacy ρ = S/(S+F) (or π when empty).
	UseBeta bool
	// ProbeOnlyHardFail: S=0 and F_hard≥1 → probe-only (R≈0 when gate closed).
	ProbeOnlyHardFail bool
	// MultiKeyEpsilon: when false, ε=0 for N>1 (exploration via g only).
	MultiKeyEpsilon bool
}

// DefaultSchedulerConfig is tuned for Amap API key pools (QPS throttle + dead keys).
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		PriorSuccessRate:  1.0,
		MinProbeRate:      0.02,
		BetaPriorA:        2.0,
		BetaPriorB:        1.0,
		ConfidenceTau:     2.0,
		FailureSoftWeight: 0.3,
		ProbeFailureScale: 5.0,
		EpsilonScale:      4.0,
		UseBeta:           true,
		ProbeOnlyHardFail: true,
		MultiKeyEpsilon:   false,
	}
}

// LegacyMinProbePreset approximates the pre-v2 equal min-probe heuristic.
func LegacyMinProbePreset() SchedulerConfig {
	return SchedulerConfig{
		PriorSuccessRate:  1.0,
		MinProbeRate:      0.33,
		BetaPriorA:        1.0,
		BetaPriorB:        1.0,
		ConfidenceTau:     0.5,
		FailureSoftWeight: 1.0,
		ProbeFailureScale: 5.0,
		EpsilonScale:      1.0,
		UseBeta:           false,
		ProbeOnlyHardFail: false,
		MultiKeyEpsilon:   true,
	}
}

// ZeroedPreset (v3): hard-fail keys get zero weight — no probe recovery.
func ZeroedPreset() SchedulerConfig {
	c := DefaultSchedulerConfig()
	c.ProbeOnlyHardFail = true
	c.MinProbeRate = 0
	c.MultiKeyEpsilon = false
	return c
}

func (c SchedulerConfig) posteriorRho(s, soft, hard float64) float64 {
	f := soft + hard
	if !c.UseBeta {
		if s+f <= 0 {
			return c.PriorSuccessRate
		}
		return s / (s + f)
	}
	return (s + c.BetaPriorA) / (s + f + c.BetaPriorA + c.BetaPriorB)
}

func (c SchedulerConfig) weightedSampleMass(s, soft, hard float64) float64 {
	return s + c.FailureSoftWeight*(soft+hard)
}

func (c SchedulerConfig) poolSafetyFactor(nItems int) float64 {
	if nItems <= 0 {
		return 0
	}
	k := c.EpsilonScale
	if k <= 0 {
		k = 4.0
	}
	eps := math.Exp(-float64(nItems) / k)
	if !c.MultiKeyEpsilon && nItems > 1 {
		return 0
	}
	return eps
}
