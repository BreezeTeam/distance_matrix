package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"distance-matrix/internal/arccover"
	"distance-matrix/internal/cache"
	"distance-matrix/internal/geo"
	"distance-matrix/internal/persist"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/provider"
)

var ErrDeadline = errors.New("MATRIX_DEADLINE")

// Request is matrix computation input.
type Request struct {
	Points     [][2]float32
	Coordinate string
	Strategy   int
	Method     int
	TimeSlot   string
	Strict     bool
	GeoWideM   int
	Provider   string
	SpeedMPS   int
	Tenant     string
}

// Result holds distance/duration matrices.
type Result struct {
	Distances [][]float32
	Durations [][]float32
	Stats     Stats
}

// Stats is internal observability (logs/metrics only).
type Stats struct {
	CacheHits     int
	ColdHits      int // MySQL L2 hits (promoted to Redis)
	CacheMisses   int
	ProviderCalls int
	FallbackEdges int
	ArcCoverCalls int // DensePlanner route count (= planned provider walks)
	ElapsedMs     int64
	CacheHitRatio float64
}

// Engine computes OD matrices.
type Engine struct {
	Cache     *cache.Store
	Archive   persist.Archive // nil = L2 off
	Async     *persist.AsyncWriter
	Planner   *planner.Planner
	Registry  *provider.Registry
	ArcCover  arccover.ArcCoverPlanner
	MaxPoints int
}

func (e *Engine) arcCover() arccover.ArcCoverPlanner {
	if e.ArcCover != nil {
		return e.ArcCover
	}
	return arccover.NewDensePlanner(arccover.DefaultConfig())
}

func (e *Engine) maxLegs() int {
	if e.Planner != nil && e.Planner.BatchSize > 1 {
		return e.Planner.BatchSize - 1
	}
	return 11
}

// Compute fills distance/duration matrices.
// Flow: cache-probe all OD pairs → DensePlanner over misses → execute walks → fill matrix.
func (e *Engine) Compute(ctx context.Context, req Request) (*Result, error) {
	start := time.Now()
	if e.MaxPoints > 0 && len(req.Points) > e.MaxPoints {
		return nil, fmt.Errorf("too many points: max %d", e.MaxPoints)
	}
	if len(req.Points) < 2 {
		return nil, fmt.Errorf("need at least 2 points")
	}

	waypoints := geo.CoordConvertToGCJ02(toSlice(req.Points), req.Coordinate)
	n := len(waypoints)
	distances := make([][]float32, n)
	durations := make([][]float32, n)
	for i := range distances {
		distances[i] = make([]float32, n)
		durations[i] = make([]float32, n)
	}

	prov, err := e.Registry.Get(req.Provider)
	if err != nil {
		return nil, err
	}

	cacheOpts := cache.LookupOpts{
		Tenant:   req.Tenant,
		Method:   req.Method,
		Strategy: req.Strategy,
		TimeSlot: req.TimeSlot,
		Strict:   req.Strict,
		GeoWideM: req.GeoWideM,
	}
	if cacheOpts.GeoWideM <= 0 {
		cacheOpts.GeoWideM = 200
	}
	wmt := req.TimeSlot
	if wmt == "" {
		wmt = cache.TimeSlotWMH(time.Now())
	}

	routeReq := provider.RouteRequest{
		Strategy: req.Strategy,
		Method:   req.Method,
		SpeedMPS: req.SpeedMPS,
	}

	stats := Stats{}
	segmentByPair := make(map[string]provider.Step)

	// 1) Probe cache for every required OD; collect miss arcs for DensePlanner.
	miss := make([]arccover.Arc, 0, n*(n-1))
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			if ctx.Err() != nil {
				return e.packResult(distances, durations, stats, start), ErrDeadline
			}
			o, d := waypoints[i], waypoints[j]
			key := pairKey(o, d)
			if _, ok := segmentByPair[key]; ok {
				continue
			}
			if st, hit, cold, err := e.lookupEdge(ctx, cacheOpts, o, d); err != nil {
				return nil, err
			} else if hit {
				if cold {
					stats.ColdHits++
				} else {
					stats.CacheHits++
				}
				segmentByPair[key] = st
				continue
			}
			if !req.Strict {
				if st, hit, cold, err := e.lookupEdge(ctx, cacheOpts, d, o); err != nil {
					return nil, err
				} else if hit {
					if cold {
						stats.ColdHits++
					} else {
						stats.CacheHits++
					}
					segmentByPair[pairKey(d, o)] = st
					continue
				}
			}
			stats.CacheMisses++
			miss = append(miss, arccover.Arc{
				From: arccover.VertexID(i),
				To:   arccover.VertexID(j),
			})
		}
	}

	// 2) Plan Provider walks over residual misses.
	if len(miss) > 0 {
		plan, err := e.arcCover().Plan(ctx, n, miss, e.maxLegs())
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return e.packResult(distances, durations, stats, start), ErrDeadline
		}
		stats.ArcCoverCalls = len(plan.Routes)

		for _, route := range plan.Routes {
			if ctx.Err() != nil {
				return e.packResult(distances, durations, stats, start), ErrDeadline
			}
			wps := routeWaypoints(waypoints, route)
			if len(wps) < 2 {
				continue
			}
			steps, calls, err := e.Planner.RouteWaypoints(ctx, prov, routeReq, wps)
			stats.ProviderCalls += calls
			if err != nil {
				if ctx.Err() != nil {
					return e.packResult(distances, durations, stats, start), ErrDeadline
				}
				return nil, err
			}
			for _, st := range steps {
				if st.Source == provider.SourceFallback {
					stats.FallbackEdges++
				}
				segmentByPair[pairKey(st.Origin, st.Destination)] = st
				if st.Source != provider.SourceFallback {
					e.persistEdge(ctx, cacheOpts, provider.StepToEdge(st, wmt, prov.Name()))
				}
			}
		}
	}

	// 3) Fill N×N matrix; pair fallback for any remaining gaps.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if ctx.Err() != nil {
				return e.packResult(distances, durations, stats, start), ErrDeadline
			}
			if i == j {
				continue
			}
			o, d := waypoints[i], waypoints[j]
			if st, ok := segmentByPair[pairKey(o, d)]; ok {
				distances[i][j] = st.DistanceM
				durations[i][j] = st.DurationS
				continue
			}
			if !req.Strict {
				if st, ok := segmentByPair[pairKey(d, o)]; ok {
					distances[i][j] = st.DistanceM
					durations[i][j] = st.DurationS
					continue
				}
			}

			st, calls, err := e.fillPair(ctx, prov, cacheOpts, routeReq, o, d, wmt)
			stats.ProviderCalls += calls
			if err != nil {
				if ctx.Err() != nil {
					return e.packResult(distances, durations, stats, start), ErrDeadline
				}
				return nil, err
			}
			if st.Source == provider.SourceFallback {
				stats.FallbackEdges++
			}
			distances[i][j] = st.DistanceM
			durations[i][j] = st.DurationS
		}
	}

	return e.packResult(distances, durations, stats, start), nil
}

func routeWaypoints(points [][2]float32, route arccover.Route) [][2]float32 {
	if len(route.Legs) == 0 {
		return nil
	}
	out := make([][2]float32, 0, len(route.Legs)+1)
	out = append(out, points[route.Legs[0].From])
	for _, leg := range route.Legs {
		out = append(out, points[leg.To])
	}
	return out
}

func (e *Engine) packResult(distances, durations [][]float32, stats Stats, start time.Time) *Result {
	total := stats.CacheHits + stats.ColdHits + stats.CacheMisses
	if total > 0 {
		stats.CacheHitRatio = float64(stats.CacheHits+stats.ColdHits) / float64(total)
	}
	stats.ElapsedMs = time.Since(start).Milliseconds()
	return &Result{Distances: distances, Durations: durations, Stats: stats}
}

// lookupEdge: Redis L1 → Archive L2 (promote on hit).
func (e *Engine) lookupEdge(ctx context.Context, opts cache.LookupOpts, o, d [2]float32) (provider.Step, bool, bool, error) {
	if e.Cache != nil {
		got, err := e.Cache.Get(ctx, opts, o, d)
		if err != nil {
			return provider.Step{}, false, false, err
		}
		if got.Hit {
			return edgeToStep(got.Edge), true, false, nil
		}
	}
	if e.Archive == nil {
		return provider.Step{}, false, false, nil
	}
	edge, ok, err := e.Archive.Get(ctx, opts, o, d)
	if err != nil || !ok {
		return provider.Step{}, false, false, err
	}
	if e.Cache != nil {
		_ = e.Cache.Put(ctx, opts, edge)
	}
	return edgeToStep(edge), true, true, nil
}

func (e *Engine) persistEdge(ctx context.Context, opts cache.LookupOpts, edge cache.Edge) {
	if e.Cache != nil {
		_ = e.Cache.Put(ctx, opts, edge)
	}
	if e.Async != nil {
		e.Async.EnqueueUpsert(opts, edge)
		return
	}
	if e.Archive != nil {
		_ = e.Archive.Upsert(ctx, opts, edge)
	}
}

func (e *Engine) fillPair(ctx context.Context, prov provider.Provider, opts cache.LookupOpts, routeReq provider.RouteRequest, o, d [2]float32, wmt string) (provider.Step, int, error) {
	if st, hit, _, err := e.lookupEdge(ctx, opts, o, d); err != nil {
		return provider.Step{}, 0, err
	} else if hit {
		return st, 0, nil
	}
	st, calls, err := e.Planner.RoutePair(ctx, prov, routeReq, o, d)
	if err != nil {
		return provider.Step{}, calls, err
	}
	if st.Source != provider.SourceFallback {
		e.persistEdge(ctx, opts, provider.StepToEdge(st, wmt, prov.Name()))
	}
	return st, calls, nil
}

func edgeToStep(e cache.Edge) provider.Step {
	return provider.Step{
		Origin: e.Origin, Destination: e.Destination,
		DistanceM: e.DistanceM, DurationS: e.DurationS,
		Polyline: e.Polyline, Source: provider.SourceCache,
	}
}

func pairKey(a, b [2]float32) string {
	return cache.FormatCoordKey(a) + ">" + cache.FormatCoordKey(b)
}

func toSlice(in [][2]float32) [][]float32 {
	out := make([][]float32, len(in))
	for i, p := range in {
		out[i] = []float32{p[0], p[1]}
	}
	return out
}
