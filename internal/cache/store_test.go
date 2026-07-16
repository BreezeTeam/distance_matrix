package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestStoreIsolationMethodStrategy(t *testing.T) {
	_, store := setupStore(t)
	ctx := context.Background()

	optsCar := LookupOpts{Tenant: "t1", Method: 0, Strategy: 0}
	optsTruck := LookupOpts{Tenant: "t1", Method: 1, Strategy: 0}

	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	edge := Edge{
		Origin: o, Destination: d,
		DistanceM: 1200, DurationS: 180,
		WMT: TimeSlotWMH(time.Now()), Provider: "amap",
	}
	if err := store.Put(ctx, optsCar, edge); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, optsCar, o, d)
	if err != nil || !got.Hit || got.Edge.DistanceM != 1200 {
		t.Fatalf("car hit: %+v err=%v", got, err)
	}
	got, err = store.Get(ctx, optsTruck, o, d)
	if err != nil || got.Hit {
		t.Fatalf("truck should miss, got %+v", got)
	}
}

func TestStoreWriteThroughReadable(t *testing.T) {
	_, store := setupStore(t)
	ctx := context.Background()
	opts := LookupOpts{Tenant: "default", Method: 0, Strategy: 0, Strict: true}
	o := [2]float32{116.397, 39.908}
	d := [2]float32{116.407, 39.918}
	e := Edge{Origin: o, Destination: d, DistanceM: 500, DurationS: 60, WMT: TimeSlotWMH(time.Now()), Provider: "amap"}
	if err := store.Put(ctx, opts, e); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, opts, o, d)
	if err != nil || !got.Hit || got.Edge.DistanceM != 500 {
		t.Fatalf("get: %+v err=%v", got, err)
	}
}

func TestStoreTenantIsolation(t *testing.T) {
	_, store := setupStore(t)
	ctx := context.Background()
	o := [2]float32{116.40, 39.90}
	d := [2]float32{116.41, 39.91}
	optsA := LookupOpts{Tenant: "alpha", Method: 0, Strategy: 0}
	optsB := LookupOpts{Tenant: "beta", Method: 0, Strategy: 0}
	edge := Edge{
		Origin: o, Destination: d,
		DistanceM: 900, DurationS: 90,
		WMT: TimeSlotWMH(time.Now()), Provider: "amap",
	}
	if err := store.Put(ctx, optsA, edge); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, optsB, o, d)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hit {
		t.Fatalf("tenant beta should not see alpha cache: %+v", got)
	}
}

func TestStoreSameGeohashZeroDistance(t *testing.T) {
	_, store := setupStore(t)
	ctx := context.Background()
	opts := LookupOpts{Tenant: "default", Method: 0, Strategy: 0}
	p := [2]float32{116.40, 39.90}

	got, err := store.Get(ctx, opts, p, p)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Hit || got.Edge.DistanceM != 0 || got.Edge.DurationS != 0 {
		t.Fatalf("same point should hit zero edge: %+v", got)
	}
}

func TestStoreFuzzySpatialHit(t *testing.T) {
	_, store := setupStore(t)
	ctx := context.Background()
	o := [2]float32{116.397128, 39.916527}
	d := [2]float32{116.407526, 39.904030}
	opts := LookupOpts{Tenant: "fuzzy", Method: 0, Strategy: 0, GeoWideM: 200}

	slot := TimeSlotWMH(time.Now())
	edge := Edge{
		Origin: o, Destination: d,
		DistanceM: 1500, DurationS: 200,
		WMT: slot, Provider: "amap",
	}
	if err := store.Put(ctx, opts, edge); err != nil {
		t.Fatal(err)
	}

	oNear := [2]float32{116.397200, 39.916600}
	dNear := [2]float32{116.407600, 39.904100}
	got, err := store.Get(ctx, opts, oNear, dNear)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Hit {
		t.Fatalf("expected fuzzy spatial hit, got miss: %+v", got)
	}
	if got.Edge.DistanceM != 1500 {
		t.Fatalf("fuzzy hit distance=%f", got.Edge.DistanceM)
	}
}

func TestParseTenant(t *testing.T) {
	if ParseTenant("") != "default" {
		t.Fatal("empty tenant")
	}
	if ParseTenant("  acme ") != "acme" {
		t.Fatal("trim tenant")
	}
}

func setupStore(t *testing.T) (*miniredis.Miniredis, *Store) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, NewStore(rdb, "test", 3600)
}
