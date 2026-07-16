package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"distance-matrix/internal/cache"
	"distance-matrix/internal/planner"
	"distance-matrix/internal/provider"
	"distance-matrix/internal/testutil"
)

func newTestEngine(reg *provider.Registry, store *cache.Store, maxPoints int) *Engine {
	return &Engine{
		Cache:     store,
		Planner:   planner.NewPlanner(12),
		Registry:  reg,
		MaxPoints: maxPoints,
	}
}

func TestEngineComputeSmallMatrix(t *testing.T) {
	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), nil, 0)

	res, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{{116.4, 39.9}, {116.41, 39.91}},
		Coordinate: "gcj02",
		Tenant:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Distances[0][1] != 1000 || res.Durations[0][1] != 120 {
		t.Fatalf("unexpected matrix: d=%v t=%v", res.Distances, res.Durations)
	}
	if res.Distances[0][0] != 0 || res.Distances[1][1] != 0 {
		t.Fatalf("diagonal should be zero")
	}
}

func TestEngineCacheHitSkipsProvider(t *testing.T) {
	_, store := testutil.SetupRedis(t)
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	testutil.SeedEdge(t, store, "t1", o, d, 777, 88)
	testutil.SeedEdge(t, store, "t1", d, o, 777, 88) // chain walk direction varies

	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), store, 0)

	res, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{o, d},
		Coordinate: "gcj02",
		Tenant:     "t1",
		Strict:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Distances[0][1] != 777 || res.Durations[0][1] != 88 {
		t.Fatalf("expected cached values, got d=%v t=%v", res.Distances, res.Durations)
	}
	if stub.Calls.Load() != 0 {
		t.Fatalf("provider should not be called, calls=%d", stub.Calls.Load())
	}
	if res.Stats.CacheHits == 0 {
		t.Fatalf("expected cache hits, stats=%+v", res.Stats)
	}
}

func TestEngineWriteThroughThenRetryHitsCache(t *testing.T) {
	_, store := testutil.SetupRedis(t)
	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), store, 0)
	req := Request{
		Points:     [][2]float32{{116.40, 39.90}, {116.41, 39.91}},
		Coordinate: "gcj02",
		Tenant:     "retry",
		Strict:     true,
	}

	first, err := eng.Compute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	callsAfterFirst := stub.Calls.Load()
	if callsAfterFirst == 0 {
		t.Fatal("first compute should call provider")
	}

	second, err := eng.Compute(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if stub.Calls.Load() != callsAfterFirst {
		t.Fatalf("retry should hit cache, calls before=%d after=%d", callsAfterFirst, stub.Calls.Load())
	}
	if second.Distances[0][1] != first.Distances[0][1] {
		t.Fatalf("retry matrix mismatch: first=%v second=%v", first.Distances, second.Distances)
	}
	if second.Stats.CacheHitRatio <= 0 {
		t.Fatalf("expected cache hits on retry, stats=%+v", second.Stats)
	}
}

func TestEngineStrictModeNoReverseEdge(t *testing.T) {
	_, store := testutil.SetupRedis(t)
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	testutil.SeedEdge(t, store, "strict", d, o, 555, 66) // reverse only

	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), store, 0)

	res, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{o, d},
		Coordinate: "gcj02",
		Tenant:     "strict",
		Strict:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Distances[0][1] != 1000 {
		t.Fatalf("strict mode should not use reverse cache, got %v", res.Distances[0][1])
	}
	if stub.Calls.Load() == 0 {
		t.Fatal("expected provider call for missing forward edge")
	}
}

func TestEngineNonStrictUsesReverseEdge(t *testing.T) {
	_, store := testutil.SetupRedis(t)
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	testutil.SeedEdge(t, store, "fuzzy", d, o, 555, 66)

	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), store, 0)

	res, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{o, d},
		Coordinate: "gcj02",
		Tenant:     "fuzzy",
		Strict:     false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Distances[0][1] != 555 || res.Durations[0][1] != 66 {
		t.Fatalf("non-strict should use reverse cached edge, got d=%v t=%v", res.Distances, res.Durations)
	}
	if stub.Calls.Load() != 0 {
		t.Fatalf("provider should not be called, calls=%d", stub.Calls.Load())
	}
}

func TestEngineDeadline(t *testing.T) {
	stub := &testutil.StubProvider{Delay: 200 * time.Millisecond}
	eng := newTestEngine(testutil.NewRegistry(stub), nil, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	pts := make([][2]float32, 0, 4)
	for i := 0; i < 4; i++ {
		pts = append(pts, [2]float32{116.40 + float32(i)*0.01, 39.90 + float32(i)*0.01})
	}
	_, err := eng.Compute(ctx, Request{
		Points:     pts,
		Coordinate: "gcj02",
		Tenant:     "deadline",
	})
	if !errors.Is(err, ErrDeadline) {
		t.Fatalf("expected ErrDeadline, got %v", err)
	}
}

func TestEngineMaxPoints(t *testing.T) {
	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), nil, 2)

	_, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{{1, 1}, {2, 2}, {3, 3}},
		Tenant:     "test",
	})
	if err == nil {
		t.Fatal("expected too many points error")
	}
}

func TestEngineTooFewPoints(t *testing.T) {
	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), nil, 0)

	_, err := eng.Compute(context.Background(), Request{
		Points: [][2]float32{{116.4, 39.9}},
		Tenant: "test",
	})
	if err == nil {
		t.Fatal("expected error for single point")
	}
}

func TestEngineThreePointMatrix(t *testing.T) {
	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), nil, 0)
	points := [][2]float32{
		{116.40, 39.90},
		{116.41, 39.91},
		{116.42, 39.92},
	}

	res, err := eng.Compute(context.Background(), Request{
		Points:     points,
		Coordinate: "gcj02",
		Tenant:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	n := len(points)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				if res.Distances[i][j] != 0 {
					t.Fatalf("diagonal [%d][%d] should be 0", i, j)
				}
				continue
			}
			if res.Distances[i][j] != 1000 || res.Durations[i][j] != 120 {
				t.Fatalf("pair [%d][%d] want 1000/120 got %f/%f", i, j, res.Distances[i][j], res.Durations[i][j])
			}
		}
	}
}

func TestEngineTenantCacheIsolation(t *testing.T) {
	_, store := testutil.SetupRedis(t)
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	testutil.SeedEdge(t, store, "tenant-a", o, d, 111, 11)

	stub := &testutil.StubProvider{}
	eng := newTestEngine(testutil.NewRegistry(stub), store, 0)

	res, err := eng.Compute(context.Background(), Request{
		Points:     [][2]float32{o, d},
		Tenant:     "tenant-b",
		Strict:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Distances[0][1] != 1000 {
		t.Fatalf("other tenant cache must not leak, got %f", res.Distances[0][1])
	}
}
