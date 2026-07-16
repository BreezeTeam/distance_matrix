package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"distance-matrix/internal/cache"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/provider"
	"distance-matrix/internal/geo"
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
	CacheMisses   int
	ProviderCalls int
	FallbackEdges int
	ElapsedMs     int64
	CacheHitRatio float64
}

// Engine computes OD matrices.
type Engine struct {
	Cache          *cache.Store
	Planner        *planner.Planner
	Registry       *provider.Registry
	Chain          planner.ChainOptimizer
	FallbackFactor float32
	MaxPoints      int
}

func (e *Engine) chainOpt() planner.ChainOptimizer {
	if e.Chain != nil {
		return e.Chain
	}
	return planner.GreedyChain{}
}

// Compute fills distance/duration matrices.
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

	distinct := distinctPoints(waypoints)
	chainEdges := e.chainOpt().Order(distinct)
	chainWps := planner.ChainToWaypoints(chainEdges)

	// Load chain segments from cache; route contiguous misses in one provider call.
	if len(chainWps) >= 2 {
		needRoute := false
		for i := 0; i+1 < len(chainWps); i++ {
			if ctx.Err() != nil {
				return e.packResult(distances, durations, stats, start), ErrDeadline
			}
			o, d := chainWps[i], chainWps[i+1]
			if e.Cache != nil {
				got, err := e.Cache.Get(ctx, cacheOpts, o, d)
				if err != nil {
					return nil, err
				}
				if got.Hit {
					stats.CacheHits++
					segmentByPair[pairKey(o, d)] = edgeToStep(got.Edge)
					continue
				}
				stats.CacheMisses++
			}
			needRoute = true
		}
		if needRoute {
			steps, calls, err := e.Planner.RouteWaypoints(ctx, prov, routeReq, chainWps)
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
				if e.Cache != nil && st.Source != provider.SourceFallback {
					_ = e.Cache.Put(ctx, cacheOpts, provider.StepToEdge(st, wmt, prov.Name()))
				}
			}
		}
	}

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

func (e *Engine) packResult(distances, durations [][]float32, stats Stats, start time.Time) *Result {
	total := stats.CacheHits + stats.CacheMisses
	if total > 0 {
		stats.CacheHitRatio = float64(stats.CacheHits) / float64(total)
	}
	stats.ElapsedMs = time.Since(start).Milliseconds()
	return &Result{Distances: distances, Durations: durations, Stats: stats}
}

func (e *Engine) fillPair(ctx context.Context, prov provider.Provider, opts cache.LookupOpts, routeReq provider.RouteRequest, o, d [2]float32, wmt string) (provider.Step, int, error) {
	if e.Cache != nil {
		got, err := e.Cache.Get(ctx, opts, o, d)
		if err != nil {
			return provider.Step{}, 0, err
		}
		if got.Hit {
			return edgeToStep(got.Edge), 0, nil
		}
	}
	st, calls, err := e.Planner.RoutePair(ctx, prov, routeReq, o, d)
	if err != nil {
		return provider.Step{}, calls, err
	}
	if e.Cache != nil && st.Source != provider.SourceFallback {
		_ = e.Cache.Put(ctx, opts, provider.StepToEdge(st, wmt, prov.Name()))
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

func distinctPoints(points [][2]float32) [][2]float32 {
	seen := map[string]struct{}{}
	var out [][2]float32
	for _, p := range points {
		k := cache.FormatCoordKey(p)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, p)
	}
	return out
}

func toSlice(in [][2]float32) [][]float32 {
	out := make([][]float32, len(in))
	for i, p := range in {
		out[i] = []float32{p[0], p[1]}
	}
	return out
}
