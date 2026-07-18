package arccover

import (
	"context"
	"time"
)

// DensePlanner is the production arc-cover planner (DenseRouteFirst + PackFragments).
type DensePlanner struct {
	Config Config
}

// NewDensePlanner returns a planner with defaults applied.
func NewDensePlanner(cfg Config) *DensePlanner {
	if cfg.DenseCandidateWindow <= 0 {
		cfg.DenseCandidateWindow = DefaultConfig().DenseCandidateWindow
	}
	if cfg.MaxLegs <= 0 {
		cfg.MaxLegs = 17
	}
	return &DensePlanner{Config: cfg}
}

// Plan implements ArcCoverPlanner.
func (p *DensePlanner) Plan(ctx context.Context, n int, required []Arc, maxLegs int) (Plan, error) {
	started := time.Now()
	if maxLegs <= 0 {
		maxLegs = p.Config.MaxLegs
	}
	if maxLegs <= 0 {
		maxLegs = 17
	}
	if len(required) == 0 {
		return Plan{}, nil
	}
	required = NormalizeRequired(required)

	lbStart := time.Now()
	lowerBound := ComputeLowerBound(n, required, maxLegs)
	lbRuntime := time.Since(lbStart)

	window := p.Config.DenseCandidateWindow
	if window <= 0 {
		window = 8
	}
	candStart := time.Now()
	plan := DensePlan(ctx, n, required, maxLegs, window)
	plan.Stats.LowerBound = lowerBound
	plan.Stats.Branch = "dense"
	if ctx != nil && ctx.Err() != nil {
		plan.Stats.Cancelled = true
	}
	plan.Stats.CandidateRuntime = time.Since(candStart)
	plan.Stats.LowerBoundRuntime = lbRuntime

	return FinalizePlan(plan, required, maxLegs, started), nil
}
