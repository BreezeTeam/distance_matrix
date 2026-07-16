package planner

import (
	"context"
	"testing"

	"distance-matrix/internal/provider"
)

type countingProvider struct {
	calls int
}

func (c *countingProvider) Name() string { return "count" }
func (c *countingProvider) Ready() bool  { return true }
func (c *countingProvider) Route(_ context.Context, req provider.RouteRequest) (*provider.RouteResult, error) {
	c.calls++
	var steps []provider.Step
	for i := 0; i+1 < len(req.Waypoints); i++ {
		a, b := req.Waypoints[i], req.Waypoints[i+1]
		steps = append(steps, provider.Step{
			Origin: a, Destination: b,
			DistanceM: 100, DurationS: 10,
			Source: provider.SourceProvider,
		})
	}
	return &provider.RouteResult{Steps: steps, Source: provider.SourceProvider}, nil
}

func TestRouteWaypointsSingleBatch(t *testing.T) {
	p := &countingProvider{}
	pl := NewPlanner(12)
	wps := [][2]float32{{1, 1}, {2, 2}, {3, 3}}

	steps, calls, err := pl.RouteWaypoints(context.Background(), p, provider.RouteRequest{}, wps)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if calls != 1 || p.calls != 1 {
		t.Fatalf("expected 1 provider call, got calls=%d p.calls=%d", calls, p.calls)
	}
}

func TestRouteWaypointsMultiBatch(t *testing.T) {
	p := &countingProvider{}
	pl := NewPlanner(3)
	wps := make([][2]float32, 0, 8)
	for i := 0; i < 8; i++ {
		wps = append(wps, [2]float32{float32(i), float32(i)})
	}

	_, calls, err := pl.RouteWaypoints(context.Background(), p, provider.RouteRequest{}, wps)
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("batch size 3 with 8 waypoints should need multiple calls, got %d", calls)
	}
}

func TestRoutePair(t *testing.T) {
	p := &countingProvider{}
	pl := NewPlanner(12)
	o := [2]float32{116.4, 39.9}
	d := [2]float32{116.41, 39.91}

	st, calls, err := pl.RoutePair(context.Background(), p, provider.RouteRequest{}, o, d)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || st.DistanceM != 100 {
		t.Fatalf("unexpected pair result: %+v calls=%d", st, calls)
	}
}

func TestChainToWaypoints(t *testing.T) {
	edges := [][][2]float32{
		{{1, 1}, {2, 2}},
		{{2, 2}, {3, 3}},
	}
	wps := ChainToWaypoints(edges)
	if len(wps) != 3 {
		t.Fatalf("expected 3 waypoints, got %v", wps)
	}
	if wps[0] != edges[0][0] || wps[2] != edges[1][1] {
		t.Fatalf("chain flatten wrong: %v", wps)
	}
}

func TestGreedyChainOrdersEdges(t *testing.T) {
	points := [][2]float32{{1, 1}, {2, 2}, {3, 3}}
	edges := GreedyChain{}.Order(points)
	if len(edges) == 0 {
		t.Fatal("expected ordered edges")
	}
}
