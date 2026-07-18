package arccover

import (
	"time"
)

// ValidateAndComplete verifies walk continuity and covers missing required edges
// with single-pair fallback routes.
func ValidateAndComplete(plan Plan, required []Arc, maxLegs int) Plan {
	covered := make([]bool, len(required))

	for routeIndex := range plan.Routes {
		route := &plan.Routes[routeIndex]
		if len(route.Legs) == 0 {
			continue
		}
		if len(route.Legs) > maxLegs {
			// Defensive: trim is wrong; fall back by replacing with single pairs.
			plan.Stats.SinglePairFallbacks += len(route.Legs)
			plan.Routes[routeIndex] = Route{}
			continue
		}
		for i, leg := range route.Legs {
			if i > 0 && route.Legs[i-1].To != leg.From {
				// Break into singles for the remainder — mark route invalid.
				plan.Routes[routeIndex] = Route{}
				break
			}
			if leg.Kind == LegRequired {
				id := int(leg.EdgeID)
				if id >= 0 && id < len(covered) {
					covered[id] = true
				}
			}
		}
	}

	// Drop empty routes produced by defensive paths.
	compact := plan.Routes[:0]
	for _, r := range plan.Routes {
		if len(r.Legs) > 0 {
			compact = append(compact, r)
		}
	}
	plan.Routes = compact

	for _, edge := range required {
		if covered[edge.ID] {
			continue
		}
		plan.Routes = append(plan.Routes, Route{
			Legs: []Leg{{
				From:   edge.From,
				To:     edge.To,
				Kind:   LegRequired,
				EdgeID: edge.ID,
			}},
		})
		plan.Stats.SinglePairFallbacks++
		covered[edge.ID] = true
	}
	return plan
}

// FinalizePlan recomputes stats after validation.
func FinalizePlan(plan Plan, required []Arc, maxLegs int, started time.Time) Plan {
	valStart := time.Now()
	plan = ValidateAndComplete(plan, required, maxLegs)
	plan.Stats.ValidationRuntime = time.Since(valStart)

	bridges := 0
	dup := 0
	seen := make([]bool, len(required))
	reqSet := NewRequiredSet(0, nil)
	n := 0
	for _, e := range required {
		if int(e.From)+1 > n {
			n = int(e.From) + 1
		}
		if int(e.To)+1 > n {
			n = int(e.To) + 1
		}
	}
	// Prefer max from vertices present; caller usually passes consistent n via graph.
	_ = reqSet
	reqSet = NewRequiredSet(maxInt(n, 1), required)

	for _, route := range plan.Routes {
		for _, leg := range route.Legs {
			switch leg.Kind {
			case LegBridge:
				bridges++
				if reqSet.Contains(leg.From, leg.To) {
					dup++
				}
			case LegRequired:
				id := int(leg.EdgeID)
				if id >= 0 && id < len(seen) {
					if seen[id] {
						dup++
					} else {
						seen[id] = true
					}
				}
			}
		}
	}

	plan.Stats.RequiredEdges = len(required)
	plan.Stats.ProviderCalls = len(plan.Routes)
	plan.Stats.BridgeEdges = bridges
	plan.Stats.DuplicateRequiredEdges = dup
	plan.Stats.PlannerRuntime = time.Since(started)
	if plan.Stats.LowerBound > 0 {
		plan.Stats.OptimalityGap = float64(plan.Stats.ProviderCalls-plan.Stats.LowerBound) /
			float64(plan.Stats.LowerBound)
		plan.Stats.Optimal = plan.Stats.ProviderCalls == plan.Stats.LowerBound
	}
	return plan
}

// CountCoverage returns how many required edges appear as LegRequired.
func CountCoverage(plan Plan, required []Arc) int {
	covered := make([]bool, len(required))
	for _, route := range plan.Routes {
		for _, leg := range route.Legs {
			if leg.Kind != LegRequired {
				continue
			}
			id := int(leg.EdgeID)
			if id >= 0 && id < len(covered) {
				covered[id] = true
			}
		}
	}
	n := 0
	for _, c := range covered {
		if c {
			n++
		}
	}
	return n
}
