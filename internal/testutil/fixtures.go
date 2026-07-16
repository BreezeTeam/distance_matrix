package testutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"distance-matrix/internal/cache"
	"distance-matrix/internal/provider"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// StubProvider returns fixed segment metrics for each leg.
type StubProvider struct {
	NameVal string
	Delay   time.Duration
	Calls   atomic.Int32
}

func (p *StubProvider) Name() string {
	if p.NameVal != "" {
		return p.NameVal
	}
	return "amap"
}

func (p *StubProvider) Ready() bool { return true }

func (p *StubProvider) Route(ctx context.Context, req provider.RouteRequest) (*provider.RouteResult, error) {
	p.Calls.Add(1)
	if p.Delay > 0 {
		select {
		case <-time.After(p.Delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var steps []provider.Step
	for i := 0; i+1 < len(req.Waypoints); i++ {
		a, b := req.Waypoints[i], req.Waypoints[i+1]
		steps = append(steps, provider.Step{
			Origin: a, Destination: b,
			DistanceM: 1000, DurationS: 120,
			Source: provider.SourceProvider,
		})
	}
	return routeResultFromSteps(steps), nil
}

func routeResultFromSteps(steps []provider.Step) *provider.RouteResult {
	var d, t float32
	for _, s := range steps {
		d += s.DistanceM
		t += s.DurationS
	}
	return &provider.RouteResult{Steps: steps, DistanceM: d, DurationS: t, Source: provider.SourceProvider}
}

func NewRegistry(p provider.Provider) *provider.Registry {
	reg := provider.NewRegistry()
	reg.Register(p)
	reg.SetDefault(p.Name())
	return reg
}

func SetupRedis(t *testing.T) (*miniredis.Miniredis, *cache.Store) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, cache.NewStore(rdb, "test", 3600)
}

func SeedEdge(t *testing.T, store *cache.Store, tenant string, o, d [2]float32, distance, duration float32) {
	t.Helper()
	opts := cache.LookupOpts{Tenant: tenant, Method: 0, Strategy: 0}
	slot := cache.TimeSlotWMH(time.Now())
	edge := cache.Edge{
		Origin: o, Destination: d,
		DistanceM: distance, DurationS: duration,
		WMT: slot, Provider: "amap",
	}
	if err := store.Put(context.Background(), opts, edge); err != nil {
		t.Fatal(err)
	}
}
