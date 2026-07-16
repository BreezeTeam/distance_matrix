package planner

import (
	"context"
	"sync"

	"distance-matrix/internal/provider"
	"distance-matrix/internal/geo"
	"golang.org/x/sync/errgroup"
)

// Planner batches waypoint routes via Provider.
type Planner struct {
	BatchSize int
}

func NewPlanner(batchSize int) *Planner {
	if batchSize <= 0 {
		batchSize = 12
	}
	return &Planner{BatchSize: batchSize}
}

// RouteWaypoints calls provider for a polyline, returns per-segment steps.
func (p *Planner) RouteWaypoints(ctx context.Context, prov provider.Provider, req provider.RouteRequest, waypoints [][2]float32) ([]provider.Step, int, error) {
	if len(waypoints) < 2 {
		return nil, 0, nil
	}
	packets := geo.WaypointsPacket(p.BatchSize, waypoints...)
	if len(packets) == 0 {
		return nil, 0, nil
	}

	results := make([][]provider.Step, len(packets))
	calls := 0
	g, gctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	for i := range packets {
		i, packet := i, packets[i]
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}
			sub := req
			sub.Waypoints = packet
			res, err := prov.Route(gctx, sub)
			mu.Lock()
			calls++
			mu.Unlock()
			if err != nil {
				return err
			}
			results[i] = res.Steps
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, calls, err
	}

	var steps []provider.Step
	for _, batch := range results {
		steps = append(steps, batch...)
	}
	return steps, calls, nil
}

// RoutePair fetches a single origin→destination segment.
func (p *Planner) RoutePair(ctx context.Context, prov provider.Provider, req provider.RouteRequest, origin, destination [2]float32) (provider.Step, int, error) {
	sub := req
	sub.Waypoints = [][2]float32{origin, destination}
	res, err := prov.Route(ctx, sub)
	if err != nil {
		return provider.Step{}, 1, err
	}
	if len(res.Steps) == 0 {
		return provider.Step{Origin: origin, Destination: destination, DistanceM: res.DistanceM, DurationS: res.DurationS, Source: res.Source}, 1, nil
	}
	return res.Steps[0], 1, nil
}
