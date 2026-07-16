package loadbalance

import (
	"math"
	"sync"
	"time"
)

const (
	defaultRecoveryHalfLife = 5 * time.Minute
	defaultProbeBase        = 30 * time.Second
	fullWeightScale         = 1000
)

// Pool selects items by smooth weighted round-robin.
type Pool struct {
	mu                sync.Mutex
	cfg               SchedulerConfig
	recoveryHalfLife  time.Duration
	probeBaseInterval time.Duration
	nowFn             func() time.Time
	items             []*poolItem
	byKey             map[interface{}]*poolItem
}

type poolItem struct {
	item            interface{}
	baseWeight      int
	successes       float64
	failuresSoft    float64 // failure while successes > 0 (e.g. QPS throttle)
	failuresHard    float64 // failure with no success history (likely dead key)
	lastEventAt    time.Time // decay reference clock
	lastOutcomeAt  time.Time // last success/failure — drives probe gate g(Δt)
	currentWeight  int
	effectiveWeight int
}

// NewPool creates a pool with default scheduler tuning.
func NewPool(recoveryHalfLife, probeBaseInterval time.Duration) *Pool {
	return NewPoolWithConfig(recoveryHalfLife, probeBaseInterval, DefaultSchedulerConfig())
}

// NewPoolWithConfig creates a pool with explicit scheduler parameters.
func NewPoolWithConfig(recoveryHalfLife, probeBaseInterval time.Duration, cfg SchedulerConfig) *Pool {
	if recoveryHalfLife <= 0 {
		recoveryHalfLife = defaultRecoveryHalfLife
	}
	if probeBaseInterval < 0 {
		probeBaseInterval = defaultProbeBase
	}
	return &Pool{
		cfg:               cfg,
		recoveryHalfLife:  recoveryHalfLife,
		probeBaseInterval: probeBaseInterval,
		byKey:             make(map[interface{}]*poolItem),
	}
}

// SetVirtualNow pins wall time for simulations; pass nil to use time.Now().
func (p *Pool) SetVirtualNow(fn func() time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nowFn = fn
}

func (p *Pool) now() time.Time {
	if p.nowFn != nil {
		return p.nowFn()
	}
	return time.Now()
}

func (p *Pool) Add(item interface{}, weight int) {
	if weight <= 0 {
		weight = 1
	}
	s := &poolItem{item: item, baseWeight: weight}
	p.items = append(p.items, s)
	p.byKey[item] = s
}

func (p *Pool) RemoveAll() {
	p.items = p.items[:0]
	p.byKey = make(map[interface{}]*poolItem)
}

func (p *Pool) Len() int { return len(p.items) }

func (p *Pool) All() map[interface{}]int {
	m := make(map[interface{}]int, len(p.items))
	for _, s := range p.items {
		m[s.item] = s.baseWeight
	}
	return m
}

// RecordSuccess notes a successful outcome for item.
func (p *Pool) RecordSuccess(item interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.byKey[item]
	if !ok {
		return
	}
	now := p.now()
	p.applyDecay(s, now)
	s.successes++
	s.lastEventAt = now
	s.lastOutcomeAt = now
}

// RecordFailure notes a failed outcome (any error type counts the same).
// Failures after at least one success in the decay window are soft; others are hard.
func (p *Pool) RecordFailure(item interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.byKey[item]
	if !ok {
		return
	}
	now := p.now()
	p.applyDecay(s, now)
	if s.successes > 1e-9 {
		s.failuresSoft++
	} else {
		s.failuresHard++
	}
	s.lastEventAt = now
	s.lastOutcomeAt = now
}

func (p *Pool) Next() interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.items) == 0 {
		return nil
	}
	now := p.now()
	for _, s := range p.items {
		p.applyDecay(s, now)
		s.effectiveWeight = p.selectionWeight(s, now)
	}
	best := nextSmoothWeighted(p.items)
	if best == nil {
		return nil
	}
	return best.item
}

func (p *Pool) applyDecay(s *poolItem, now time.Time) {
	if s.lastEventAt.IsZero() {
		return
	}
	elapsed := now.Sub(s.lastEventAt)
	if elapsed <= 0 {
		return
	}
	d := math.Exp(-math.Ln2 * elapsed.Seconds() / p.recoveryHalfLife.Seconds())
	s.successes *= d
	s.failuresSoft *= d
	s.failuresHard *= d
	s.successes = math.Max(0, s.successes)
	s.failuresSoft = math.Max(0, s.failuresSoft)
	s.failuresHard = math.Max(0, s.failuresHard)
	s.lastEventAt = now
}

func probeGate(sinceSelect time.Duration, backoffFailures float64, probeBase time.Duration, phi float64) float64 {
	if probeBase <= 0 {
		return 1
	}
	if sinceSelect < 0 {
		sinceSelect = 0
	}
	if phi <= 0 {
		phi = 5.0
	}
	intervalSec := probeBase.Seconds() * math.Exp(backoffFailures/phi)
	if intervalSec <= 0 {
		return 1
	}
	return 1 - math.Exp(-sinceSelect.Seconds()/intervalSec)
}

func sinceLastOutcome(s *poolItem, now time.Time) time.Duration {
	if s.lastOutcomeAt.IsZero() {
		return time.Duration(math.MaxInt64)
	}
	return now.Sub(s.lastOutcomeAt)
}

func (p *Pool) effectiveRate(s *poolItem, sinceSelect time.Duration, nItems int) float64 {
	cfg := p.cfg
	rho := cfg.posteriorRho(s.successes, s.failuresSoft, s.failuresHard)
	nEff := cfg.weightedSampleMass(s.successes, s.failuresSoft, s.failuresHard)
	tau := cfg.ConfidenceTau
	if tau <= 0 {
		tau = 2.0
	}
	alpha := 1 - math.Exp(-nEff/tau)
	shrink := (1-alpha)*cfg.PriorSuccessRate + alpha*rho
	fb := s.failuresHard + cfg.FailureSoftWeight*s.failuresSoft
	gate := probeGate(sinceSelect, fb, p.probeBaseInterval, cfg.ProbeFailureScale)
	gPrime := gate + (1-gate)*cfg.poolSafetyFactor(nItems)

	if cfg.ProbeOnlyHardFail && s.successes <= 1e-9 && s.failuresHard >= 1.0 {
		if gPrime <= 0 || cfg.MinProbeRate <= 0 {
			return 0
		}
		return cfg.MinProbeRate * gPrime * (1 - shrink)
	}
	return shrink + cfg.MinProbeRate*gPrime*(1-shrink)
}

func (p *Pool) selectionWeight(s *poolItem, now time.Time) int {
	rate := p.effectiveRate(s, sinceLastOutcome(s, now), len(p.items))
	return int(math.Round(float64(s.baseWeight) * rate * fullWeightScale))
}

func nextSmoothWeighted(items []*poolItem) *poolItem {
	var best *poolItem
	total := 0
	for _, s := range items {
		if s == nil || s.effectiveWeight <= 0 {
			continue
		}
		s.currentWeight += s.effectiveWeight
		total += s.effectiveWeight
		if best == nil || s.currentWeight > best.currentWeight {
			best = s
		}
	}
	if best == nil {
		return nil
	}
	best.currentWeight -= total
	return best
}

type SmoothWeighted = Pool

func (p *Pool) ReduceWeight(item interface{}) { p.RecordFailure(item) }

func (p *Pool) ActiveItems() []interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	out := make([]interface{}, 0, len(p.items))
	for _, s := range p.items {
		p.applyDecay(s, now)
		if p.selectionWeight(s, now) > 0 {
			out = append(out, s.item)
		}
	}
	return out
}

func (p *Pool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.items {
		s.successes = 0
		s.failuresSoft = 0
		s.failuresHard = 0
		s.currentWeight = 0
		s.lastEventAt = time.Time{}
		s.lastOutcomeAt = time.Time{}
	}
}

// SuccessRate returns Beta posterior ρ.
func (p *Pool) SuccessRate(item interface{}) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.byKey[item]
	if !ok {
		return 0
	}
	return p.cfg.posteriorRho(s.successes, s.failuresSoft, s.failuresHard)
}

// EffectiveRate exposes R at the current clock (applies outcome decay first).
func (p *Pool) EffectiveRate(item interface{}) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.byKey[item]
	if !ok {
		return 0
	}
	now := p.now()
	p.applyDecay(s, now)
	return p.effectiveRate(s, sinceLastOutcome(s, now), len(p.items))
}

// EffectiveRateAfter exposes R as if Δt passed since the last selection.
func (p *Pool) EffectiveRateAfter(item interface{}, sinceSelect time.Duration) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.byKey[item]
	if !ok {
		return 0
	}
	p.applyDecay(s, p.now())
	return p.effectiveRate(s, sinceSelect, len(p.items))
}

// Config returns a copy of the scheduler configuration.
func (p *Pool) Config() SchedulerConfig { return p.cfg }
